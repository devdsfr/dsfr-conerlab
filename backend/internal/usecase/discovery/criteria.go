package discovery

import (
	"fmt"

	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

// Critérios mínimos de validação do doc 08 ("Critérios mínimos"). Uma combinação
// só vira estratégia publicada se passar em TODOS eles — a regra explícita do
// documento é "nunca considerar apenas Win Rate; sempre utilizar múltiplos
// indicadores".
const (
	DefaultMinGames     = 100  // jogos
	DefaultMinROI       = 10.0 // %
	DefaultMinYield     = 5.0  // %
	DefaultMinWinRate   = 75.0 // %
	DefaultMaxDrawdown  = 20.0 // % do capital movimentado
	DefaultMinDSFRScore = 40.0 // abaixo disso o doc manda descartar
	DefaultMaxPerLeague = 40   // teto de descobertas publicadas por liga

	// AbsoluteMinimumGames é a trava anti-overfitting: nunca publicar abaixo
	// disso, mesmo que o chamador peça. Exportada porque o documento de contexto
	// para IA (internal/usecase/aidocs) cita este número — assim ele nunca
	// diverge do valor real usado pelo motor.
	AbsoluteMinimumGames = 50

	// --- AUD-003: validação fora da amostra e testes múltiplos ---------------

	// DefaultTrainFraction é a fatia MAIS ANTIGA do histórico que a mineração
	// pode enxergar. Os 30% mais recentes ficam reservados para validação e não
	// entram em nenhum backtest de descoberta.
	//
	// 70/30 é a divisão convencional. O efeito colateral é intencional: a
	// mineração passa a ter menos dado, então menos combinações alcançam os 100
	// jogos do doc 08. Isso é um APERTO, não um problema a compensar — a trava
	// de amostra mínima NÃO deve ser reduzida para recuperar volume.
	DefaultTrainFraction = 0.70

	// DefaultHoldoutMinGames é a amostra mínima na janela de validação. Abaixo
	// disso o resultado fora da amostra não conclui nada, e inconclusivo reprova.
	DefaultHoldoutMinGames = 30

	// DefaultFDRq é a taxa de falsas descobertas tolerada entre as estratégias
	// publicadas: em média, até 10% das publicadas podem ser falso positivo.
	//
	// Não é o "α de 5%" de um teste isolado — aqui há milhares de testes, e o
	// que se controla é a proporção de erro no conjunto do que se publica.
	DefaultFDRq = 0.10

	// DefaultHoldoutAlpha é o α do teste de significância NA JANELA DE VALIDAÇÃO.
	// Vale sozinho, sem correção de multiplicidade, porque nessa altura o
	// conjunto de candidatos já foi fixado sem olhar para esses dados.
	DefaultHoldoutAlpha = 0.05
)

// absoluteMinimumGames mantém o nome interno usado no restante do arquivo.
const absoluteMinimumGames = AbsoluteMinimumGames

// Criteria são os limiares de aceitação de uma estratégia descoberta. Os valores
// padrão vêm do doc 08; ficam configuráveis para permitir uma varredura mais
// permissiva em ligas com histórico curto, mas a trava anti-overfitting
// (absoluteMinimumGames) não é configurável de propósito.
type Criteria struct {
	MinGames     int
	MinROI       float64
	MinYield     float64
	MinWinRate   float64
	MaxDrawdown  float64 // % do total movimentado
	MinDSFR      float64
	MaxPerLeague int

	// --- AUD-003 ------------------------------------------------------------

	// TrainFraction é a fatia mais antiga do histórico visível à mineração.
	// Fora de (0,1) cai no padrão.
	TrainFraction float64

	// HoldoutMinGames é a amostra mínima exigida na janela de validação.
	HoldoutMinGames int

	// FDRq é a taxa de falsas descobertas tolerada no conjunto publicado.
	FDRq float64

	// HoldoutAlpha é o α do teste de significância na janela de validação.
	HoldoutAlpha float64

	// FDRMethod escolhe o procedimento de correção. Vazio = Benjamini–Yekutieli
	// (conservador sob dependência arbitrária), que é o padrão do CornerLab
	// porque as combinações compartilham as mesmas partidas.
	FDRMethod formulas.MultipleTestingMethod
}

// DefaultCriteria devolve os limiares do doc 08.
func DefaultCriteria() Criteria {
	return Criteria{
		MinGames:     DefaultMinGames,
		MinROI:       DefaultMinROI,
		MinYield:     DefaultMinYield,
		MinWinRate:   DefaultMinWinRate,
		MaxDrawdown:  DefaultMaxDrawdown,
		MinDSFR:      DefaultMinDSFRScore,
		MaxPerLeague: DefaultMaxPerLeague,

		TrainFraction:   DefaultTrainFraction,
		HoldoutMinGames: DefaultHoldoutMinGames,
		FDRq:            DefaultFDRq,
		HoldoutAlpha:    DefaultHoldoutAlpha,
		FDRMethod:       formulas.FDRBenjaminiYekutieli,
	}
}

// withDefaults preenche campos zerados com o padrão do doc 08, para que um
// chamador possa sobrescrever só o que interessa.
func (c Criteria) withDefaults() Criteria {
	d := DefaultCriteria()
	if c.MinGames <= 0 {
		c.MinGames = d.MinGames
	}
	if c.MinROI == 0 {
		c.MinROI = d.MinROI
	}
	if c.MinYield == 0 {
		c.MinYield = d.MinYield
	}
	if c.MinWinRate == 0 {
		c.MinWinRate = d.MinWinRate
	}
	if c.MaxDrawdown <= 0 {
		c.MaxDrawdown = d.MaxDrawdown
	}
	if c.MinDSFR == 0 {
		c.MinDSFR = d.MinDSFR
	}
	if c.MaxPerLeague <= 0 {
		c.MaxPerLeague = d.MaxPerLeague
	}
	// A amostra mínima nunca pode ser afrouxada abaixo da trava anti-overfitting.
	if c.MinGames < absoluteMinimumGames {
		c.MinGames = absoluteMinimumGames
	}

	// AUD-003. Todos caem no padrão quando fora de faixa. Note que não existe
	// valor de configuração capaz de DESLIGAR a validação fora da amostra ou a
	// correção de múltiplos testes: uma TrainFraction de 1.0 (minerar tudo) ou
	// um q de 1.0 (aceitar qualquer p-valor) voltam ao padrão em vez de valer.
	// Essas duas travas são o conteúdo da correção; torná-las desligáveis por
	// configuração seria devolver o defeito pela porta dos fundos.
	if c.TrainFraction <= 0 || c.TrainFraction >= 1 {
		c.TrainFraction = DefaultTrainFraction
	}
	if c.HoldoutMinGames <= 0 {
		c.HoldoutMinGames = DefaultHoldoutMinGames
	}
	if c.FDRq <= 0 || c.FDRq >= 1 {
		c.FDRq = DefaultFDRq
	}
	if c.HoldoutAlpha <= 0 || c.HoldoutAlpha >= 1 {
		c.HoldoutAlpha = DefaultHoldoutAlpha
	}
	if c.FDRMethod != formulas.FDRBenjaminiHochberg && c.FDRMethod != formulas.FDRBenjaminiYekutieli {
		c.FDRMethod = formulas.FDRBenjaminiYekutieli
	}
	return c
}

// drawdownPct converte o drawdown máximo (expresso em unidades de stake pelo
// motor de backtest) em percentual do capital movimentado — é assim que o doc 08
// enuncia o limite ("Drawdown <= 20%").
func drawdownPct(r *usecase.BacktestResult) float64 {
	if r.TotalStaked <= 0 {
		return 0
	}
	return 100 * r.MaxDrawdown / r.TotalStaked
}

// rejection descreve por que uma combinação foi descartada. Serve para
// observabilidade do ciclo (quais critérios mais barram) e nunca é publicado.
type rejection string

const (
	rejectSample   rejection = "amostra_insuficiente"
	rejectWinRate  rejection = "win_rate_baixo"
	rejectROI      rejection = "roi_baixo"
	rejectYield    rejection = "yield_baixo"
	rejectDrawdown rejection = "drawdown_alto"
	rejectScore    rejection = "score_baixo"

	// AUD-002: antes chamava-se "ev_nao_positivo" e checava r.Yield <= 0. Não
	// era um teste de EV — o EV não é calculado em lugar nenhum do sistema
	// (ver strategyengine.backtestRow). É um teste de lucro não positivo, e é
	// isso que o nome passa a dizer. O comportamento é idêntico.
	rejectNonPositiveProfit rejection = "lucro_nao_positivo"

	// AUD-003. Motivos ligados à significância estatística e à validação fora
	// da amostra.
	rejectNotTestable         rejection = "sem_pvalor_calculavel"
	rejectMultipleTesting     rejection = "nao_sobreviveu_correcao_fdr"
	rejectHoldoutSample       rejection = "validacao_amostra_insuficiente"
	rejectHoldoutProfit       rejection = "validacao_sem_lucro"
	rejectHoldoutSignificance rejection = "validacao_nao_significativa"

	// A liga inteira não pôde ser dividida em descoberta/validação (histórico
	// curto demais, ou todas as partidas no mesmo dia). Nesse caso NADA é
	// publicado: sem janela de validação não há como saber se o padrão
	// sobrevive fora da amostra, e publicar sem saber é o defeito original.
	rejectNotSplittable rejection = "liga_sem_janela_de_validacao"
)

// validate aplica os critérios do doc 08 na ordem do documento e devolve o
// PRIMEIRO motivo de rejeição, ou "" se a combinação foi aprovada.
//
// ATENÇÃO ao ler esta lista como "múltiplos indicadores" (regra do doc 08:
// "nunca considerar apenas Win Rate"). Os gates de ROI, de Yield e o de lucro
// abaixo incidem TODOS sobre a mesma quantidade — lucro/volume apostado —
// porque sob stake fixa investimento e volume são iguais (AUD-002). Com os
// limiares padrão (ROI >= 10%, Yield >= 5%), só o de ROI chega a barrar
// alguma coisa; os outros dois nunca são atingidos.
//
// Os gates permanecem porque continuam corretos e porque MinYield é
// configurável — removê-los afrouxaria a validação em configurações não
// padrão. O que muda aqui é a honestidade do rótulo. Enquanto Yield e EV não
// forem quantidades genuinamente distintas, os indicadores realmente
// independentes desta função são três: amostra, win rate e drawdown, mais o
// retorno.
func (c Criteria) validate(r *usecase.BacktestResult) rejection {
	if r.MatchCount < c.MinGames {
		return rejectSample
	}
	if r.HitRate < c.MinWinRate {
		return rejectWinRate
	}
	if r.ROI < c.MinROI {
		return rejectROI
	}
	if r.Yield < c.MinYield {
		return rejectYield
	}
	if r.Yield <= 0 {
		return rejectNonPositiveProfit
	}
	if drawdownPct(r) > c.MaxDrawdown {
		return rejectDrawdown
	}
	return ""
}

// Classification é a faixa de qualidade do doc 08 ("Classificação").
type Classification string

const (
	ClassElite    Classification = "Elite"     // 91-100
	ClassExcelent Classification = "Excelente" // 81-90
	ClassVeryGood Classification = "Muito Boa" // 71-80
	ClassGood     Classification = "Boa"       // 61-70
	ClassRegular  Classification = "Regular"   // 40-60
	ClassDiscard  Classification = "Descartar" // 0-39
)

// Classify traduz o DSFR Score na faixa de qualidade do doc 08.
func Classify(dsfrScore float64) Classification {
	switch {
	case dsfrScore >= 91:
		return ClassElite
	case dsfrScore >= 81:
		return ClassExcelent
	case dsfrScore >= 71:
		return ClassVeryGood
	case dsfrScore >= 61:
		return ClassGood
	case dsfrScore >= 40:
		return ClassRegular
	default:
		return ClassDiscard
	}
}

// String satisfaz fmt.Stringer para uso direto em descrições.
func (c Classification) String() string { return string(c) }

var _ fmt.Stringer = ClassElite
