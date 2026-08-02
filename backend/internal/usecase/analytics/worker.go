// Package analytics implementa o Analytics Worker da Remodelagem Fase 3
// (Remodelagem/15-workers-analytics-pipeline.md, Workers 02 e 03).
//
// Filosofia do doc 15: "O usuário consulta. Workers calculam. Nunca o
// contrário." Este worker pré-calcula as métricas por equipe (médias, janelas
// last5/10/20, casa/fora, variância, consistência, tendência e frequências)
// para as 5 métricas do Dashboard e grava em team_metrics (camada ANALYTICS,
// migration 011). Todas as contas vêm do Formula Catalog (internal/formulas).
package analytics

import (
	"context"
	"fmt"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

// Metrics calculadas por equipe — mesmas chaves usadas no Dashboard/Simulador.
var metricNames = []string{"corners", "goals", "offsides", "shots", "shots_on_target"}

// thresholds de frequência por métrica — mesmos do Dashboard (semântica `>`).
var metricThresholds = map[string][]int{
	"corners":         usecase.DefaultFrequencyThresholds,
	"goals":           usecase.GoalFrequencyThresholds,
	"offsides":        usecase.OffsideFrequencyThresholds,
	"shots":           usecase.ShotFrequencyThresholds,
	"shots_on_target": usecase.ShotOnTargetFrequencyThresholds,
}

type Worker struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
	repo    repository.AnalyticsRepository
}

func NewWorker(matches repository.MatchRepository, teams repository.TeamRepository, repo repository.AnalyticsRepository) *Worker {
	return &Worker{matches: matches, teams: teams, repo: repo}
}

// Result resume uma execução (observabilidade — doc 15).
type Result struct {
	LeagueSeasons int
	TeamsSeen     int
	MetricsSaved  int
	Errors        int
}

// Run recalcula team_metrics para todas as ligas reais com jogos finalizados.
// Erros por equipe são contados e não interrompem o ciclo (regra do worker
// legado mantida: uma falha pontual nunca derruba o pipeline inteiro).
func (w *Worker) Run(ctx context.Context) (Result, error) {
	var res Result
	pairs, err := w.repo.ListLeagueSeasons(ctx)
	if err != nil {
		return res, fmt.Errorf("listar liga/temporada: %w", err)
	}
	res.LeagueSeasons = len(pairs)

	for _, ls := range pairs {
		teams, err := w.teams.List(ctx, &ls.LeagueID, ls.SeasonID)
		if err != nil {
			res.Errors++
			continue
		}
		for _, t := range teams {
			res.TeamsSeen++
			saved, errs := w.computeTeam(ctx, t.ID, ls)
			res.MetricsSaved += saved
			res.Errors += errs
		}
	}
	return res, nil
}

// computeTeam calcula e grava as 5 métricas de uma equipe em uma liga/temporada.
func (w *Worker) computeTeam(ctx context.Context, teamID int64, ls repository.LeagueSeason) (saved, errs int) {
	views, err := w.matches.TeamMatches(ctx, repository.MatchFilter{
		TeamID:   teamID,
		LeagueID: &ls.LeagueID,
		SeasonID: &ls.SeasonID,
		// Limit 0 = todos os jogos finalizados da temporada.
	})
	if err != nil || len(views) == 0 {
		if err != nil {
			errs++
		}
		return saved, errs
	}

	for _, metric := range metricNames {
		tm := buildMetrics(teamID, ls, metric, views)
		if tm == nil {
			continue // métrica sem nenhum dado (ex.: liga sem impedimentos)
		}
		if err := w.repo.UpsertTeamMetrics(ctx, tm); err != nil {
			errs++
			continue
		}
		saved++
	}
	return saved, errs
}

// metricValues extrai (a favor, sofridos) da view para a métrica; ok=false
// quando o jogo não tem o dado (métricas nullable — mesmo critério do Dashboard).
func metricValues(v domain.TeamMatchView, metric string) (forV, againstV float64, ok bool) {
	switch metric {
	case "corners":
		return float64(v.CornersFor), float64(v.CornersAgainst), true
	case "goals":
		return float64(v.GoalsFor), float64(v.GoalsAgainst), true
	case "offsides":
		if v.OffsidesFor == nil || v.OffsidesAgainst == nil {
			return 0, 0, false
		}
		return float64(*v.OffsidesFor), float64(*v.OffsidesAgainst), true
	case "shots":
		if v.ShotsFor == nil || v.ShotsAgainst == nil {
			return 0, 0, false
		}
		return float64(*v.ShotsFor), float64(*v.ShotsAgainst), true
	case "shots_on_target":
		if v.ShotsOnTargetFor == nil || v.ShotsOnTargetAgainst == nil {
			return 0, 0, false
		}
		return float64(*v.ShotsOnTargetFor), float64(*v.ShotsOnTargetAgainst), true
	}
	return 0, 0, false
}

// buildMetrics monta o domain.TeamMetrics de uma métrica a partir dos jogos
// (views chegam do repositório em ordem mais recente → mais antigo).
func buildMetrics(teamID int64, ls repository.LeagueSeason, metric string, views []domain.TeamMatchView) *domain.TeamMetrics {
	var totals, fors, againsts, homeTotals, awayTotals []float64
	for _, v := range views {
		f, a, ok := metricValues(v, metric)
		if !ok {
			continue
		}
		total := f + a
		totals = append(totals, total)
		fors = append(fors, f)
		againsts = append(againsts, a)
		if v.IsHome {
			homeTotals = append(homeTotals, total)
		} else {
			awayTotals = append(awayTotals, total)
		}
	}
	if len(totals) == 0 {
		return nil
	}

	variance, _ := formulas.Variance(totals)
	stdDev, _ := formulas.StdDev(totals)
	avgTotal := mean(totals)

	// Consistência da equipe: 1 − coeficiente de variação, em escala 0..100 —
	// mesma definição do consistency_index já exibido no Dashboard (não confundir
	// com o ConsistencyIndex de estratégias, Catálogo 22, que pondera win rate).
	var consistency *float64
	if avgTotal > 0 {
		c := 1 - stdDev/avgTotal
		if c < 0 {
			c = 0
		}
		c *= 100
		consistency = &c
	}

	// Tendência (Catálogo 29): variação relativa das janelas recentes contra a
	// média geral, clampada em [-1,1] por componente.
	trend := formulas.TrendScore(
		windowDelta(totals, 5, avgTotal),
		windowDelta(totals, 10, avgTotal),
		windowDelta(totals, 20, avgTotal),
	)

	freqs := map[string]float64{}
	for _, th := range metricThresholds[metric] {
		count := 0
		for _, t := range totals {
			if t > float64(th) {
				count++
			}
		}
		freqs[strconv.Itoa(th)] = round2(float64(count) / float64(len(totals)) * 100)
	}

	return &domain.TeamMetrics{
		TeamID:           teamID,
		SeasonID:         ls.SeasonID,
		LeagueID:         ls.LeagueID,
		Metric:           metric,
		SampleSize:       len(totals),
		AvgTotal:         ptr(round2(avgTotal)),
		AvgFor:           ptr(round2(mean(fors))),
		AvgAgainst:       ptr(round2(mean(againsts))),
		AvgHome:          meanPtr(homeTotals),
		AvgAway:          meanPtr(awayTotals),
		Last5Avg:         windowMeanPtr(totals, 5),
		Last10Avg:        windowMeanPtr(totals, 10),
		Last20Avg:        windowMeanPtr(totals, 20),
		Variance:         ptr(round2(variance)),
		StdDev:           ptr(round2(stdDev)),
		Consistency:      consistency,
		Trend:            ptr(round3(trend)),
		Frequencies:      freqs,
		AlgorithmVersion: formulas.Version,
	}
}

// windowDelta mede a variação relativa da janela dos últimos n jogos contra a
// média geral: (média da janela − média geral) / média geral, clampada [-1,1].
func windowDelta(totals []float64, n int, overall float64) float64 {
	if overall == 0 || len(totals) == 0 {
		return 0
	}
	w := totals
	if len(w) > n {
		w = w[:n] // mais recentes primeiro
	}
	d := (mean(w) - overall) / overall
	if d > 1 {
		return 1
	}
	if d < -1 {
		return -1
	}
	return d
}

func mean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range vs {
		s += v
	}
	return s / float64(len(vs))
}

func meanPtr(vs []float64) *float64 {
	if len(vs) == 0 {
		return nil
	}
	return ptr(round2(mean(vs)))
}

func windowMeanPtr(totals []float64, n int) *float64 {
	if len(totals) == 0 {
		return nil
	}
	w := totals
	if len(w) > n {
		w = w[:n]
	}
	return ptr(round2(mean(w)))
}

func ptr(v float64) *float64 { return &v }

func round2(v float64) float64 { return float64(int(v*100+sign(v)*0.5)) / 100 }
func round3(v float64) float64 { return float64(int(v*1000+sign(v)*0.5)) / 1000 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
