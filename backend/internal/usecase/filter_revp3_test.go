package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P3 — Fase C. Testes das seis correções comprovadas na Fase A.
//
// Todos usam os dublês já definidos em filter_odds_source_test.go
// (stubMatchRepo / stubTeamRepo / stubLeagueRepo, newFilterUsecaseWith).
//
// Nenhum destes testes foi escrito para "passar": cada um fixa uma regra que a
// Fase A provou estar quebrada em produção, com os números observados lá.

// partida monta uma partida com o placar de escanteios pedido. Sem odd — a odd
// é acrescentada pelos testes que precisam dela, justamente para que a AUSÊNCIA
// de odd seja o caso padrão e não uma exceção.
func partida(id int64, mandante, visitante int) domain.Match {
	return domain.Match{
		ID:          id,
		LeagueID:    1,
		SeasonID:    1,
		MatchDate:   time.Date(2025, 3, int(id), 0, 0, 0, 0, time.UTC),
		HomeTeamID:  10,
		AwayTeamID:  20,
		HomeCorners: mandante,
		AwayCorners: visitante,
		HomeGoals:   1,
		AwayGoals:   1,
	}
}

// =============================================================================
// Correção 1 — DUPLA CONTAGEM
//
// Fase A provou em produção: 2 partidas devolviam 4 ocorrências, e os pares
// 138/69 e 200/100 apareciam no Simulador. A causa é que toda partida era
// expandida em duas linhas (mandante e visitante) mesmo quando a regra é da
// PARTIDA INTEIRA, e não de um time.
//
// A regra fixada aqui NÃO é "dedup por match_id". É a distinção entre:
//   - MATCH-LEVEL (escanteios, gols, impedimentos, chutes): o total é da
//     partida. Uma partida = UMA ocorrência.
//   - TEAM-LEVEL (vitória, empate, não perde): o desfecho é de cada time, e
//     mandante e visitante têm respostas diferentes. Uma partida = DUAS
//     ocorrências, e isso continua certo.
// =============================================================================

func TestDedup_MatchLevel_UmaOcorrenciaPorPartida(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		partida(1, 6, 5), // 11 escanteios — acerta a linha 8
		partida(2, 7, 4), // 11 escanteios — acerta a linha 8
	})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2 (uma ocorrência por partida). "+
			"4 significa que a dupla contagem voltou", res.MatchCount)
	}
	if len(res.Entries) != 2 {
		t.Errorf("len(Entries) = %d, esperado 2", len(res.Entries))
	}
	// E o par 2×N não pode reaparecer em nenhum lugar do resumo.
	if res.Hits != 2 {
		t.Errorf("Hits = %d, esperado 2", res.Hits)
	}
}

func TestDedup_MatchLevel_MesmaPartidaNaoApareceDuasVezes(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(7, 6, 5)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	vistos := map[int64]int{}
	for _, e := range res.Entries {
		vistos[e.MatchID]++
	}
	for id, n := range vistos {
		if n != 1 {
			t.Errorf("partida %d apareceu %d vezes em uma regra de partida inteira", id, n)
		}
	}
}

func TestDedup_TeamLevel_ContinuaComDuasOcorrencias(t *testing.T) {
	// Mercado de resultado: mandante e visitante têm desfechos distintos, então
	// DUAS ocorrências por partida é o comportamento correto — não é o defeito.
	m := partida(3, 5, 5)
	m.HomeGoals, m.AwayGoals = 2, 0 // mandante vence
	u := newFilterUsecaseWith([]domain.Match{m})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: MetricWin, Stake: 10, FixedOdd: 1.80, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2 (mandante e visitante são ocorrências "+
			"diferentes em mercado de resultado)", res.MatchCount)
	}
	if res.Hits != 1 || res.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, esperado 1/1 (só o mandante venceu)", res.Hits, res.Misses)
	}
}

func TestDedup_MatchLevelComMando_ContinuaSeparando(t *testing.T) {
	// Com home_away explícito, o usuário pediu a perspectiva de um lado. A
	// filtragem por mando é o que reduz, não a dedup.
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, HomeAway: "home", Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2 (uma linha de mandante por partida)", res.MatchCount)
	}
}

// =============================================================================
// Correção 2 — ESCANTEIOS + ODD FIXA
//
// Fase A provou: escanteios devolvia ZERO ocorrências em qualquer limiar mesmo
// com odd fixa informada, enquanto gols com a mesma configuração devolvia 200.
// A odd fixa simplesmente não era aceita para escanteios.
//
// Regra fixada: escanteios ACEITA odd fixa, e o resultado é marcado
// odds_source = "fixed". NUNCA "real" — a odd não veio de mercado.
// =============================================================================

func TestEscanteios_AceitaOddFixa_EMarcaComoFixed(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount == 0 {
		t.Fatal("escanteios com odd fixa devolveu ZERO ocorrências — é exatamente o defeito da Fase A")
	}
	if res.OddsSource != oddsSourceFixed {
		t.Errorf("OddsSource = %q, esperado %q", res.OddsSource, oddsSourceFixed)
	}
	if res.FinancialsReliable {
		t.Error("FinancialsReliable = true com odd fixa: a odd não veio de mercado, " +
			"o número financeiro descreve um cenário e não uma medida")
	}
	for _, e := range res.Entries {
		if e.Odd == nil || *e.Odd != 1.50 {
			t.Errorf("entrada com odd %v, esperado 1.50 (a odd informada)", e.Odd)
		}
		if e.OddsSource == "real" {
			t.Error("entrada marcada como odd REAL usando a odd digitada pelo usuário")
		}
	}
}

func TestEscanteios_OddRealTemPrecedenciaSobreAFixa(t *testing.T) {
	// Havendo odd de mercado, ela vence: a fixa é fallback, não substituta.
	m := partida(1, 6, 5)
	m.CornerOdds = map[string]float64{"8.5": 1.90}
	m.OddsSource = "real"
	u := newFilterUsecaseWith([]domain.Match{m})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if len(res.Entries) == 0 {
		t.Fatal("sem ocorrências")
	}
	if res.Entries[0].Odd == nil || *res.Entries[0].Odd != 1.90 {
		t.Errorf("odd = %v, esperado 1.90 (a odd real, não a fixa)", res.Entries[0].Odd)
	}
}

// =============================================================================
// Correção 3 — AUSÊNCIA DE ODD NÃO É ODD 1.00
//
// Antes, sem odd o motor usava 1.00 e produzia lucro, ROI e drawdown zerados
// que a tela exibia como se fossem medidas. Ausência de dado ≠ zero.
//
// Regra fixada: o bloco ESTATÍSTICO existe sempre que houver partidas; o bloco
// FINANCEIRO é nil quando não há odd, e FinancialsAvailable = false.
// =============================================================================

func TestSemOdd_EstatisticaExiste_FinanceiroENil(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(2, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	// Estatística: existe.
	if res.MatchCount != 2 || res.Hits != 2 {
		t.Errorf("estatística ausente sem odd: MatchCount=%d Hits=%d", res.MatchCount, res.Hits)
	}
	if res.HitRate != 100 {
		t.Errorf("HitRate = %v, esperado 100", res.HitRate)
	}
	// Financeiro: não calculável.
	if res.FinancialsAvailable {
		t.Error("FinancialsAvailable = true sem nenhuma odd")
	}
	for nome, v := range map[string]*float64{
		"Profit": res.Profit, "ROI": res.ROI, "Yield": res.Yield,
		"TotalStaked": res.TotalStaked, "MaxDrawdown": res.MaxDrawdown,
	} {
		if v != nil {
			t.Errorf("%s = %v, esperado nil — ausência de odd não é zero", nome, *v)
		}
	}
	for _, e := range res.Entries {
		if e.Odd != nil {
			t.Errorf("entrada com odd %v, esperado nil (nenhuma odd disponível)", *e.Odd)
		}
		if e.ProfitLoss != nil {
			t.Errorf("entrada com P/L %v, esperado nil", *e.ProfitLoss)
		}
	}
}

func TestSemOdd_NuncaUsaOddUm(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	for _, e := range res.Entries {
		if e.Odd != nil && *e.Odd == 1.0 {
			t.Error("odd 1.00 fabricada para partida sem odd — é o fallback removido no REV-P3")
		}
	}
}

// --- Os três tipos de ausência, que NÃO podem se confundir ------------------

func TestAusenciaA_NenhumaPartidaNoRecorte(t *testing.T) {
	u := newFilterUsecaseWith(nil)
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 0 {
		t.Errorf("MatchCount = %d, esperado 0", res.MatchCount)
	}
	if res.FinancialsAvailable {
		t.Error("FinancialsAvailable = true sem nenhuma partida")
	}
}

func TestAusenciaB_PartidasSemAMetrica(t *testing.T) {
	// Impedimentos não publicados pelo provedor: a partida existe, o dado não.
	// Ela fica de fora do backtest — e isso NÃO é o mesmo que ter dado zero.
	m := partida(1, 6, 5)
	m.HomeOffsides, m.AwayOffsides = nil, nil
	u := newFilterUsecaseWith([]domain.Match{m})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "offsides", OffsidesThreshold: 2, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 0 {
		t.Errorf("MatchCount = %d, esperado 0 — partida sem o dado não pode entrar como zero", res.MatchCount)
	}
}

func TestAusenciaC_ZeroObservadoContinuaSendoZero(t *testing.T) {
	// Zero impedimentos REGISTRADOS é uma observação válida: a partida entra no
	// backtest e conta como erro da linha "acima de 2".
	zero := 0
	m := partida(1, 6, 5)
	m.HomeOffsides, m.AwayOffsides = &zero, &zero
	u := newFilterUsecaseWith([]domain.Match{m})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "offsides", OffsidesThreshold: 2, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, esperado 1 — zero observado é observação, não ausência", res.MatchCount)
	}
	if res.Hits != 0 || res.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, esperado 0/1", res.Hits, res.Misses)
	}
}

// =============================================================================
// TESTE DETERMINÍSTICO OBRIGATÓRIO (especificado no prompt do REV-P3)
//
// 20 ocorrências · 15 WIN · 5 LOSS · stake R$100 · odd 1.50
//   → hit_rate 75% · lucro R$250 · total apostado R$2.000 · ROI 12,5%
//   → odds_source "fixed" · financials_reliable false · EV ausente (N/A)
//
// Conferência aritmética: 15 × 100 × (1,50 − 1) = +750; 5 × (−100) = −500;
// lucro = 250. Apostado = 20 × 100 = 2.000. ROI = 250 ÷ 2.000 = 12,5%.
// =============================================================================

func TestDeterministico_20Ocorrencias_15Win_5Loss_Stake100_Odd150(t *testing.T) {
	var ms []domain.Match
	for i := 1; i <= 15; i++ {
		ms = append(ms, partida(int64(i), 6, 5)) // 11 escanteios: acerta a linha 8
	}
	for i := 16; i <= 20; i++ {
		ms = append(ms, partida(int64(i), 2, 1)) // 3 escanteios: erra a linha 8
	}

	u := newFilterUsecaseWith(ms)
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}

	if res.MatchCount != 20 {
		t.Fatalf("MatchCount = %d, esperado 20 (40 significa dupla contagem)", res.MatchCount)
	}
	if res.Hits != 15 || res.Misses != 5 {
		t.Errorf("Hits/Misses = %d/%d, esperado 15/5", res.Hits, res.Misses)
	}
	if res.HitRate != 75 {
		t.Errorf("HitRate = %v, esperado 75", res.HitRate)
	}

	if !res.FinancialsAvailable {
		t.Fatal("FinancialsAvailable = false com odd fixa em TODAS as ocorrências")
	}
	esperado := map[string]struct {
		got  *float64
		want float64
	}{
		"Profit":      {res.Profit, 250},
		"TotalStaked": {res.TotalStaked, 2000},
		"ROI":         {res.ROI, 12.5},
		"Yield":       {res.Yield, 12.5},
	}
	for nome, c := range esperado {
		if c.got == nil {
			t.Errorf("%s = nil, esperado %v", nome, c.want)
			continue
		}
		if *c.got != c.want {
			t.Errorf("%s = %v, esperado %v", nome, *c.got, c.want)
		}
	}

	if res.OddsSource != oddsSourceFixed {
		t.Errorf("OddsSource = %q, esperado %q", res.OddsSource, oddsSourceFixed)
	}
	if res.FinancialsReliable {
		t.Error("FinancialsReliable = true: odd fixa NUNCA é medida de mercado")
	}
}

// O EV continua ausente do contrato — não existe campo para ele. Este teste
// falha se alguém reintroduzir um "valor esperado" calculado com a taxa de
// acerto do próprio lote, que é o defeito da correção 4.
func TestEV_PermaneceAusenteDoContrato(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 100, FixedOdd: 1.50}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, proibido := range []string{`"ev"`, `"expected_value"`, `"expected_roi"`} {
		if strings.Contains(string(b), proibido) {
			t.Errorf("campo %s voltou ao contrato do backtest — ver correção 4 do REV-P3", proibido)
		}
	}
}

// =============================================================================
// Correção 6 — RECORTE AUDITÁVEL
//
// O cap do plano gratuito é relativo a hoje, então a MESMA configuração analisa
// um conjunto diferente a cada dia. Sem as datas efetivas, dois backtests
// "iguais" com números diferentes são indistinguíveis de um bug.
// =============================================================================

func TestRecorteEfetivo_ComCapDoPlanoGratuito(t *testing.T) {
	recente := partida(1, 6, 5)
	recente.MatchDate = time.Now().AddDate(0, 0, -10)
	antiga := partida(2, 7, 4)
	antiga.MatchDate = time.Now().AddDate(0, 0, -400)

	u := newFilterUsecaseWith([]domain.Match{recente, antiga})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 90)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if !res.HistoryCapped || res.HistoryCapDays != 90 {
		t.Errorf("cap não reportado: capped=%v days=%d", res.HistoryCapped, res.HistoryCapDays)
	}
	if res.EffectiveFrom == "" || res.EffectiveTo == "" {
		t.Fatalf("recorte efetivo vazio (de=%q ate=%q) — o cap deixa de ser auditável",
			res.EffectiveFrom, res.EffectiveTo)
	}
	querDe := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	if res.EffectiveFrom != querDe {
		t.Errorf("EffectiveFrom = %q, esperado %q (hoje − 90 dias)", res.EffectiveFrom, querDe)
	}
	if res.MatchCount != 1 {
		t.Errorf("MatchCount = %d, esperado 1 (a partida de 400 dias atrás está fora do cap)", res.MatchCount)
	}
}

func TestRecorteEfetivo_SemCap_UsaJanelaObservada(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{partida(1, 6, 5), partida(5, 7, 4)})
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.HistoryCapped {
		t.Error("HistoryCapped = true sem cap aplicado")
	}
	if res.EffectiveFrom != "2025-03-01" || res.EffectiveTo != "2025-03-05" {
		t.Errorf("recorte observado = %q..%q, esperado 2025-03-01..2025-03-05",
			res.EffectiveFrom, res.EffectiveTo)
	}
}
