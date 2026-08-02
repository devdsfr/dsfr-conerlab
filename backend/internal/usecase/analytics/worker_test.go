package analytics

import (
	"math"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

func iptr(v int) *int { return &v }

// views em ordem mais recente → mais antigo (contrato do MatchRepository).
func sampleViews() []domain.TeamMatchView {
	return []domain.TeamMatchView{
		{IsHome: true, CornersFor: 9, CornersAgainst: 1, GoalsFor: 0, GoalsAgainst: 1, ShotsFor: iptr(16), ShotsAgainst: iptr(8)},
		{IsHome: false, CornersFor: 7, CornersAgainst: 5, GoalsFor: 2, GoalsAgainst: 1, ShotsFor: iptr(13), ShotsAgainst: iptr(21)},
		{IsHome: true, CornersFor: 5, CornersAgainst: 4, GoalsFor: 1, GoalsAgainst: 1}, // sem chutes (nullable)
		{IsHome: false, CornersFor: 8, CornersAgainst: 6, GoalsFor: 2, GoalsAgainst: 1, ShotsFor: iptr(20), ShotsAgainst: iptr(8)},
	}
}

func TestBuildMetricsCorners(t *testing.T) {
	ls := repository.LeagueSeason{LeagueID: 18, SeasonID: 26}
	tm := buildMetrics(368, ls, "corners", sampleViews())
	if tm == nil {
		t.Fatal("esperava métricas de escanteios")
	}
	// totais: 10, 12, 9, 14 → média 11.25
	if tm.SampleSize != 4 {
		t.Errorf("sample: got %d want 4", tm.SampleSize)
	}
	if math.Abs(*tm.AvgTotal-11.25) > 0.01 {
		t.Errorf("avg_total: got %v want 11.25", *tm.AvgTotal)
	}
	// casa: 10, 9 → 9.5 | fora: 12, 14 → 13
	if math.Abs(*tm.AvgHome-9.5) > 0.01 || math.Abs(*tm.AvgAway-13) > 0.01 {
		t.Errorf("home/away: got %v/%v want 9.5/13", *tm.AvgHome, *tm.AvgAway)
	}
	// frequência acima de 9: 3 de 4 = 75%
	if got := tm.Frequencies["9"]; math.Abs(got-75) > 0.01 {
		t.Errorf("freq >9: got %v want 75", got)
	}
	if tm.AlgorithmVersion == "" {
		t.Error("algorithm_version vazio")
	}
}

func TestBuildMetricsNullable(t *testing.T) {
	ls := repository.LeagueSeason{LeagueID: 18, SeasonID: 26}
	// chutes: só 3 dos 4 jogos têm dado → amostra própria menor
	tm := buildMetrics(368, ls, "shots", sampleViews())
	if tm == nil {
		t.Fatal("esperava métricas de chutes")
	}
	if tm.SampleSize != 3 {
		t.Errorf("sample de chutes: got %d want 3 (1 jogo sem dado)", tm.SampleSize)
	}
	// totais: 24, 34, 28 → média 28.67
	if math.Abs(*tm.AvgTotal-28.67) > 0.01 {
		t.Errorf("avg_total chutes: got %v want 28.67", *tm.AvgTotal)
	}

	// métrica 100% ausente → nil (não grava linha vazia)
	if got := buildMetrics(368, ls, "offsides", sampleViews()); got != nil {
		t.Error("offsides sem nenhum dado deveria retornar nil")
	}
}

func TestWindowDelta(t *testing.T) {
	totals := []float64{12, 12, 12, 12, 12, 8, 8, 8, 8, 8} // recente 12, antigo 8 → média 10
	d5 := windowDelta(totals, 5, 10)
	if math.Abs(d5-0.2) > 0.001 { // (12-10)/10
		t.Errorf("delta5: got %v want 0.2", d5)
	}
	if d := windowDelta(totals, 20, 10); math.Abs(d) > 0.001 { // janela = tudo → 0
		t.Errorf("delta20: got %v want 0", d)
	}
	if d := windowDelta(nil, 5, 10); d != 0 {
		t.Errorf("série vazia deveria dar 0, got %v", d)
	}
	if d := windowDelta(totals, 5, 0); d != 0 {
		t.Errorf("média geral 0 deveria dar 0, got %v", d)
	}
}
