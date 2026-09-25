package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// Testes dos mercados de resultado (vitória, empate, dupla chance).
//
// Estes mercados quebram a premissa de todo o resto do motor: não são limiar
// sobre um total, são desfecho da partida. O risco de regressão mora aí — o
// caminho antigo (`hit := total > threshold`) continua valendo para as demais
// métricas, e um erro na separação faria vitória ser avaliada como "0 > 0".

// resultMatch monta uma partida com placar definido, na perspectiva mandante.
func resultMatch(id int64, homeGoals, awayGoals int) domain.Match {
	return domain.Match{
		ID:         id,
		LeagueID:   1,
		SeasonID:   1,
		MatchDate:  time.Date(2025, 3, int(id), 0, 0, 0, 0, time.UTC),
		HomeTeamID: 10,
		AwayTeamID: 20,
		HomeGoals:  homeGoals,
		AwayGoals:  awayGoals,
	}
}

func resultCriteria(metric string) FilterCriteria {
	// FixedOdd porque não existe odd de resultado coletada: é o caminho do
	// Simulador. Sem ela, toda partida seria descartada (AUD-012).
	return FilterCriteria{Metric: metric, FixedOdd: 2.00, Stake: 10}
}

// O acerto tem que sair do PLACAR, não de um limiar.
func TestMercadosDeResultadoUsamOPlacar(t *testing.T) {
	// 3 partidas: mandante vence, empate, visitante vence.
	matches := []domain.Match{
		resultMatch(1, 2, 0),
		resultMatch(2, 1, 1),
		resultMatch(3, 0, 3),
	}
	u := newFilterUsecaseWith(matches)

	casos := []struct {
		metric    string
		wantHits  int
		wantTotal int
		porque    string
	}{
		// Cada partida vira 2 candidatos (mandante e visitante) = 6 no total.
		// Vitória: mandante da 1 + visitante da 3 = 2 acertos.
		{MetricWin, 2, 6, "só o vencedor de cada jogo decidido acerta"},
		// Empate: os DOIS candidatos da partida 2 acertam.
		{MetricDraw, 2, 6, "empate é desfecho da partida: vale para os dois lados"},
		// Não perde: vencedores (2) + os dois empatados (2) = 4.
		{MetricWinOrDraw, 4, 6, "vencedores mais os dois lados do empate"},
	}

	for _, c := range casos {
		res, err := u.RunBacktest(context.Background(), 1, nil, resultCriteria(c.metric), 0)
		if err != nil {
			t.Fatalf("%s: %v", c.metric, err)
		}
		if res.MatchCount != c.wantTotal {
			t.Errorf("%s: MatchCount = %d, esperado %d", c.metric, res.MatchCount, c.wantTotal)
		}
		if res.Hits != c.wantHits {
			t.Errorf("%s: Hits = %d, esperado %d (%s)", c.metric, res.Hits, c.wantHits, c.porque)
		}
	}
}

// O caminho antigo não pode ser afetado: escanteios continua sendo limiar.
func TestEscanteiosContinuaUsandoLimiar(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceReal),
	})
	res, err := u.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatalf("backtest de escanteios quebrou: %v", err)
	}
	// matchWithOdds tem 11 escanteios e a linha é 8 -> todos acertam.
	//
	// REV-P3 (AUD-021): esperado passou de 4/4 para 2/2. São 2 PARTIDAS, e
	// escanteios é regra MATCH-LEVEL — o total é o mesmo olhando dos dois lados,
	// então cada partida vale UMA observação. Antes o engine contava as duas
	// perspectivas e dizia 4. O teste codificava o defeito; a expectativa mudou
	// porque a regra mudou, não para o teste passar.
	if res.MatchCount != 2 || res.Hits != 2 {
		t.Errorf("escanteios: %d jogos / %d acertos, esperado 2/2", res.MatchCount, res.Hits)
	}
}

// AUD-001/AUD-003: o Discovery só valida sobre odd de mercado, e não existe odd
// de resultado coletada. Toda combinação de resultado tem que sair vazia.
func TestDiscoveryRecusaResultadoSemOddReal(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		resultMatch(1, 2, 0),
		resultMatch(2, 1, 1),
		resultMatch(3, 0, 3),
	})

	for _, metric := range []string{MetricWin, MetricDraw, MetricWinOrDraw} {
		c := resultCriteria(metric)
		c.RequireRealOdds = true

		res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
		if err != nil {
			t.Fatalf("%s: %v", metric, err)
		}
		if res.MatchCount != 0 {
			t.Errorf("%s: %d partidas entraram sem odd real de resultado", metric, res.MatchCount)
		}
	}
}

// Com odd real registrada, o mercado passa a ser validável — e o resultado sai
// marcado como medida de mercado.
func TestResultadoComOddRealEConfiavel(t *testing.T) {
	m1 := resultMatch(1, 2, 0)
	m1.ResultOdds = map[string]float64{
		domain.ResultOutcomeHome: 1.80,
		domain.ResultOutcomeAway: 4.20,
	}
	m1.ResultOddsSource = domain.OddsSourceReal

	u := newFilterUsecaseWith([]domain.Match{m1})

	c := FilterCriteria{Metric: MetricWin, Stake: 10, RequireRealOdds: true}
	res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.MatchCount != 2 {
		t.Fatalf("MatchCount = %d, esperado 2 (mandante e visitante)", res.MatchCount)
	}
	if res.OddsSource != domain.OddsSourceReal || !res.FinancialsReliable {
		t.Errorf("odd real de resultado deveria produzir financeiro confiável: source=%q reliable=%v",
			res.OddsSource, res.FinancialsReliable)
	}
}

// A odd precisa vir do lado certo: mandante e visitante têm preços diferentes.
func TestOddDeResultadoRespeitaOMando(t *testing.T) {
	casos := []struct {
		metric string
		isHome bool
		want   string
	}{
		{MetricWin, true, domain.ResultOutcomeHome},
		{MetricWin, false, domain.ResultOutcomeAway},
		{MetricWinOrDraw, true, domain.ResultOutcomeHomeOrDraw},
		{MetricWinOrDraw, false, domain.ResultOutcomeAwayOrDraw},
		// Empate é o mesmo desfecho dos dois lados.
		{MetricDraw, true, domain.ResultOutcomeDraw},
		{MetricDraw, false, domain.ResultOutcomeDraw},
	}
	for _, c := range casos {
		if got := resultOutcome(c.metric, c.isHome); got != c.want {
			t.Errorf("resultOutcome(%q, isHome=%v) = %q, esperado %q", c.metric, c.isHome, got, c.want)
		}
	}
}

// Odd fixa informada pelo usuário nunca vira medida de mercado.
func TestResultadoComOddFixaNaoEConfiavel(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{resultMatch(1, 2, 0)})

	res, err := u.RunBacktest(context.Background(), 1, nil, resultCriteria(MetricWin), 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.OddsSource != "fixed" {
		t.Errorf("OddsSource = %q, esperado \"fixed\"", res.OddsSource)
	}
	if res.FinancialsReliable {
		t.Error("odd informada na tela não é odd de mercado")
	}
}

func TestValidateAceitaMercadosDeResultado(t *testing.T) {
	for _, metric := range []string{MetricWin, MetricDraw, MetricWinOrDraw} {
		// Sem limiar nenhum: mercados de resultado não precisam de threshold.
		if err := (FilterCriteria{Metric: metric}).Validate(); err != nil {
			t.Errorf("%s deveria ser válido sem limiar: %v", metric, err)
		}
	}
	if err := (FilterCriteria{Metric: "inexistente"}).Validate(); err == nil {
		t.Error("métrica desconhecida deveria ser recusada")
	}
}

// O placar precisa chegar na linha da tabela — sem ele o usuário vê "acertou"
// sem saber por quê.
func TestEntradaDeResultadoCarregaOPlacar(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{resultMatch(1, 3, 1)})

	res, err := u.RunBacktest(context.Background(), 1, nil, resultCriteria(MetricWin), 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("esperava 2 entradas, veio %d", len(res.Entries))
	}
	var achouMandante bool
	for _, e := range res.Entries {
		if e.IsHome {
			achouMandante = true
			if e.GoalsFor != 3 || e.GoalsAgainst != 1 {
				t.Errorf("placar do mandante errado: %d x %d", e.GoalsFor, e.GoalsAgainst)
			}
			if !e.Hit {
				t.Error("mandante venceu 3x1 e não foi marcado como acerto")
			}
		}
	}
	if !achouMandante {
		t.Error("nenhuma entrada na perspectiva do mandante")
	}
}
