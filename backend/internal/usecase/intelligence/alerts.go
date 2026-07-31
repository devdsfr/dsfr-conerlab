package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

// AlertDefinition é o formato serializado em AlertRule.Definition. Dois tipos são
// suportados, espelhando os exemplos do documento de requisitos (Módulo 7):
//   - "team_frequency": notifica quando uma equipe (ou qualquer equipe da liga, se
//     TeamID for omitido) supera `CornersThreshold` escanteios em pelo menos
//     `ThresholdPct`% dos últimos `LastNGames` jogos.
//   - "opponent_average": notifica quando um adversário (ou qualquer equipe da liga)
//     cede, em média, mais que `MinAvgConceded` escanteios por jogo.
type AlertDefinition struct {
	Type             string  `json:"type"`
	LeagueID         int64   `json:"league_id"`
	TeamID           *int64  `json:"team_id,omitempty"`
	LastNGames       int     `json:"last_n_games,omitempty"`
	CornersThreshold int     `json:"corners_threshold,omitempty"`
	ThresholdPct     float64 `json:"threshold_pct,omitempty"`
	MinAvgConceded   float64 `json:"min_avg_conceded,omitempty"`
}

type AlertMatch struct {
	TeamID   int64   `json:"team_id"`
	TeamName string  `json:"team_name"`
	Value    float64 `json:"value"`
	Detail   string  `json:"detail"`
}

type AlertEvaluationResult struct {
	RuleID    int64        `json:"rule_id"`
	RuleName  string       `json:"rule_name"`
	Triggered bool         `json:"triggered"`
	Matches   []AlertMatch `json:"matches"`
	Meta      Meta         `json:"meta"`
}

type AlertUsecase struct {
	rules   repository.AlertRuleRepository
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
	stats   repository.LeagueStatsRepository
	matches repository.MatchRepository
}

func NewAlertUsecase(
	rules repository.AlertRuleRepository,
	teams repository.TeamRepository,
	leagues repository.LeagueRepository,
	stats repository.LeagueStatsRepository,
	matches repository.MatchRepository,
) *AlertUsecase {
	return &AlertUsecase{rules: rules, teams: teams, leagues: leagues, stats: stats, matches: matches}
}

func (u *AlertUsecase) Create(ctx context.Context, userID int64, name string, def AlertDefinition) (*domain.AlertRule, error) {
	if def.Type != "team_frequency" && def.Type != "opponent_average" {
		return nil, fmt.Errorf("type deve ser 'team_frequency' ou 'opponent_average'")
	}
	raw, err := json.Marshal(def)
	if err != nil {
		return nil, err
	}
	rule := &domain.AlertRule{UserID: userID, Name: name, Definition: string(raw), Active: true}
	if err := u.rules.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (u *AlertUsecase) List(ctx context.Context, userID int64) ([]domain.AlertRule, error) {
	return u.rules.List(ctx, userID)
}

func (u *AlertUsecase) Delete(ctx context.Context, id, userID int64) error {
	return u.rules.Delete(ctx, id, userID)
}

// Evaluate executa a regra imediatamente contra os dados atuais e retorna quais
// equipes (uma ou várias) atendem ao critério — nunca sugere uma aposta, apenas
// relata o fato estatístico observado.
func (u *AlertUsecase) Evaluate(ctx context.Context, ruleID int64) (*AlertEvaluationResult, error) {
	rule, err := u.rules.GetByID(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("regra não encontrada: %w", err)
	}
	var def AlertDefinition
	if err := json.Unmarshal([]byte(rule.Definition), &def); err != nil {
		return nil, fmt.Errorf("definição de regra inválida: %w", err)
	}
	league, err := u.leagues.GetByID(ctx, def.LeagueID)
	if err != nil {
		return nil, fmt.Errorf("campeonato não encontrado: %w", err)
	}

	var candidateTeams []domain.Team
	if def.TeamID != nil {
		t, err := u.teams.GetByID(ctx, *def.TeamID)
		if err != nil {
			return nil, err
		}
		candidateTeams = []domain.Team{*t}
	} else {
		candidateTeams, err = u.teams.List(ctx, &def.LeagueID)
		if err != nil {
			return nil, err
		}
	}

	var matches []AlertMatch
	gamesAnalyzed := 0

	switch def.Type {
	case "team_frequency":
		limit := def.LastNGames
		if limit <= 0 {
			limit = 10
		}
		for _, t := range candidateTeams {
			views, err := u.matches.TeamMatches(ctx, repository.MatchFilter{TeamID: t.ID, LeagueID: &def.LeagueID, Limit: limit})
			if err != nil || len(views) == 0 {
				continue
			}
			values := make([]int, len(views))
			for i, v := range views {
				values[i] = v.TotalCorners
			}
			freq := usecase.FrequencyAboveThresholds(values, []int{def.CornersThreshold})[0]
			gamesAnalyzed += len(views)
			if freq.Pct >= def.ThresholdPct {
				matches = append(matches, AlertMatch{
					TeamID:   t.ID,
					TeamName: t.Name,
					Value:    freq.Pct,
					Detail:   fmt.Sprintf("%d de %d jogos acima de %d escanteios (%.1f%%)", freq.Count, freq.Total, def.CornersThreshold, freq.Pct),
				})
			}
		}

	case "opponent_average":
		aggregates, err := u.stats.TeamAggregates(ctx, def.LeagueID, nil, def.LastNGames)
		if err != nil {
			return nil, err
		}
		teamFilter := map[int64]bool{}
		for _, t := range candidateTeams {
			teamFilter[t.ID] = true
		}
		for _, a := range aggregates {
			if !teamFilter[a.Team.ID] {
				continue
			}
			gamesAnalyzed += a.SampleSize
			if a.AvgAgainst >= def.MinAvgConceded {
				matches = append(matches, AlertMatch{
					TeamID:   a.Team.ID,
					TeamName: a.Team.Name,
					Value:    a.AvgAgainst,
					Detail:   fmt.Sprintf("Média de %.2f escanteios cedidos por jogo", a.AvgAgainst),
				})
			}
		}
	}

	return &AlertEvaluationResult{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Triggered: len(matches) > 0,
		Matches:   matches,
		Meta: Meta{
			LeagueID:      league.ID,
			LeagueName:    league.Name,
			Period:        periodLabel(def.LastNGames),
			GamesAnalyzed: gamesAnalyzed,
			UpdatedAt:     time.Now(),
		},
	}, nil
}
