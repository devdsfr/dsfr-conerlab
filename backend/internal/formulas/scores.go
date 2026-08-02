package formulas

// Scores proprietários do CornerLab (Catálogo 22–32). Todos os componentes de
// entrada são normalizados em [0,1] pelo chamador (a normalização depende do
// contexto — ex.: ROI normalizado contra um teto configurado) e a saída é
// 0..100. Componentes "quanto menor, melhor" (variância, drawdown, risco)
// devem ser passados já invertidos (1 − normalizado), como indicado em cada
// assinatura. Pesos exportados para auditoria — mudá-los exige nova versão do
// Formula Catalog.

// Pesos do ConsistencyIndex (Catálogo 22).
const (
	ConsistencyWWinRate    = 0.40
	ConsistencyWVariance   = 0.20
	ConsistencyWDrawdown   = 0.20
	ConsistencyWRobustness = 0.20
)

// ConsistencyIndex (Catálogo 22) — índice de consistência 0..100.
//
//	40% win rate + 20% (1−variância) + 20% (1−drawdown) + 20% robustez
//
// invVariance e invDrawdown já invertidos: 1 = baixa variância / sem drawdown.
func ConsistencyIndex(winRate, invVariance, invDrawdown, robustness float64) float64 {
	return 100 * (ConsistencyWWinRate*clamp01(winRate) +
		ConsistencyWVariance*clamp01(invVariance) +
		ConsistencyWDrawdown*clamp01(invDrawdown) +
		ConsistencyWRobustness*clamp01(robustness))
}

// ConfidenceScore (Catálogo 23) — confiança estatística 0..100, média
// igualitária de: volume de jogos, consistência, (1−variância) e robustez
// temporal, todos normalizados.
func ConfidenceScore(sampleSizeNorm, consistencyNorm, invVariance, temporalRobustness float64) float64 {
	return 100 * (clamp01(sampleSizeNorm) +
		clamp01(consistencyNorm) +
		clamp01(invVariance) +
		clamp01(temporalRobustness)) / 4
}

// DSFRInputs agrupa os componentes normalizados [0,1] do DSFR Score.
// InvDrawdown e InvVariance já invertidos (1 = melhor).
type DSFRInputs struct {
	ROI         float64
	EV          float64
	WinRate     float64
	Yield       float64
	InvDrawdown float64
	SampleSize  float64
	Consistency float64
	InvVariance float64
}

// Pesos do DSFRScore (Catálogo 24).
const (
	DSFRWROI         = 0.20
	DSFRWEV          = 0.20
	DSFRWWinRate     = 0.15
	DSFRWYield       = 0.10
	DSFRWDrawdown    = 0.10
	DSFRWSampleSize  = 0.10
	DSFRWConsistency = 0.10
	DSFRWVariance    = 0.05
)

// DSFRScore (Catálogo 24) — score proprietário 0..100 que resume a qualidade
// geral de uma estratégia.
func DSFRScore(in DSFRInputs) float64 {
	return 100 * (DSFRWROI*clamp01(in.ROI) +
		DSFRWEV*clamp01(in.EV) +
		DSFRWWinRate*clamp01(in.WinRate) +
		DSFRWYield*clamp01(in.Yield) +
		DSFRWDrawdown*clamp01(in.InvDrawdown) +
		DSFRWSampleSize*clamp01(in.SampleSize) +
		DSFRWConsistency*clamp01(in.Consistency) +
		DSFRWVariance*clamp01(in.InvVariance))
}

// HealthScore (Catálogo 25) — saúde da estratégia a partir das variações
// recentes (Δ = período recente − período anterior, normalizados em [-1,1]).
// ΔDrawdown entra invertido (drawdown subindo = saúde caindo). Saída 0..100,
// onde 50 = estável, >50 melhorando, <50 piorando.
func HealthScore(deltaROI, deltaEV, deltaDrawdown, deltaConsistency float64) float64 {
	clampD := func(v float64) float64 {
		if v < -1 {
			return -1
		}
		if v > 1 {
			return 1
		}
		return v
	}
	avg := (clampD(deltaROI) + clampD(deltaEV) + clampD(-deltaDrawdown) + clampD(deltaConsistency)) / 4
	return 50 + 50*avg
}

// Pesos do OpportunityScore (Catálogo 26).
const (
	OpportunityWHealth      = 0.30
	OpportunityWDSFR        = 0.25
	OpportunityWROI         = 0.20
	OpportunityWWinRate     = 0.15
	OpportunityWConsistency = 0.10
)

// OpportunityScore (Catálogo 26) — prioriza o que merece atenção agora.
// health e dsfr em escala 0..100; demais componentes normalizados [0,1].
func OpportunityScore(health, dsfr, roiNorm, winRate, consistencyNorm float64) float64 {
	return OpportunityWHealth*clamp01(health/100)*100 +
		OpportunityWDSFR*clamp01(dsfr/100)*100 +
		OpportunityWROI*clamp01(roiNorm)*100 +
		OpportunityWWinRate*clamp01(winRate)*100 +
		OpportunityWConsistency*clamp01(consistencyNorm)*100
}

// Stage é o estágio de vida de uma estratégia (Catálogo 27).
type Stage string

const (
	StageBirth    Stage = "nascimento"
	StageGrowth   Stage = "crescimento"
	StageMaturity Stage = "maturidade"
	StageDecline  Stage = "declinio"
	StageObsolete Stage = "obsoleta"
)

// LifecycleStage (Catálogo 27) — classifica o estágio de vida:
// amostra pequena = nascimento; melhorando = crescimento; estável e saudável =
// maturidade; piorando = declínio; saúde crítica = obsoleta.
// minSample é o tamanho mínimo de amostra para sair de "nascimento".
func LifecycleStage(sampleSize, minSample int, health, trend float64) Stage {
	if sampleSize < minSample {
		return StageBirth
	}
	switch {
	case health < 25:
		return StageObsolete
	case health < 45 || trend < -0.15:
		return StageDecline
	case trend > 0.15:
		return StageGrowth
	default:
		return StageMaturity
	}
}

// Pesos do RankingScore (Catálogo 28) — ordem de prioridade do catálogo:
// DSFR > Health > ROI > Yield > Confidence.
const (
	RankingWDSFR       = 0.35
	RankingWHealth     = 0.25
	RankingWROI        = 0.20
	RankingWYield      = 0.10
	RankingWConfidence = 0.10
)

// RankingScore (Catálogo 28) — chave única de ordenação de estratégias.
// dsfr, health e confidence em 0..100; roiNorm e yieldNorm em [0,1].
func RankingScore(dsfr, health, roiNorm, yieldNorm, confidence float64) float64 {
	return RankingWDSFR*clamp01(dsfr/100)*100 +
		RankingWHealth*clamp01(health/100)*100 +
		RankingWROI*clamp01(roiNorm)*100 +
		RankingWYield*clamp01(yieldNorm)*100 +
		RankingWConfidence*clamp01(confidence/100)*100
}

// Pesos do TrendScore (Catálogo 29) — janelas recentes pesam mais.
const (
	TrendWLast5  = 0.50
	TrendWLast10 = 0.30
	TrendWLast20 = 0.20
)

// TrendScore (Catálogo 29) — inclinação da performance em [-1,1]:
// média ponderada da variação normalizada nas janelas de 5, 10 e 20 jogos
// (cada delta em [-1,1]; ex.: variação da taxa de acerto na janela).
func TrendScore(delta5, delta10, delta20 float64) float64 {
	clampD := func(v float64) float64 {
		if v < -1 {
			return -1
		}
		if v > 1 {
			return 1
		}
		return v
	}
	return TrendWLast5*clampD(delta5) + TrendWLast10*clampD(delta10) + TrendWLast20*clampD(delta20)
}

// RobustnessScore (Catálogo 30) — solidez estatística 0..100, média
// igualitária de: volume de jogos, consistência, baixa variância (invertida),
// ROI e histórico temporal — todos normalizados [0,1].
func RobustnessScore(sampleSizeNorm, consistencyNorm, invVariance, roiNorm, temporalNorm float64) float64 {
	return 100 * (clamp01(sampleSizeNorm) +
		clamp01(consistencyNorm) +
		clamp01(invVariance) +
		clamp01(roiNorm) +
		clamp01(temporalNorm)) / 5
}

// VolatilityScore (Catálogo 31) — instabilidade 0..100 (quanto maior, mais
// volátil): média de desvio padrão, oscilação de ROI e oscilação de EV,
// normalizados [0,1].
func VolatilityScore(stdDevNorm, roiOscNorm, evOscNorm float64) float64 {
	return 100 * (clamp01(stdDevNorm) + clamp01(roiOscNorm) + clamp01(evOscNorm)) / 3
}

// RiskScore (Catálogo 32) — risco 0..100 (quanto maior, mais arriscada):
// média de drawdown, variância, volatilidade e loss rate, normalizados [0,1].
func RiskScore(drawdownNorm, varianceNorm, volatilityNorm, lossRate float64) float64 {
	return 100 * (clamp01(drawdownNorm) + clamp01(varianceNorm) +
		clamp01(volatilityNorm) + clamp01(lossRate)) / 4
}
