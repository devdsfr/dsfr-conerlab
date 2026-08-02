package discovery

import (
	"fmt"

	"github.com/devdsfr/cornerlab/internal/usecase"
)

// Critérios mínimos de validação do doc 08 ("Critérios mínimos"). Uma combinação
// só vira estratégia publicada se passar em TODOS eles — a regra explícita do
// documento é "nunca considerar apenas Win Rate; sempre utilizar múltiplos
// indicadores".
const (
	DefaultMinGames      = 100  // jogos
	DefaultMinROI        = 10.0 // %
	DefaultMinYield      = 5.0  // %
	DefaultMinWinRate    = 75.0 // %
	DefaultMaxDrawdown   = 20.0 // % do capital movimentado
	DefaultMinDSFRScore  = 40.0 // abaixo disso o doc manda descartar
	DefaultMaxPerLeague  = 40   // teto de descobertas publicadas por liga
	absoluteMinimumGames = 50   // trava anti-overfitting: nunca publicar abaixo disso
)

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
	rejectEV       rejection = "ev_nao_positivo"
	rejectDrawdown rejection = "drawdown_alto"
	rejectScore    rejection = "score_baixo"
)

// validate aplica os critérios do doc 08 na ordem do documento e devolve o
// PRIMEIRO motivo de rejeição, ou "" se a combinação foi aprovada.
//
// EV é avaliado como o yield por aposta (mesma convenção do Strategy Engine da
// F4: sem odds reais para todas as métricas, EV por entrada ≈ yield).
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
		return rejectEV
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
