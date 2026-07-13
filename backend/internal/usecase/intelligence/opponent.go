package intelligence

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/devdsfr/cornerlab/internal/repository"
)

type OpponentUsecase struct {
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
	stats   repository.LeagueStatsRepository
}

func NewOpponentUsecase(teams repository.TeamRepository, leagues repository.LeagueRepository, stats repository.LeagueStatsRepository) *OpponentUsecase {
	return &OpponentUsecase{teams: teams, leagues: leagues, stats: stats}
}

// OpponentReport descreve o quanto um adversário costuma ceder de escanteios e sua
// posição no ranking da liga nessa métrica (ex: "3º pior" = 3ª maior média cedida).
type OpponentReport struct {
	OpponentID   int64   `json:"opponent_id"`
	OpponentName string  `json:"opponent_name"`
	AvgConceded  float64 `json:"avg_conceded"`
	LeagueRank   int     `json:"league_rank"` // 1 = maior média cedida (pior defesa) da amostra
	LeagueSize   int     `json:"league_size"`
	RankLabel    string  `json:"rank_label"` // ex: "3º pior da liga"
	Meta         Meta    `json:"meta"`
}

func (u *OpponentUsecase) Compute(ctx context.Context, opponentID int64, leagueID int64, seasonIDs []int64, limit int) (*OpponentReport, error) {
	opponent, err := u.teams.GetByID(ctx, opponentID)
	if err != nil {
		return nil, fmt.Errorf("equipe não encontrada: %w", err)
	}
	league, err := u.leagues.GetByID(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("campeonato não encontrado: %w", err)
	}

	aggregates, err := u.stats.TeamAggregates(ctx, leagueID, seasonIDs, limit)
	if err != nil {
		return nil, err
	}
	if len(aggregates) == 0 {
		return nil, fmt.Errorf("sem dados suficientes para calcular o ranking da liga")
	}

	sort.Slice(aggregates, func(i, j int) bool {
		return aggregates[i].AvgAgainst > aggregates[j].AvgAgainst
	})

	rank := 0
	var avgConceded float64
	gamesAnalyzed := 0
	for i, a := range aggregates {
		if a.Team.ID == opponentID {
			rank = i + 1
			avgConceded = a.AvgAgainst
			gamesAnalyzed = a.SampleSize
			break
		}
	}
	if rank == 0 {
		return nil, fmt.Errorf("equipe não possui jogos suficientes no período selecionado")
	}

	return &OpponentReport{
		OpponentID:   opponent.ID,
		OpponentName: opponent.Name,
		AvgConceded:  avgConceded,
		LeagueRank:   rank,
		LeagueSize:   len(aggregates),
		RankLabel:    fmt.Sprintf("%dº pior da liga em escanteios cedidos", rank),
		Meta: Meta{
			LeagueID:      league.ID,
			LeagueName:    league.Name,
			SeasonIDs:     seasonIDs,
			Period:        periodLabel(limit),
			GamesAnalyzed: gamesAnalyzed,
			UpdatedAt:     time.Now(),
		},
	}, nil
}

func periodLabel(limit int) string {
	if limit <= 0 {
		return "Todos os jogos do período selecionado"
	}
	return fmt.Sprintf("Últimos %d jogos", limit)
}
