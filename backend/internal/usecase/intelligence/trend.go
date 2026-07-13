package intelligence

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

type TrendUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
}

func NewTrendUsecase(matches repository.MatchRepository, teams repository.TeamRepository, leagues repository.LeagueRepository) *TrendUsecase {
	return &TrendUsecase{matches: matches, teams: teams, leagues: leagues}
}

// TrendReport compara a média de escanteios totais entre uma janela mais antiga e
// uma janela mais recente (ex: últimos 5 vs últimos 10 jogos), reportando a variação
// percentual e sinalizando se a tendência é de queda (negativa).
type TrendReport struct {
	TeamID        int64   `json:"team_id"`
	TeamName      string  `json:"team_name"`
	PreviousLabel string  `json:"previous_label"`
	CurrentLabel  string  `json:"current_label"`
	PreviousAvg   float64 `json:"previous_avg"`
	CurrentAvg    float64 `json:"current_avg"`
	VariationPct  float64 `json:"variation_pct"`
	IsNegative    bool    `json:"is_negative"`
	Direction     string  `json:"direction"` // "alta" | "queda" | "estável"
	Meta          Meta    `json:"meta"`
}

// Compute compara a janela de `shortWindow` jogos mais recentes (ex: 5) com a janela
// de `longWindow` jogos mais recentes (ex: 10) — a janela curta está contida na longa,
// então "anterior" aqui representa os jogos da janela longa que ficaram de fora da
// janela curta (ou seja, o desempenho "antes" do recorte mais recente).
func (u *TrendUsecase) Compute(ctx context.Context, teamID int64, leagueID int64, shortWindow, longWindow int) (*TrendReport, error) {
	if shortWindow <= 0 {
		shortWindow = 5
	}
	if longWindow <= 0 || longWindow <= shortWindow {
		longWindow = shortWindow * 2
	}

	team, err := u.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("equipe não encontrada: %w", err)
	}
	league, err := u.leagues.GetByID(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("campeonato não encontrado: %w", err)
	}

	views, err := u.matches.TeamMatches(ctx, repository.MatchFilter{TeamID: teamID, LeagueID: &leagueID, Limit: longWindow})
	if err != nil {
		return nil, err
	}
	// views vem em ordem "mais recente -> mais antigo".
	recent := make([]int, 0, shortWindow)
	for i := 0; i < len(views) && i < shortWindow; i++ {
		recent = append(recent, views[i].TotalCorners)
	}
	older := make([]int, 0)
	for i := shortWindow; i < len(views); i++ {
		older = append(older, views[i].TotalCorners)
	}

	currentSummary := usecase.Summarize(recent)
	previousSummary := usecase.Summarize(older)

	variation := 0.0
	if previousSummary.Mean != 0 {
		variation = round2(100 * (currentSummary.Mean - previousSummary.Mean) / previousSummary.Mean)
	}

	direction := "estável"
	if variation > 1 {
		direction = "alta"
	} else if variation < -1 {
		direction = "queda"
	}

	return &TrendReport{
		TeamID:        team.ID,
		TeamName:      team.Name,
		PreviousLabel: fmt.Sprintf("Jogos %d a %d (mais antigos)", shortWindow+1, len(views)),
		CurrentLabel:  fmt.Sprintf("Últimos %d jogos", shortWindow),
		PreviousAvg:   previousSummary.Mean,
		CurrentAvg:    currentSummary.Mean,
		VariationPct:  variation,
		IsNegative:    variation < -1,
		Direction:     direction,
		Meta: Meta{
			LeagueID:      league.ID,
			LeagueName:    league.Name,
			Period:        fmt.Sprintf("Últimos %d jogos (comparando %d vs %d)", longWindow, shortWindow, longWindow),
			GamesAnalyzed: len(views),
			UpdatedAt:     time.Now(),
		},
	}, nil
}
