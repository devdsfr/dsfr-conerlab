package formulas

import "testing"

func TestConsistencyIndex(t *testing.T) {
	cases := []struct{ wr, iv, id, rob, want float64 }{
		{1, 1, 1, 1, 100}, // perfeito
		{0, 0, 0, 0, 0},   // pior caso
		{0.5, 0.5, 0.5, 0.5, 50},
		{1, 0, 0, 0, 40}, // só win rate
		{0, 1, 0, 0, 20}, // só variância invertida
		{0, 0, 1, 0, 20},
		{0, 0, 0, 1, 20},
		{0.8, 0.6, 0.7, 0.9, 76},
		{1.5, 1, 1, 1, 100}, // clamp acima
		{-0.5, 0, 0, 0, 0},  // clamp abaixo
	}
	for _, c := range cases {
		approx(t, ConsistencyIndex(c.wr, c.iv, c.id, c.rob), c.want, "ConsistencyIndex")
	}
	// soma dos pesos deve ser 1 (auditoria do catálogo)
	approx(t, ConsistencyWWinRate+ConsistencyWVariance+ConsistencyWDrawdown+ConsistencyWRobustness, 1, "pesos Consistency")
}

func TestConfidenceScore(t *testing.T) {
	cases := []struct{ n, cons, iv, temp, want float64 }{
		{1, 1, 1, 1, 100},
		{0, 0, 0, 0, 0},
		{0.5, 0.5, 0.5, 0.5, 50},
		{1, 0, 0, 0, 25},
		{0.4, 0.8, 0.6, 0.2, 50},
		{2, 1, 1, 1, 100}, // clamp
	}
	for _, c := range cases {
		got := ConfidenceScore(c.n, c.cons, c.iv, c.temp)
		approx(t, got, c.want, "ConfidenceScore")
		if got < 0 || got > 100 {
			t.Errorf("fora da escala 0-100: %v", got) // escala do catálogo
		}
	}
}

func TestDSFRScore(t *testing.T) {
	// pesos somam 1
	approx(t, DSFRWROI+DSFRWEV+DSFRWWinRate+DSFRWYield+DSFRWDrawdown+DSFRWSampleSize+DSFRWConsistency+DSFRWVariance, 1, "pesos DSFR")

	perfect := DSFRInputs{1, 1, 1, 1, 1, 1, 1, 1}
	approx(t, DSFRScore(perfect), 100, "DSFR perfeito")
	approx(t, DSFRScore(DSFRInputs{}), 0, "DSFR zerado")
	// componente isolado = seu peso × 100
	approx(t, DSFRScore(DSFRInputs{ROI: 1}), 20, "DSFR só ROI")
	approx(t, DSFRScore(DSFRInputs{EV: 1}), 20, "DSFR só EV")
	approx(t, DSFRScore(DSFRInputs{WinRate: 1}), 15, "DSFR só WinRate")
	approx(t, DSFRScore(DSFRInputs{Yield: 1}), 10, "DSFR só Yield")
	approx(t, DSFRScore(DSFRInputs{InvDrawdown: 1}), 10, "DSFR só Drawdown")
	approx(t, DSFRScore(DSFRInputs{SampleSize: 1}), 10, "DSFR só Jogos")
	approx(t, DSFRScore(DSFRInputs{Consistency: 1}), 10, "DSFR só Consistência")
	approx(t, DSFRScore(DSFRInputs{InvVariance: 1}), 5, "DSFR só Variância")
	// meio a meio
	half := DSFRInputs{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
	approx(t, DSFRScore(half), 50, "DSFR médio")
}

func TestHealthScore(t *testing.T) {
	cases := []struct{ dROI, dEV, dDD, dCons, want float64 }{
		{0, 0, 0, 0, 50},   // estável
		{1, 1, -1, 1, 100}, // tudo melhorando (drawdown caindo)
		{-1, -1, 1, -1, 0}, // tudo piorando
		{0.5, 0.5, 0, 0.5, 68.75},
		{1, 0, 0, 0, 62.5},
		{0, 0, 1, 0, 37.5}, // só drawdown subiu → piora
		{2, 2, -2, 2, 100}, // clamp
		{-2, -2, 2, -2, 0}, // clamp
		{0.2, -0.2, 0, 0, 50},
		{0, 0, -1, 0, 62.5}, // drawdown caiu → melhora
	}
	for _, c := range cases {
		got := HealthScore(c.dROI, c.dEV, c.dDD, c.dCons)
		approx(t, got, c.want, "HealthScore")
		if got < 0 || got > 100 {
			t.Errorf("fora da escala 0-100: %v", got)
		}
	}
}

func TestOpportunityScore(t *testing.T) {
	approx(t, OpportunityWHealth+OpportunityWDSFR+OpportunityWROI+OpportunityWWinRate+OpportunityWConsistency, 1, "pesos Opportunity")
	approx(t, OpportunityScore(100, 100, 1, 1, 1), 100, "Opportunity máximo")
	approx(t, OpportunityScore(0, 0, 0, 0, 0), 0, "Opportunity mínimo")
	approx(t, OpportunityScore(100, 0, 0, 0, 0), 30, "só Health")
	approx(t, OpportunityScore(0, 100, 0, 0, 0), 25, "só DSFR")
	approx(t, OpportunityScore(0, 0, 1, 0, 0), 20, "só ROI")
	approx(t, OpportunityScore(0, 0, 0, 1, 0), 15, "só WinRate")
	approx(t, OpportunityScore(0, 0, 0, 0, 1), 10, "só Consistência")
	approx(t, OpportunityScore(50, 50, 0.5, 0.5, 0.5), 50, "médio")
}

func TestLifecycleStage(t *testing.T) {
	cases := []struct {
		sample, min int
		health      float64
		trend       float64
		want        Stage
	}{
		{5, 30, 90, 0.5, StageBirth},      // amostra pequena vence tudo
		{50, 30, 80, 0.5, StageGrowth},    // melhorando
		{50, 30, 80, 0, StageMaturity},    // estável e saudável
		{50, 30, 40, 0, StageDecline},     // saúde baixa
		{50, 30, 60, -0.3, StageDecline},  // tendência ruim
		{50, 30, 10, 0, StageObsolete},    // saúde crítica
		{30, 30, 50, 0, StageMaturity},    // amostra exatamente no mínimo
		{100, 30, 20, 0.9, StageObsolete}, // saúde crítica mesmo com trend alto
	}
	for i, c := range cases {
		if got := LifecycleStage(c.sample, c.min, c.health, c.trend); got != c.want {
			t.Errorf("caso %d: got %s want %s", i, got, c.want)
		}
	}
}

func TestRankingScore(t *testing.T) {
	approx(t, RankingWDSFR+RankingWHealth+RankingWROI+RankingWYield+RankingWConfidence, 1, "pesos Ranking")
	approx(t, RankingScore(100, 100, 1, 1, 100), 100, "Ranking máximo")
	approx(t, RankingScore(0, 0, 0, 0, 0), 0, "Ranking mínimo")
	// ordem de prioridade do catálogo: DSFR pesa mais que Health, que pesa mais que ROI...
	if !(RankingWDSFR > RankingWHealth && RankingWHealth > RankingWROI &&
		RankingWROI > RankingWYield && RankingWYield >= RankingWConfidence) {
		t.Error("prioridade de pesos do Ranking não respeita o catálogo")
	}
	// estratégia com DSFR maior deve ranquear acima (demais iguais)
	a := RankingScore(80, 50, 0.5, 0.5, 50)
	b := RankingScore(60, 50, 0.5, 0.5, 50)
	if a <= b {
		t.Error("DSFR maior deveria ranquear acima")
	}
}

func TestTrendScore(t *testing.T) {
	approx(t, TrendWLast5+TrendWLast10+TrendWLast20, 1, "pesos Trend")
	cases := []struct{ d5, d10, d20, want float64 }{
		{0, 0, 0, 0},
		{1, 1, 1, 1},
		{-1, -1, -1, -1},
		{1, 0, 0, 0.5}, // janela recente pesa mais
		{0, 1, 0, 0.3},
		{0, 0, 1, 0.2},
		{0.5, 0.5, 0.5, 0.5},
		{2, 2, 2, 1},     // clamp
		{-2, 0, 0, -0.5}, // clamp por componente
		{1, -1, 0, 0.2},
	}
	for _, c := range cases {
		got := TrendScore(c.d5, c.d10, c.d20)
		approx(t, got, c.want, "TrendScore")
		if got < -1 || got > 1 {
			t.Errorf("fora de [-1,1]: %v", got)
		}
	}
}

func TestAggregateScores(t *testing.T) {
	// Robustness / Volatility / Risk: médias igualitárias 0..100 com clamp.
	approx(t, RobustnessScore(1, 1, 1, 1, 1), 100, "Robustness máximo")
	approx(t, RobustnessScore(0, 0, 0, 0, 0), 0, "Robustness mínimo")
	approx(t, RobustnessScore(0.5, 0.5, 0.5, 0.5, 0.5), 50, "Robustness médio")
	approx(t, RobustnessScore(1, 0, 0, 0, 0), 20, "Robustness um componente")

	approx(t, VolatilityScore(1, 1, 1), 100, "Volatility máximo")
	approx(t, VolatilityScore(0, 0, 0), 0, "Volatility mínimo")
	approx(t, VolatilityScore(0.3, 0.6, 0.9), 60, "Volatility misto")

	approx(t, RiskScore(1, 1, 1, 1), 100, "Risk máximo")
	approx(t, RiskScore(0, 0, 0, 0), 0, "Risk mínimo")
	approx(t, RiskScore(0.4, 0.4, 0.4, 0.4), 40, "Risk médio")
	approx(t, RiskScore(2, 2, 2, 2), 100, "Risk clamp")
}
