package intelligence

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

type ConsistencyUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
}

func NewConsistencyUsecase(matches repository.MatchRepository, teams repository.TeamRepository, leagues repository.LeagueRepository) *ConsistencyUsecase {
	return &ConsistencyUsecase{matches: matches, teams: teams, leagues: leagues}
}

// ThresholdConsistency é uma linha da tabela "Acima de N escanteios: X de Y (Z%)".
type ThresholdConsistency struct {
	Threshold int     `json:"threshold"`
	Hits      int     `json:"hits"`
	Total     int     `json:"total"`
	Pct       float64 `json:"pct"`
}

// ConsistencyReport é a resposta do indicador "Consistência" do módulo de
// Inteligência Estatística. ConsistencyIndex reaproveita a mesma métrica (1 - CV,
// limitado a [0,1]) já usada no Dashboard Principal — aqui expressa como percentual
// para casar com o formato do documento de requisitos ("Índice de consistência: 94%").
type ConsistencyReport struct {
	TeamID           int64                  `json:"team_id"`
	TeamName         string                 `json:"team_name"`
	Thresholds       []ThresholdConsistency `json:"thresholds"`
	ConsistencyIndex float64                `json:"consistency_index_pct"`
	Meta             Meta                   `json:"meta"`
}

var defaultThresholds = []int{4, 5, 6, 7, 8, 9, 10}

func (u *ConsistencyUsecase) Compute(ctx context.Context, teamID int64, leagueID int64, seasonID *int64, limit int) (*ConsistencyReport, error) {
	if limit <= 0 {
		limit = 10
	}
	team, err := u.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("equipe não encontrada: %w", err)
	}
	league, err := u.leagues.GetByID(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("campeonato não encontrado: %w", err)
	}

	var seasonIDPtr *int64
	var seasonIDs []int64
	if seasonID != nil {
		seasonIDPtr = seasonID
		seasonIDs = []int64{*seasonID}
	}

	views, err := u.matches.TeamMatches(ctx, repository.MatchFilter{TeamID: teamID, LeagueID: &leagueID, SeasonID: seasonIDPtr, Limit: limit})
	if err != nil {
		return nil, err
	}

	values := make([]int, 0, len(views))
	for _, v := range views {
		values = append(values, v.TotalCorners)
	}

	freqs := usecase.FrequencyAboveThresholds(values, defaultThresholds)
	thresholds := make([]ThresholdConsistency, 0, len(freqs))
	for _, f := range freqs {
		thresholds = append(thresholds, ThresholdConsistency{Threshold: f.Threshold, Hits: f.Count, Total: f.Total, Pct: f.Pct})
	}

	summary := usecase.Summarize(values)

	return &ConsistencyReport{
		TeamID:           team.ID,
		TeamName:         team.Name,
		Thresholds:       thresholds,
		ConsistencyIndex: round2(summary.ConsistencyIndex * 100),
		Meta: Meta{
			LeagueID:      league.ID,
			LeagueName:    league.Name,
			SeasonIDs:     seasonIDs,
			Period:        fmt.Sprintf("Últimos %d jogos", limit),
			GamesAnalyzed: len(views),
			UpdatedAt:     time.Now(),
		},
	}, nil
}
