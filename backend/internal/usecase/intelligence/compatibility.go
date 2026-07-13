package intelligence

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/repository"
)

type CompatibilityUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
}

func NewCompatibilityUsecase(matches repository.MatchRepository, teams repository.TeamRepository, leagues repository.LeagueRepository) *CompatibilityUsecase {
	return &CompatibilityUsecase{matches: matches, teams: teams, leagues: leagues}
}

// CompatibilityReport cruza a média de escanteios a favor da equipe A com a média de
// escanteios cedidos pela equipe B, expressando o quão "compatível" esse confronto é
// com o perfil ofensivo de A — não é uma previsão de resultado, apenas uma leitura
// estatística de aderência entre ataque e defesa históricos.
type CompatibilityReport struct {
	TeamAID          int64   `json:"team_a_id"`
	TeamAName        string  `json:"team_a_name"`
	TeamAAvgFor      float64 `json:"team_a_avg_for"`
	TeamBID          int64   `json:"team_b_id"`
	TeamBName        string  `json:"team_b_name"`
	TeamBAvgConceded float64 `json:"team_b_avg_conceded"`
	CompatibilityPct float64 `json:"compatibility_pct"`
	Meta             Meta    `json:"meta"`
}

// Compute calcula a compatibilidade estatística entre o ataque de escanteios da
// equipe A e a defesa (escanteios cedidos) da equipe B. A métrica é
// min(avgFor, avgConceded) / max(avgFor, avgConceded) * 100 — quanto mais próximos os
// dois valores, maior a aderência entre o padrão ofensivo de A e o padrão defensivo de B.
func (u *CompatibilityUsecase) Compute(ctx context.Context, teamAID, teamBID int64, leagueID int64, limit int) (*CompatibilityReport, error) {
	if limit <= 0 {
		limit = 10
	}
	teamA, err := u.teams.GetByID(ctx, teamAID)
	if err != nil {
		return nil, fmt.Errorf("equipe A não encontrada: %w", err)
	}
	teamB, err := u.teams.GetByID(ctx, teamBID)
	if err != nil {
		return nil, fmt.Errorf("equipe B não encontrada: %w", err)
	}
	league, err := u.leagues.GetByID(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("campeonato não encontrado: %w", err)
	}

	viewsA, err := u.matches.TeamMatches(ctx, repository.MatchFilter{TeamID: teamAID, LeagueID: &leagueID, Limit: limit})
	if err != nil {
		return nil, err
	}
	viewsB, err := u.matches.TeamMatches(ctx, repository.MatchFilter{TeamID: teamBID, LeagueID: &leagueID, Limit: limit})
	if err != nil {
		return nil, err
	}

	avgFor := average(func() []int {
		vals := make([]int, len(viewsA))
		for i, v := range viewsA {
			vals[i] = v.CornersFor
		}
		return vals
	}())
	avgConceded := average(func() []int {
		vals := make([]int, len(viewsB))
		for i, v := range viewsB {
			vals[i] = v.CornersAgainst
		}
		return vals
	}())

	compatibility := 0.0
	if avgFor > 0 || avgConceded > 0 {
		hi, lo := avgFor, avgConceded
		if lo > hi {
			hi, lo = lo, hi
		}
		if hi > 0 {
			compatibility = round2(100 * lo / hi)
		}
	}

	return &CompatibilityReport{
		TeamAID:          teamA.ID,
		TeamAName:        teamA.Name,
		TeamAAvgFor:      round2(avgFor),
		TeamBID:          teamB.ID,
		TeamBName:        teamB.Name,
		TeamBAvgConceded: round2(avgConceded),
		CompatibilityPct: compatibility,
		Meta: Meta{
			LeagueID:      league.ID,
			LeagueName:    league.Name,
			Period:        periodLabel(limit),
			GamesAnalyzed: len(viewsA) + len(viewsB),
			UpdatedAt:     time.Now(),
		},
	}, nil
}

func average(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}
