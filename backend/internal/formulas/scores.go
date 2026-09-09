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

// ---------------------------------------------------------------------------
// Formula Catalog v1.1 — correção do AUD-002
//
// PROBLEMA (auditoria, AUD-002): no CornerLab, ROI e Yield são o MESMO NÚMERO.
// Não por erro de digitação, mas por identidade estrutural: o motor de backtest
// aposta stake constante e liquida toda entrada, então "investimento"
// (Catálogo 07) e "volume apostado" (Catálogo 08) são a mesma quantidade. E o
// EV nunca foi calculado — o Strategy Engine atribuía o yield ao campo EV.
//
// Resultado: o DSFR v1.0 punha 50% do peso (ROI 20% + EV 20% + Yield 10%) em
// uma única quantidade — lucro/volume — apresentada como três dimensões
// independentes.
//
// POR QUE NÃO FOI "CONSERTADO" MANTENDO OS TRÊS SLOTS:
//
//   - Um Yield genuinamente distinto de ROI exige stake variável por entrada.
//     O motor não tem política de staking; inventar uma para diferenciar as
//     métricas seria fabricar dado.
//   - Um EV genuinamente distinto exige P(vitória) estimada por um modelo
//     independente e FORA da amostra. Usando a taxa de acerto observada e a
//     odd média do próprio lote, EV colapsa algebricamente no ROI realizado —
//     o mesmo número com outro nome. Estimativa fora da amostra é o AUD-003 e
//     ainda não existe.
//
// Então a v1.1 faz o que é honesto hoje: PARA de contar a mesma quantidade três
// vezes. EV e Yield saem do DSFR e do Ranking; o peso é redistribuído entre as
// dimensões que de fato carregam informação distinta.
//
// A v1.0 continua aqui, intacta e usada por nada: linhas de `backtests` gravadas
// antes desta versão trazem algorithm_version = "1.0" e só podem ser reproduzidas
// pela fórmula da época. Comparar score entre versões é inválido.
//
// PENDÊNCIAS CONHECIDAS que a v1.1 NÃO resolve (têm AUD próprio, corrigir na
// ordem de prioridade — não antecipar aqui):
//
//	AUD-007 — InvVariance = 1 − 4p(1−p) é função pura de WinRate, e Consistency
//	          é composta dos outros quatro componentes. Ou seja: mesmo na v1.1
//	          as dimensões não são ortogonais.
//	AUD-008 — TrendScore recebe o mesmo delta nas três janelas.
// ---------------------------------------------------------------------------

// DSFRInputsV11 agrupa os componentes normalizados [0,1] do DSFR v1.1.
// Não tem EV nem Yield — ver o bloco acima.
// InvDrawdown e InvVariance já invertidos (1 = melhor).
type DSFRInputsV11 struct {
	ROI         float64
	WinRate     float64
	InvDrawdown float64
	SampleSize  float64
	Consistency float64
	InvVariance float64
}

// Pesos do DSFRScoreV11 (Catálogo 24, v1.1). Somam 1.
//
// Redistribuição dos 30 pontos liberados por EV (20) e Yield (10): o peso NÃO
// foi devolvido ao ROI, o que apenas reconcentraria a mesma quantidade. Foi
// espalhado entre as dimensões restantes, mantendo a ordem de importância
// declarada no doc 08 (retorno > acerto > risco/robustez > variância).
const (
	DSFRv11WROI         = 0.30
	DSFRv11WWinRate     = 0.20
	DSFRv11WDrawdown    = 0.15
	DSFRv11WSampleSize  = 0.15
	DSFRv11WConsistency = 0.15
	DSFRv11WVariance    = 0.05
)

// DSFRScoreV11 (Catálogo 24, v1.1) — score proprietário 0..100.
func DSFRScoreV11(in DSFRInputsV11) float64 {
	return 100 * (DSFRv11WROI*clamp01(in.ROI) +
		DSFRv11WWinRate*clamp01(in.WinRate) +
		DSFRv11WDrawdown*clamp01(in.InvDrawdown) +
		DSFRv11WSampleSize*clamp01(in.SampleSize) +
		DSFRv11WConsistency*clamp01(in.Consistency) +
		DSFRv11WVariance*clamp01(in.InvVariance))
}

// Pesos do RankingScoreV11 (Catálogo 28, v1.1). Somam 1.
//
// O yield saiu pelo mesmo motivo do DSFR: no v1.0 o ranking somava ROI 20% +
// Yield 10% sobre a mesma quantidade, ainda por cima em cima de um DSFR que já
// era 50% dela. Os 10 pontos foram para Confidence, a única entrada do ranking
// que não deriva de retorno.
const (
	Rankingv11WDSFR       = 0.40
	Rankingv11WHealth     = 0.25
	Rankingv11WROI        = 0.20
	Rankingv11WConfidence = 0.15
)

// RankingScoreV11 (Catálogo 28, v1.1) — chave única de ordenação de estratégias.
// dsfr, health e confidence em 0..100; roiNorm em [0,1].
func RankingScoreV11(dsfr, health, roiNorm, confidence float64) float64 {
	return Rankingv11WDSFR*clamp01(dsfr/100)*100 +
		Rankingv11WHealth*clamp01(health/100)*100 +
		Rankingv11WROI*clamp01(roiNorm)*100 +
		Rankingv11WConfidence*clamp01(confidence/100)*100
}

// HealthScoreV11 (Catálogo 25, v1.1) — saúde a partir das variações recentes.
//
// O v1.0 promediava quatro deltas, mas ΔEV era ΔYield reescalado, isto é, o
// mesmo ΔROI: metade da "saúde" era uma variável só. A v1.1 promedia os três
// deltas que existem de verdade. ΔDrawdown entra invertido (drawdown subindo =
// saúde caindo). Saída 0..100, onde 50 = estável, >50 melhorando, <50 piorando.
func HealthScoreV11(deltaROI, deltaDrawdown, deltaConsistency float64) float64 {
	clampD := func(v float64) float64 {
		if v < -1 {
			return -1
		}
		if v > 1 {
			return 1
		}
		return v
	}
	avg := (clampD(deltaROI) + clampD(-deltaDrawdown) + clampD(deltaConsistency)) / 3
	return 50 + 50*avg
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
