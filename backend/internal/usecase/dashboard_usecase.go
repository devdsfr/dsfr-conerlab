package usecase

import (
	"context"
	"fmt"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

type DashboardUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
}

func NewDashboardUsecase(matches repository.MatchRepository, teams repository.TeamRepository) *DashboardUsecase {
	return &DashboardUsecase{matches: matches, teams: teams}
}

// DashboardResult representa toda a resposta do Módulo 1 (Dashboard Principal).
type DashboardResult struct {
	Team           domain.Team            `json:"team"`
	SampleSize     int                    `json:"sample_size"`
	Period         string                 `json:"period"` // descreve o período/quantidade de jogos analisados (exigência: sempre exibir o período)
	RecentMatches  []domain.TeamMatchView `json:"recent_matches"`
	CornersFor     StatSummary            `json:"corners_for"`
	CornersAgainst StatSummary            `json:"corners_against"`
	TotalCorners   StatSummary            `json:"total_corners"`
	Balance        int                    `json:"balance"` // saldo = a favor - sofridos
	Frequencies    []FrequencyResult      `json:"frequencies"`
	Trend          []int                  `json:"trend"` // últimos jogos em ordem cronológica (mais antigo -> mais recente)
	HomeStats      *SplitStats            `json:"home_stats,omitempty"`
	AwayStats      *SplitStats            `json:"away_stats,omitempty"`

	// Análise de GOLS — mesma estrutura dos escanteios (marcados/sofridos/total,
	// saldo, frequências, tendência, casa/fora), exposta em paralelo para a aba "Gols"
	// do Dashboard. Frequências usam linhas over/under (ver GoalFrequencyThresholds).
	GoalsFor        StatSummary       `json:"goals_for"`
	GoalsAgainst    StatSummary       `json:"goals_against"`
	TotalGoals      StatSummary       `json:"total_goals"`
	GoalBalance     int               `json:"goal_balance"`
	GoalFrequencies []FrequencyResult `json:"goal_frequencies"`
	GoalTrend       []int             `json:"goal_trend"`
	GoalHomeStats   *SplitStats       `json:"goal_home_stats,omitempty"`
	GoalAwayStats   *SplitStats       `json:"goal_away_stats,omitempty"`
}

type SplitStats struct {
	SampleSize  int     `json:"sample_size"`
	Mean        float64 `json:"mean"`
	Max         int     `json:"max"`
	Min         int     `json:"min"`
	Consistency float64 `json:"consistency"` // índice de consistência (1 - CV, limitado a [0,1])
}

var DefaultFrequencyThresholds = []int{4, 5, 6, 7, 8, 9, 10}

// GoalFrequencyThresholds são as linhas over/under de gols. Como FrequencyAboveThresholds
// conta valores estritamente MAIORES que o limite (v > t), o limite inteiro N equivale à
// linha "N.5": t=0 -> "acima de 0.5" (1+ gol), t=1 -> "acima de 1.5" (2+ gols), etc. O
// frontend exibe o rótulo como "{N}.5".
var GoalFrequencyThresholds = []int{0, 1, 2, 3, 4}

func (u *DashboardUsecase) GetDashboard(ctx context.Context, teamID int64, leagueID *int64, seasonID *int64, limit int) (*DashboardResult, error) {
	if limit <= 0 {
		limit = 10
	}

	team, err := u.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("equipe não encontrada: %w", err)
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

	cornersFor := make([]int, 0, len(views))
	cornersAgainst := make([]int, 0, len(views))
	totalCorners := make([]int, 0, len(views))
	trend := make([]int, 0, len(views))

	goalsFor := make([]int, 0, len(views))
	goalsAgainst := make([]int, 0, len(views))
	totalGoals := make([]int, 0, len(views))
	goalTrend := make([]int, 0, len(views))

	for _, v := range views {
		cornersFor = append(cornersFor, v.CornersFor)
		cornersAgainst = append(cornersAgainst, v.CornersAgainst)
		totalCorners = append(totalCorners, v.TotalCorners)
		goalsFor = append(goalsFor, v.GoalsFor)
		goalsAgainst = append(goalsAgainst, v.GoalsAgainst)
		totalGoals = append(totalGoals, v.TotalGoals)
	}
	// trend em ordem cronológica: views vêm mais recente -> mais antigo, então invertemos
	for i := len(views) - 1; i >= 0; i-- {
		trend = append(trend, views[i].TotalCorners)
		goalTrend = append(goalTrend, views[i].TotalGoals)
	}

	result := &DashboardResult{
		Team:           *team,
		SampleSize:     len(views),
		Period:         fmt.Sprintf("Últimos %d jogos", limit),
		RecentMatches:  views,
		CornersFor:     Summarize(cornersFor),
		CornersAgainst: Summarize(cornersAgainst),
		TotalCorners:   Summarize(totalCorners),
		Balance:        sumInts(cornersFor) - sumInts(cornersAgainst),
		Frequencies:    FrequencyAboveThresholds(totalCorners, DefaultFrequencyThresholds),
		Trend:          trend,

		GoalsFor:        Summarize(goalsFor),
		GoalsAgainst:    Summarize(goalsAgainst),
		TotalGoals:      Summarize(totalGoals),
		GoalBalance:     sumInts(goalsFor) - sumInts(goalsAgainst),
		GoalFrequencies: FrequencyAboveThresholds(totalGoals, GoalFrequencyThresholds),
		GoalTrend:       goalTrend,
	}

	// Casa x Fora — escanteios e gols em paralelo.
	homeValues := []int{}
	awayValues := []int{}
	goalHomeValues := []int{}
	goalAwayValues := []int{}
	for _, v := range views {
		if v.IsHome {
			homeValues = append(homeValues, v.TotalCorners)
			goalHomeValues = append(goalHomeValues, v.TotalGoals)
		} else {
			awayValues = append(awayValues, v.TotalCorners)
			goalAwayValues = append(goalAwayValues, v.TotalGoals)
		}
	}
	result.HomeStats = buildSplitStats(homeValues)
	result.AwayStats = buildSplitStats(awayValues)
	result.GoalHomeStats = buildSplitStats(goalHomeValues)
	result.GoalAwayStats = buildSplitStats(goalAwayValues)

	return result, nil
}

func buildSplitStats(values []int) *SplitStats {
	if len(values) == 0 {
		return &SplitStats{}
	}
	s := Summarize(values)
	return &SplitStats{
		SampleSize:  s.Count,
		Mean:        s.Mean,
		Max:         s.Max,
		Min:         s.Min,
		Consistency: s.ConsistencyIndex,
	}
}

func sumInts(values []int) int {
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum
}
