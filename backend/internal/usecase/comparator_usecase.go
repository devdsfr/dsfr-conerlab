package usecase

import (
	"context"
	"fmt"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

type ComparatorUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
}

func NewComparatorUsecase(matches repository.MatchRepository, teams repository.TeamRepository) *ComparatorUsecase {
	return &ComparatorUsecase{matches: matches, teams: teams}
}

type TeamComparisonSide struct {
	Team       domain.Team `json:"team"`
	SampleSize int         `json:"sample_size"`

	// Period descreve a amostra REAL desta equipe, e fica DENTRO do lado porque
	// as duas equipes podem ter amostras diferentes.
	//
	// Antes havia um único `period` no topo, com a janela pedida: dois times com
	// 20 e 13 jogos apareciam ambos sob "Últimos 20 jogos". Comparar médias de
	// amostras diferentes já exige cuidado; esconder que são diferentes torna a
	// comparação enganosa.
	Period string `json:"period"`

	TotalCorners   StatSummary `json:"total_corners"`
	CornersFor     StatSummary `json:"corners_for"`
	CornersAgainst StatSummary `json:"corners_against"`
	Home           *SplitStats `json:"home"`
	Away           *SplitStats `json:"away"`
	Trend          []int       `json:"trend"`
}

type ComparisonResult struct {
	// Period do conjunto: a janela PEDIDA, explicitamente rotulada como pedido.
	// A amostra real de cada equipe está em TeamA.Period / TeamB.Period.
	Period         string             `json:"period"`
	RequestedLimit int                `json:"requested_limit"`
	TeamA          TeamComparisonSide `json:"team_a"`
	TeamB          TeamComparisonSide `json:"team_b"`
}

func (u *ComparatorUsecase) Compare(ctx context.Context, teamAID, teamBID int64, leagueID *int64, seasonID *int64, limit int) (*ComparisonResult, error) {
	if limit <= 0 {
		limit = 10
	}

	sideA, err := u.buildSide(ctx, teamAID, leagueID, seasonID, limit)
	if err != nil {
		return nil, fmt.Errorf("equipe A: %w", err)
	}
	sideB, err := u.buildSide(ctx, teamBID, leagueID, seasonID, limit)
	if err != nil {
		return nil, fmt.Errorf("equipe B: %w", err)
	}

	return &ComparisonResult{
		Period:         fmt.Sprintf("Janela pedida: últimos %d jogos", limit),
		RequestedLimit: limit,
		TeamA:          *sideA,
		TeamB:          *sideB,
	}, nil
}

// seasonID é repassado ao repositório. Sem ele, "últimos 20 jogos da La Liga"
// atravessava a fronteira de temporada: em 23/09/2026 o Celta Vigo devolvia 20
// jogos que eram 7 de 2026 + 13 de 2025, com média 7.9 — número que não
// corresponde a nenhuma das duas temporadas (2026: 8.0; 2025: 8.15). Misturar
// temporadas sem dizer é apresentar um recorte que não existe.
func (u *ComparatorUsecase) buildSide(ctx context.Context, teamID int64, leagueID *int64, seasonID *int64, limit int) (*TeamComparisonSide, error) {
	team, err := u.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	views, err := u.matches.TeamMatches(ctx, repository.MatchFilter{
		TeamID:   teamID,
		LeagueID: leagueID,
		SeasonID: seasonID,
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}

	total := make([]int, 0, len(views))
	forVals := make([]int, 0, len(views))
	againstVals := make([]int, 0, len(views))
	homeVals := []int{}
	awayVals := []int{}
	trend := make([]int, 0, len(views))

	for _, v := range views {
		total = append(total, v.TotalCorners)
		forVals = append(forVals, v.CornersFor)
		againstVals = append(againstVals, v.CornersAgainst)
		if v.IsHome {
			homeVals = append(homeVals, v.TotalCorners)
		} else {
			awayVals = append(awayVals, v.TotalCorners)
		}
	}
	for i := len(views) - 1; i >= 0; i-- {
		trend = append(trend, views[i].TotalCorners)
	}

	return &TeamComparisonSide{
		Team:       *team,
		SampleSize: len(views),
		// Mesma semântica consolidada no REV-P1 (describePeriod): a frase
		// descreve o que foi analisado, não o que foi pedido.
		Period:         describePeriod(len(views), limit),
		TotalCorners:   Summarize(total),
		CornersFor:     Summarize(forVals),
		CornersAgainst: Summarize(againstVals),
		Home:           buildSplitStats(homeVals),
		Away:           buildSplitStats(awayVals),
		Trend:          trend,
	}, nil
}
