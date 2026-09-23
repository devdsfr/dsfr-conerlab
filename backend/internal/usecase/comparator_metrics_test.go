package usecase

import (
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P2 fatia 2 — LOCAL, MÉTRICA e PERSPECTIVA.
//
// Datasets minúsculos com resultado calculável à mão, como pede o enunciado: o
// objetivo é pegar inversão de sinal e troca de eixo sem depender da base real.

func ptr(v int) *int { return &v }

// vista monta uma TeamMatchView com escanteios e gols; os campos nullable ficam
// nil de propósito (é o estado real de boa parte da base para impedimentos).
func vista(isHome bool, cornersFor, cornersAgainst, goalsFor, goalsAgainst int) domain.TeamMatchView {
	return domain.TeamMatchView{
		IsHome:         isHome,
		CornersFor:     cornersFor,
		CornersAgainst: cornersAgainst,
		GoalsFor:       goalsFor,
		GoalsAgainst:   goalsAgainst,
	}
}

// --- Perspectiva -----------------------------------------------------------
//
// Exemplo literal do enunciado: equipe faz 6, adversário faz 4.
// Produzido = 6 · Concedido = 4 · Total = 10.

func TestPerspectivaComEquipeMandante(t *testing.T) {
	v := vista(true, 6, 4, 0, 0)

	if got := *metricValue(v, MetricCorners, PerspectiveProduzido); got != 6 {
		t.Errorf("produzido (mandante) = %d, esperado 6", got)
	}
	if got := *metricValue(v, MetricCorners, PerspectiveConcedido); got != 4 {
		t.Errorf("concedido (mandante) = %d, esperado 4", got)
	}
	if got := *metricValue(v, MetricCorners, PerspectiveTotal); got != 10 {
		t.Errorf("total (mandante) = %d, esperado 10", got)
	}
}

// O MESMO resultado com a equipe visitante. Se algum lugar reorientasse por
// mando em vez de por equipe consultada, este teste inverteria 6 e 4.
func TestPerspectivaComEquipeVisitanteNaoInverte(t *testing.T) {
	v := vista(false, 6, 4, 0, 0)

	if got := *metricValue(v, MetricCorners, PerspectiveProduzido); got != 6 {
		t.Errorf("produzido (visitante) = %d, esperado 6 — houve inversão home/away", got)
	}
	if got := *metricValue(v, MetricCorners, PerspectiveConcedido); got != 4 {
		t.Errorf("concedido (visitante) = %d, esperado 4 — houve inversão home/away", got)
	}
	if got := *metricValue(v, MetricCorners, PerspectiveTotal); got != 10 {
		t.Errorf("total (visitante) = %d, esperado 10", got)
	}
}

func TestPerspectivaEmGols(t *testing.T) {
	v := vista(true, 0, 0, 2, 1)

	if got := *metricValue(v, MetricGoals, PerspectiveProduzido); got != 2 {
		t.Errorf("gols produzidos = %d, esperado 2", got)
	}
	if got := *metricValue(v, MetricGoals, PerspectiveConcedido); got != 1 {
		t.Errorf("gols concedidos = %d, esperado 1", got)
	}
	if got := *metricValue(v, MetricGoals, PerspectiveTotal); got != 3 {
		t.Errorf("total de gols = %d, esperado 3", got)
	}
}

// --- Métrica indisponível ---------------------------------------------------

func TestMetricaAusenteNaoViraZero(t *testing.T) {
	v := vista(true, 6, 4, 0, 0) // OffsidesFor/Against ficam nil

	for _, p := range []Perspective{PerspectiveProduzido, PerspectiveConcedido, PerspectiveTotal} {
		if got := metricValue(v, MetricOffsides, p); got != nil {
			t.Errorf("impedimentos (%s) = %d, esperado nil — o provedor não publicou o dado", p, *got)
		}
	}
}

// Total exige as DUAS pontas. Com só uma publicada, devolver a metade seria
// inventar a soma.
func TestTotalComApenasUmaPontaPublicadaEhIndisponivel(t *testing.T) {
	v := vista(true, 0, 0, 0, 0)
	v.ShotsFor = ptr(11) // só o lado da equipe

	if got := *metricValue(v, MetricShots, PerspectiveProduzido); got != 11 {
		t.Errorf("chutes produzidos = %d, esperado 11", got)
	}
	if got := metricValue(v, MetricShots, PerspectiveTotal); got != nil {
		t.Errorf("total de chutes = %d, esperado nil: só uma das pontas existe", *got)
	}
}

// --- Frequências ------------------------------------------------------------

func TestFrequenciaTrazNumeradorEDenominador(t *testing.T) {
	// 4, 6, 8 — "acima de 5" pega 6 e 8.
	b := buildFrequencies([]int{4, 6, 8}, []int{5})[0]

	if b.Hits != 2 {
		t.Errorf("hits = %d, esperado 2", b.Hits)
	}
	if b.Sample != 3 {
		t.Errorf("sample = %d, esperado 3", b.Sample)
	}
	if b.Percentage < 66.66 || b.Percentage > 66.67 {
		t.Errorf("percentage = %v, esperado ~66.67", b.Percentage)
	}
}

// O limite é ESTRITO: valor igual ao limite não conta (limite N = linha N.5).
func TestFrequenciaLimiteExatoNaoConta(t *testing.T) {
	b := buildFrequencies([]int{5, 5, 6}, []int{5})[0]

	if b.Hits != 1 {
		t.Errorf("hits = %d, esperado 1 — só o 6 é estritamente maior que 5", b.Hits)
	}
}

// O denominador é a amostra COM a métrica, não a de partidas.
func TestFrequenciaSemValoresTemDenominadorZero(t *testing.T) {
	b := buildFrequencies(nil, []int{5})[0]

	if b.Sample != 0 || b.Hits != 0 || b.Percentage != 0 {
		t.Errorf("faixa sem amostra = %+v, esperado tudo zero sem inventar denominador", b)
	}
}

// Faixas só existem para o total da partida — não há definição de produto para
// produzido/concedido, e reaproveitar os limites do total seria enganoso.
func TestFaixasSoExistemParaTotalDaPartida(t *testing.T) {
	if _, ok := frequencyThresholds(MetricCorners, PerspectiveTotal); !ok {
		t.Error("faixas de escanteios (total) deveriam existir")
	}
	for _, p := range []Perspective{PerspectiveProduzido, PerspectiveConcedido} {
		if _, ok := frequencyThresholds(MetricCorners, p); ok {
			t.Errorf("faixas inventadas para perspectiva %s", p)
		}
	}
}

// Cada métrica tem as suas faixas; as de escanteios não podem vazar para gols.
func TestFaixasNaoSeMisturamEntreMetricas(t *testing.T) {
	corners, _ := frequencyThresholds(MetricCorners, PerspectiveTotal)
	goals, _ := frequencyThresholds(MetricGoals, PerspectiveTotal)

	if len(goals) == 0 {
		t.Fatal("gols ficaram sem faixas")
	}
	if goals[0] == corners[0] {
		t.Errorf("gols herdaram a faixa de escanteios (%d) — linhas de escanteio não "+
			"fazem sentido para gols", goals[0])
	}
}

// --- Parsing dos eixos ------------------------------------------------------

func TestPadroesSegurosDosEixos(t *testing.T) {
	if ParseVenue("qualquer-coisa") != VenueGeral {
		t.Error("venue desconhecido deveria cair em geral")
	}
	if ParseMetric("") != MetricCorners {
		t.Error("métrica vazia deveria cair em escanteios")
	}
	if ParsePerspective("xyz") != PerspectiveTotal {
		t.Error("perspectiva desconhecida deveria cair em total")
	}
	if ParseVenue("casa") != VenueCasa || ParseVenue("fora") != VenueFora {
		t.Error("casa/fora não foram reconhecidos")
	}
	if ParseMetric("offsides") != MetricOffsides {
		t.Error("offsides não foi reconhecido")
	}
}

// --- H2H: orientação por mando ---------------------------------------------

func TestH2HOrientaPeloMandoENaoPelaEquipeConsultada(t *testing.T) {
	m := domain.Match{HomeCorners: 7, AwayCorners: 2, HomeGoals: 1, AwayGoals: 0}

	home, away := matchMetricValues(m, MetricCorners)
	if *home != 7 || *away != 2 {
		t.Errorf("H2H escanteios = %d/%d, esperado 7/2", *home, *away)
	}

	hg, ag := matchMetricValues(m, MetricGoals)
	if *hg != 1 || *ag != 0 {
		t.Errorf("H2H gols = %d/%d, esperado 1/0", *hg, *ag)
	}
}

func TestH2HMetricaAusenteContinuaNil(t *testing.T) {
	m := domain.Match{} // sem impedimentos publicados
	home, away := matchMetricValues(m, MetricOffsides)
	if home != nil || away != nil {
		t.Error("impedimentos ausentes viraram valor no H2H")
	}
}
