package usecase

import (
	"context"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P3 — correções finais das pendências comprovadas na Fase D (25/09/2026).
//
// Usa os dublês de filter_odds_source_test.go e o helper `partida` de
// filter_revp3_test.go.

// =============================================================================
// §1 — ESTADO VAZIO: ausência não pode virar zero observado
//
// Em produção, La Liga 2025 (fora da janela de 90 dias) devolvia match_count 0
// e a tela renderizava "Taxa de acerto 0%" e "Média de escanteios 0" — zeros de
// ausência apresentados como observação. Além disso o resumo declarava
// odds_source "fixed" só porque o usuário tinha digitado uma odd, exibindo
// "Cenário com odd fixa" ao lado de "0 partidas".
//
// O backend agora precisa entregar os três estados distinguíveis.
// =============================================================================

func TestEstadoA_NenhumaPartida_NaoDeclaraProcedenciaDeOdd(t *testing.T) {
	u := newFilterUsecaseWith(nil)
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.OddsSource != oddsSourceNone {
		t.Errorf("OddsSource = %q com ZERO ocorrências, esperado %q. "+
			"Declarar \"fixed\" aqui é intenção do usuário apresentada como observação",
			res.OddsSource, oddsSourceNone)
	}
	if res.FinancialsAvailable {
		t.Error("FinancialsAvailable = true sem nenhuma partida")
	}
	// O que distingue o estado A: não havia partida NO RECORTE.
	if res.Accounting.MatchesInWindow != 0 {
		t.Errorf("MatchesInWindow = %d, esperado 0", res.Accounting.MatchesInWindow)
	}
	if res.Accounting.ObservationsInWindow != 0 {
		t.Errorf("ObservationsInWindow = %d, esperado 0", res.Accounting.ObservationsInWindow)
	}
}

func TestEstadoB_PartidasExistemMasMetricaIndisponivel(t *testing.T) {
	// Duas partidas no recorte; nenhuma tem impedimentos publicados.
	a, b := partida(1, 6, 5), partida(2, 7, 4)
	a.HomeOffsides, a.AwayOffsides = nil, nil
	b.HomeOffsides, b.AwayOffsides = nil, nil

	u := newFilterUsecaseWith([]domain.Match{a, b})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "offsides", OffsidesThreshold: 2, Stake: 100,
			FixedOdd: 1.50, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 0 {
		t.Errorf("MatchCount = %d, esperado 0 — partida sem o dado não entra como zero", res.MatchCount)
	}
	// O que distingue o estado B do estado A: HAVIA partidas no recorte, e elas
	// saíram por falta da métrica. Sem isto a tela não consegue diferenciar
	// "não achei jogo nenhum" de "achei jogos mas o provedor não publicou o dado".
	if res.Accounting.MatchesInWindow != 2 {
		t.Errorf("MatchesInWindow = %d, esperado 2", res.Accounting.MatchesInWindow)
	}
	if res.Accounting.ExcludedNoMetric != 2 {
		t.Errorf("ExcludedNoMetric = %d, esperado 2", res.Accounting.ExcludedNoMetric)
	}
	if res.Accounting.EligibleEntries != 0 {
		t.Errorf("EligibleEntries = %d, esperado 0", res.Accounting.EligibleEntries)
	}
}

func TestEstadoC_ZeroRealNaoViraIndisponivel(t *testing.T) {
	// Zero impedimentos REGISTRADOS: observação válida. A partida entra e conta
	// como erro da linha "acima de 2" — não pode ser tratada como ausência.
	zero := 0
	m := partida(1, 6, 5)
	m.HomeOffsides, m.AwayOffsides = &zero, &zero

	u := newFilterUsecaseWith([]domain.Match{m})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "offsides", OffsidesThreshold: 2, Stake: 100,
			FixedOdd: 1.50, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, esperado 1 — zero observado é observação", res.MatchCount)
	}
	if res.Accounting.ExcludedNoMetric != 0 {
		t.Errorf("ExcludedNoMetric = %d, esperado 0 — zero real não é ausência",
			res.Accounting.ExcludedNoMetric)
	}
	if res.Hits != 0 || res.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, esperado 0/1", res.Hits, res.Misses)
	}
	if len(res.Entries) != 1 || res.Entries[0].TotalOffsides != 0 {
		t.Error("o zero observado precisa chegar à tabela como 0")
	}
}

// =============================================================================
// §2 — MetricScope: a linha é da PARTIDA ou da EQUIPE?
//
// Em produção a tabela mostrava "Mando: Casa" em 100/100 linhas de escanteios,
// porque a correção 1 elegeu a perspectiva do mandante como representante único
// da partida. O número é da partida inteira; o rótulo dizia que era do mandante.
// =============================================================================

func TestMetricScope_MatchLevel(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	for _, metrica := range []string{"corners", "goals", "offsides", "shots", "shots_on_target"} {
		crit := FilterCriteria{Metric: metrica, CornersThreshold: 8, GoalsThreshold: 1,
			OffsidesThreshold: 0, ShotsThreshold: 0, ShotsOnTargetThreshold: 0,
			Stake: 100, FixedOdd: 1.50, AllowMissingOdds: true}
		res, err := u.RunBacktest(context.Background(), 1, nil, crit, 0)
		if err != nil {
			t.Fatalf("%s: %v", metrica, err)
		}
		if res.MetricScope != metricScopeMatch {
			t.Errorf("%s: MetricScope = %q, esperado %q — o total é da partida inteira, "+
				"a linha não tem perspectiva de mandante", metrica, res.MetricScope, metricScopeMatch)
		}
	}
}

func TestMetricScope_TeamLevel(t *testing.T) {
	m := partida(3, 5, 5)
	m.HomeGoals, m.AwayGoals = 2, 0
	u := newFilterUsecaseWith([]domain.Match{m})
	for _, metrica := range []string{MetricWin, MetricDraw, MetricWinOrDraw} {
		res, err := u.RunBacktest(context.Background(), 1, nil,
			FilterCriteria{Metric: metrica, Stake: 100, FixedOdd: 1.80, AllowMissingOdds: true}, 0)
		if err != nil {
			t.Fatalf("%s: %v", metrica, err)
		}
		if res.MetricScope != metricScopeTeam {
			t.Errorf("%s: MetricScope = %q, esperado %q — mandante e visitante têm "+
				"desfechos diferentes, a perspectiva é real", metrica, res.MetricScope, metricScopeTeam)
		}
		if res.MatchCount != 2 {
			t.Errorf("%s: MatchCount = %d, esperado 2", metrica, res.MatchCount)
		}
	}
}

func TestMetricScope_MatchLevelComMandoPedido_ContinuaMatch(t *testing.T) {
	// Pedir "só em casa" filtra a amostra, mas não transforma o total de
	// escanteios da partida numa estatística do mandante.
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, HomeAway: "home",
			Stake: 100, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MetricScope != metricScopeMatch {
		t.Errorf("MetricScope = %q, esperado %q", res.MetricScope, metricScopeMatch)
	}
}

// =============================================================================
// §3 — max_odds: filtro de elegibilidade sobre odd DE MERCADO
//
// A investigação (25/09/2026) mostrou que o filtro nunca esteve quebrado nem é
// legado: está implementado em filter_usecase.go e descarta a partida cuja odd
// registrada passe do teto. O que faltava era dizer quantas partidas tinham odd
// para comparar — sem isso o usuário digita "odds máximas 5,00", recebe 100
// partidas e não descobre que o controle não agiu sobre nenhuma.
// =============================================================================

func TestMaxOdds_TemEfeitoQuandoHaOddDeMercado(t *testing.T) {
	alta := partida(1, 6, 5)
	alta.CornerOdds = map[string]float64{"8.5": 3.00}
	alta.OddsSource = "real"
	baixa := partida(2, 7, 4)
	baixa.CornerOdds = map[string]float64{"8.5": 1.40}
	baixa.OddsSource = "real"

	u := newFilterUsecaseWith([]domain.Match{alta, baixa})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, MaxOdds: 2.00}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 1 {
		t.Errorf("MatchCount = %d, esperado 1 (a odd 3.00 passa do teto 2.00)", res.MatchCount)
	}
	if res.Accounting.ExcludedByMaxOdds != 1 {
		t.Errorf("ExcludedByMaxOdds = %d, esperado 1", res.Accounting.ExcludedByMaxOdds)
	}
	if res.Accounting.MaxOddsApplicable != 2 {
		t.Errorf("MaxOddsApplicable = %d, esperado 2 (as duas tinham odd para comparar)",
			res.Accounting.MaxOddsApplicable)
	}
}

func TestMaxOdds_SemOddNaBase_NaoFiltraEDizQueNaoFiltrou(t *testing.T) {
	// Situação REAL de produção: nenhuma partida da janela tem corner_odds.
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100,
			MaxOdds: 5.00, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2", res.MatchCount)
	}
	if res.Accounting.MaxOddsRequested != 5.00 {
		t.Errorf("MaxOddsRequested = %v, esperado 5", res.Accounting.MaxOddsRequested)
	}
	// O ponto do teste: o contrato precisa DIZER que o teto não teve onde agir.
	if res.Accounting.MaxOddsApplicable != 0 {
		t.Errorf("MaxOddsApplicable = %d, esperado 0 — nenhuma partida tinha odd de mercado",
			res.Accounting.MaxOddsApplicable)
	}
	if res.Accounting.ExcludedByMaxOdds != 0 {
		t.Errorf("ExcludedByMaxOdds = %d, esperado 0", res.Accounting.ExcludedByMaxOdds)
	}
}

// =============================================================================
// §4 — a contabilidade tem que FECHAR
// =============================================================================

func conferirIdentidade(t *testing.T, res *BacktestResult, ctx string) {
	t.Helper()
	a := res.Accounting
	soma := a.EligibleEntries + a.ExcludedNoMetric + a.ExcludedByMaxOdds +
		a.ExcludedNoOdd + a.ExcludedByVenue + a.ExcludedOther
	if soma != a.ObservationsInWindow {
		t.Errorf("%s: contabilidade não fecha — elegíveis %d + excluídas "+
			"(métrica %d, teto de odd %d, sem odd %d, mando %d, outras %d) = %d, "+
			"mas ObservationsInWindow = %d",
			ctx, a.EligibleEntries, a.ExcludedNoMetric, a.ExcludedByMaxOdds,
			a.ExcludedNoOdd, a.ExcludedByVenue, a.ExcludedOther, soma, a.ObservationsInWindow)
	}
	if a.EligibleEntries != res.MatchCount {
		t.Errorf("%s: EligibleEntries %d ≠ MatchCount %d", ctx, a.EligibleEntries, res.MatchCount)
	}
	if a.ExcludedEntries != a.ObservationsInWindow-a.EligibleEntries {
		t.Errorf("%s: ExcludedEntries %d inconsistente", ctx, a.ExcludedEntries)
	}
}

func TestContabilidade_Fecha_EmTodosOsCenarios(t *testing.T) {
	zero := 0
	semDado := partida(10, 6, 5)
	semDado.HomeOffsides, semDado.AwayOffsides = nil, nil
	comZero := partida(11, 7, 4)
	comZero.HomeOffsides, comZero.AwayOffsides = &zero, &zero
	comOddAlta := partida(12, 6, 6)
	comOddAlta.CornerOdds = map[string]float64{"8.5": 4.00}
	comOddAlta.OddsSource = "real"

	base := []domain.Match{partida(1, 6, 5), partida(2, 7, 4), semDado, comZero, comOddAlta}

	casos := []struct {
		nome string
		crit FilterCriteria
	}{
		{"escanteios match-level com odd fixa",
			FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, FixedOdd: 1.50}},
		{"escanteios com teto de odd",
			FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, MaxOdds: 2.00, AllowMissingOdds: true}},
		{"escanteios só em casa",
			FilterCriteria{Metric: "corners", CornersThreshold: 8, HomeAway: "home", Stake: 100, FixedOdd: 1.50}},
		{"impedimentos com dado faltando",
			FilterCriteria{Metric: "offsides", OffsidesThreshold: 2, Stake: 100, FixedOdd: 1.50, AllowMissingOdds: true}},
		{"vitória team-level",
			FilterCriteria{Metric: MetricWin, Stake: 100, FixedOdd: 1.80, AllowMissingOdds: true}},
		{"vitória team-level só fora",
			FilterCriteria{Metric: MetricWin, HomeAway: "away", Stake: 100, FixedOdd: 1.80, AllowMissingOdds: true}},
		{"sem AllowMissingOdds (Engine/Discovery)",
			FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100}},
	}
	for _, c := range casos {
		res, err := newFilterUsecaseWith(base).RunBacktest(context.Background(), 1, nil, c.crit, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.nome, err)
		}
		conferirIdentidade(t, res, c.nome)
	}
}

func TestContabilidade_TeamLevelContaObservacoesNaoPartidas(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: MetricWin, Stake: 100, FixedOdd: 1.80, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.Accounting.MatchesInWindow != 2 {
		t.Errorf("MatchesInWindow = %d, esperado 2 (partidas)", res.Accounting.MatchesInWindow)
	}
	if res.Accounting.ObservationsInWindow != 4 {
		t.Errorf("ObservationsInWindow = %d, esperado 4 (duas perspectivas por partida)",
			res.Accounting.ObservationsInWindow)
	}
	conferirIdentidade(t, res, "team-level")
}

func TestContabilidade_MandoExcluiEContabiliza(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: MetricWin, HomeAway: "home", Stake: 100,
			FixedOdd: 1.80, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.Accounting.ExcludedByVenue != 2 {
		t.Errorf("ExcludedByVenue = %d, esperado 2 (as perspectivas de visitante)",
			res.Accounting.ExcludedByVenue)
	}
	conferirIdentidade(t, res, "mando")
}

// =============================================================================
// §5 — escanteios NULL vs 0
//
// A investigação provou que o banco NÃO guarda NULL: `home_corners INT NOT NULL
// DEFAULT 0` (migrations/001_init.sql) e a ingestão converte a ausência em zero
// antes de gravar (internal/usecase/sync_usecase.go, "homeCorners := 0").
//
// Portanto, HOJE, o motor não tem como distinguir — e não existe correção
// possível no Simulador. Os testes abaixo fixam o que É verdade agora, sem
// fingir que a distinção existe:
//   - em métrica NULLABLE (impedimentos) a distinção funciona;
//   - em escanteios o zero é indistinguível, e isso está documentado.
// =============================================================================

func TestNullVsZero_MetricaNullable_DistinguiCorretamente(t *testing.T) {
	zero := 0
	ausente := partida(1, 6, 5)
	ausente.HomeOffsides, ausente.AwayOffsides = nil, nil
	observadoZero := partida(2, 7, 4)
	observadoZero.HomeOffsides, observadoZero.AwayOffsides = &zero, &zero

	u := newFilterUsecaseWith([]domain.Match{ausente, observadoZero})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "offsides", OffsidesThreshold: 0, Stake: 100,
			FixedOdd: 1.50, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	// NULL fica indisponível (fora do backtest); 0 permanece zero real (dentro,
	// como erro da linha "acima de 0").
	if res.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, esperado 1 — só a partida com dado observado entra", res.MatchCount)
	}
	if res.Accounting.ExcludedNoMetric != 1 {
		t.Errorf("ExcludedNoMetric = %d, esperado 1 (a partida com NULL)", res.Accounting.ExcludedNoMetric)
	}
	if res.Entries[0].MatchID != 2 {
		t.Errorf("entrou a partida %d, esperado a 2 (a do zero observado)", res.Entries[0].MatchID)
	}
	if res.Entries[0].TotalOffsides != 0 {
		t.Errorf("TotalOffsides = %d, esperado 0 — zero observado continua zero",
			res.Entries[0].TotalOffsides)
	}
	if res.Hits != 0 || res.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, esperado 0/1", res.Hits, res.Misses)
	}
}

// TestNullVsZero_Escanteios_LimitacaoConhecida documenta uma LIMITAÇÃO REAL, não
// um comportamento desejado.
//
// domain.Match.HomeCorners é `int`, não `*int`, porque a coluna é NOT NULL. Uma
// partida cujo provedor não publicou escanteios chega aqui como 0 e é
// indistinguível de um 0-0 de escanteios observado. Em produção há pelo menos um
// caso: match_id 16451 (2026-07-21, Atlético-MG × Bahia) com 0 escanteios totais.
//
// Corrigir exige migration tornando as colunas nullable, trocar o tipo no domínio
// e propagar por Comparador (REV-P2) e Dashboard (REV-P1) — fora do escopo
// autorizado agora. E a informação histórica já está PERDIDA: os zeros gravados
// não guardam se eram ausência.
//
// Se alguém tornar a coluna nullable no futuro, este teste falha e obriga a
// revisitar a decisão em vez de deixá-la silenciosamente desatualizada.
func TestNullVsZero_Escanteios_LimitacaoConhecida(t *testing.T) {
	semEscanteios := partida(1, 0, 0)
	u := newFilterUsecaseWith([]domain.Match{semEscanteios})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		// Validate() exige CornersThreshold > 0, então a linha é "acima de 1":
		// a partida com 0 escanteios ENTRA e conta como erro. O ponto do teste é
		// que ela entra — e não deveria, se fosse ausência de dado.
		FilterCriteria{Metric: "corners", CornersThreshold: 1, Stake: 100,
			FixedOdd: 1.50, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, esperado 1", res.MatchCount)
	}
	if res.Accounting.ExcludedNoMetric != 0 {
		t.Errorf("ExcludedNoMetric = %d: se virou 1, a nulabilidade de escanteios "+
			"foi implementada — reveja esta limitação e o registro no "+
			"CORRECOES_AUDITORIA.md em vez de apenas ajustar o teste",
			res.Accounting.ExcludedNoMetric)
	}
	if res.Entries[0].TotalCorners != 0 {
		t.Errorf("TotalCorners = %d, esperado 0", res.Entries[0].TotalCorners)
	}
}
