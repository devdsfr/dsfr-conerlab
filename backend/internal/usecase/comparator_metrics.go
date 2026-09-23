package usecase

import "github.com/devdsfr/cornerlab/internal/domain"

// Vocabulário do Comparador (REV-P2, fatia 2).
//
// Os três eixos — LOCAL, MÉTRICA e PERSPECTIVA — são independentes e nenhum
// deles pode ser confundido com outro:
//
//	LOCAL       recorta QUAIS partidas entram (todas / mandante / visitante)
//	MÉTRICA     escolhe O QUE é contado (escanteios, gols, chutes...)
//	PERSPECTIVA escolhe DE QUEM é o número (produzido / concedido / total)
//
// Exemplo do enunciado: equipe faz 6 escanteios, adversário faz 4.
// Produzido = 6, Concedido = 4, Total da partida = 10 — e isso vale igualmente
// com a equipe jogando em casa ou fora, porque TeamMatchView já reorienta os
// campos For/Against pela perspectiva da equipe consultada.

type Venue string

const (
	VenueGeral Venue = "geral"
	VenueCasa  Venue = "casa"
	VenueFora  Venue = "fora"
)

func ParseVenue(s string) Venue {
	switch Venue(s) {
	case VenueCasa:
		return VenueCasa
	case VenueFora:
		return VenueFora
	default:
		return VenueGeral
	}
}

type Metric string

const (
	MetricCorners       Metric = "corners"
	MetricGoals         Metric = "goals"
	MetricShots         Metric = "shots"
	MetricShotsOnTarget Metric = "shots_on_target"
	MetricOffsides      Metric = "offsides"
)

func ParseMetric(s string) Metric {
	switch Metric(s) {
	case MetricGoals:
		return MetricGoals
	case MetricShots:
		return MetricShots
	case MetricShotsOnTarget:
		return MetricShotsOnTarget
	case MetricOffsides:
		return MetricOffsides
	default:
		return MetricCorners
	}
}

type Perspective string

const (
	PerspectiveProduzido Perspective = "produzido"
	PerspectiveConcedido Perspective = "concedido"
	PerspectiveTotal     Perspective = "total"
)

func ParsePerspective(s string) Perspective {
	switch Perspective(s) {
	case PerspectiveProduzido:
		return PerspectiveProduzido
	case PerspectiveConcedido:
		return PerspectiveConcedido
	default:
		return PerspectiveTotal
	}
}

// metricValues devolve (produzido, concedido) daquela partida para a métrica
// pedida. Ponteiros: nil significa que o provedor NÃO publicou o dado para
// aquela partida — que é diferente de ter publicado zero.
//
// Escanteios e gols são colunas NOT NULL em `matches`; chutes, chutes no alvo e
// impedimentos são nullable (migrations 004 e 010).
func metricValues(v domain.TeamMatchView, m Metric) (produzido, concedido *int) {
	switch m {
	case MetricGoals:
		f, a := v.GoalsFor, v.GoalsAgainst
		return &f, &a
	case MetricShots:
		return v.ShotsFor, v.ShotsAgainst
	case MetricShotsOnTarget:
		return v.ShotsOnTargetFor, v.ShotsOnTargetAgainst
	case MetricOffsides:
		return v.OffsidesFor, v.OffsidesAgainst
	default:
		f, a := v.CornersFor, v.CornersAgainst
		return &f, &a
	}
}

// metricValue aplica a perspectiva sobre o par produzido/concedido.
//
// O total exige AS DUAS pontas: se o provedor publicou só uma, não existe total
// — devolver a metade disponível seria inventar a soma.
func metricValue(v domain.TeamMatchView, m Metric, p Perspective) *int {
	produzido, concedido := metricValues(v, m)
	switch p {
	case PerspectiveProduzido:
		return produzido
	case PerspectiveConcedido:
		return concedido
	default:
		if produzido == nil || concedido == nil {
			return nil
		}
		soma := *produzido + *concedido
		return &soma
	}
}

// matchMetricValues devolve (mandante, visitante) da métrica numa partida bruta
// — usado pelo H2H, onde a orientação é pelo mando e não pela equipe consultada.
func matchMetricValues(m domain.Match, metric Metric) (home, away *int) {
	switch metric {
	case MetricGoals:
		h, a := m.HomeGoals, m.AwayGoals
		return &h, &a
	case MetricShots:
		return m.HomeShots, m.AwayShots
	case MetricShotsOnTarget:
		return m.HomeShotsOnTarget, m.AwayShotsOnTarget
	case MetricOffsides:
		return m.HomeOffsides, m.AwayOffsides
	default:
		h, a := m.HomeCorners, m.AwayCorners
		return &h, &a
	}
}

// FrequencyBand é uma faixa observada, sempre com numerador E denominador.
//
// O denominador é a amostra COM A MÉTRICA PRESENTE, não a amostra de partidas:
// numa janela de 20 jogos onde só 9 têm impedimentos, "≥2" precisa ser sobre 9.
type FrequencyBand struct {
	Threshold  int     `json:"threshold"`
	Hits       int     `json:"hits"`
	Sample     int     `json:"sample"`
	Percentage float64 `json:"percentage"`
}

// frequencyThresholds devolve as faixas daquela métrica, e um segundo retorno
// dizendo se existe definição de produto para o par métrica+perspectiva.
//
// AS FAIXAS SÓ EXISTEM PARA A PERSPECTIVA "TOTAL DA PARTIDA". Os limites abaixo
// vêm do Dashboard, onde foram definidos e documentados para o total dos dois
// times. Reaproveitá-los em "produzido" ou "concedido" seria errado por ordem de
// grandeza — escanteios de UMA equipe giram em torno da metade do total, então
// "acima de 8" produziria percentuais artificialmente baixos com cara de
// estatística.
//
// Não há, hoje, definição de produto para faixas por perspectiva individual.
// Em vez de inventar, devolvemos ok=false e a interface explica a ausência.
func frequencyThresholds(m Metric, p Perspective) ([]int, bool) {
	if p != PerspectiveTotal {
		return nil, false
	}
	switch m {
	case MetricGoals:
		return GoalFrequencyThresholds, true
	case MetricShots:
		return ShotFrequencyThresholds, true
	case MetricShotsOnTarget:
		return ShotOnTargetFrequencyThresholds, true
	case MetricOffsides:
		return OffsideFrequencyThresholds, true
	default:
		return DefaultFrequencyThresholds, true
	}
}

// buildFrequencies conta, para cada limite, quantos valores são ESTRITAMENTE
// maiores — mesma convenção do Dashboard (limite N equivale à linha "N.5" nas
// métricas de meio-ponto).
func buildFrequencies(values []int, thresholds []int) []FrequencyBand {
	out := make([]FrequencyBand, 0, len(thresholds))
	for _, t := range thresholds {
		hits := 0
		for _, v := range values {
			if v > t {
				hits++
			}
		}
		pct := 0.0
		if len(values) > 0 {
			pct = round2(100 * float64(hits) / float64(len(values)))
		}
		out = append(out, FrequencyBand{Threshold: t, Hits: hits, Sample: len(values), Percentage: pct})
	}
	return out
}
