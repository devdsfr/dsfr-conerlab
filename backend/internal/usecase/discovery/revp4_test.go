package discovery

import (
	"context"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

// REV-P4 — Fases B/C. Testes das correções B1 (FDR sobre o conjunto completo),
// B1c (ciclo interrompido não desativa), B3 (funil) e da grade de 162.
//
// Usam os dublês e o gerador de ruído de outofsample_test.go.

// =============================================================================
// GRADE
// =============================================================================

func TestGrade_162_Por_Liga(t *testing.T) {
	combos := generateCombos(teamList(), false)
	escanteios, resultado := 0, 0
	for _, c := range combos {
		if c.isResult() {
			resultado++
		} else {
			escanteios++
		}
	}
	if escanteios != 135 {
		t.Errorf("escanteios = %d, esperado 135 (5 linhas × 3 mandos × 3 janelas × 3 tetos)", escanteios)
	}
	if resultado != 27 {
		t.Errorf("resultado = %d, esperado 27 (3 mercados × 3 mandos × 3 janelas)", resultado)
	}
	if len(combos) != 162 {
		t.Errorf("total = %d, esperado 162", len(combos))
	}
}

func TestGrade_PorEquipeSoma45(t *testing.T) {
	// O worker usa IncludeTeams=true: +5 linhas × 3 mandos × 3 tetos por equipe.
	com := generateCombos(teamList(), true)
	if want := 162 + 45*len(teamList()); len(com) != want {
		t.Errorf("com equipes = %d, esperado %d", len(com), want)
	}
}

// =============================================================================
// B1 — o FDR recebe o conjunto COMPLETO de hipóteses elegíveis
// =============================================================================

// elegiveisIndependente recalcula, fora do motor, quantas combinações passam
// SÓ nos critérios estruturais (amostra mínima + p-valor calculável). É o m
// que o FDR deve receber.
func elegiveisIndependente(t *testing.T, hist []domain.Match, crit Criteria) int {
	t.Helper()
	train, _, ok := splitHistory(hist, crit.TrainFraction)
	if !ok {
		t.Fatal("histórico não divisível")
	}
	fu := usecase.NewFilterUsecase(&fakeMatchRepo{matches: hist}, &fakeTeamRepo{teams: teamList()}, fakeLeagueRepo{})
	n := 0
	for _, c := range generateCombos(teamList(), false) {
		r, err := fu.RunBacktest(context.Background(), 1, nil, usecase.FilterCriteria{
			LastNGames: c.window, HomeAway: c.homeAway, CornersThreshold: c.line,
			MaxOdds: c.maxOdds, Metric: c.effectiveMetric(), RequireRealOdds: true,
			DateFrom: &train.From, DateTo: train.To,
		}, 0)
		if err != nil || r.MatchCount < crit.MinGames {
			continue
		}
		if _, ok := pValue(r); ok {
			n++
		}
	}
	return n
}

func TestB1_FDRRecebeTodasAsHipotesesElegiveis(t *testing.T) {
	hist := noiseHistory(700, 42)
	crit := DefaultCriteria()
	res, err := newTestEngine(hist, &fakeStrategyRepo{}, crit).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := elegiveisIndependente(t, hist, crit)
	if want == 0 {
		t.Fatal("cenário degenerado: nenhuma hipótese elegível — o teste não provaria nada")
	}
	if res.Funnel.TestedStatistically != want || res.Tested != want {
		t.Errorf("m entregue ao FDR = %d (Tested %d), esperado %d — todas as elegíveis",
			res.Funnel.TestedStatistically, res.Tested, want)
	}
}

func TestB1_FiltrosDeResultadoNaoReduzemM(t *testing.T) {
	// Mesmo histórico, critérios do doc 08 opostos (produção × permissivos):
	// o m do FDR tem de ser IDÊNTICO. Antes, com os de produção, era ~0,75.
	hist := noiseHistory(700, 7)
	prod, err := newTestEngine(hist, &fakeStrategyRepo{}, DefaultCriteria()).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	perm, err := newTestEngine(hist, &fakeStrategyRepo{}, permissiveDocCriteria()).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	// permissiveDocCriteria usa MinGames = 50; alinhamos a amostra, que é
	// critério estrutural legítimo, e comparamos só o efeito dos de resultado.
	semAmostra := DefaultCriteria()
	semAmostra.MinWinRate, semAmostra.MinROI, semAmostra.MinYield = 0.001, -1000, -1000
	semAmostra.MaxDrawdown, semAmostra.MinDSFR = 100, 0.001
	solto, err := newTestEngine(hist, &fakeStrategyRepo{}, semAmostra).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if prod.Funnel.TestedStatistically != solto.Funnel.TestedStatistically {
		t.Errorf("m com critérios de produção = %d, com critérios de resultado desligados = %d — "+
			"win rate/ROI/drawdown/score não podem reduzir m antes do FDR",
			prod.Funnel.TestedStatistically, solto.Funnel.TestedStatistically)
	}
	_ = perm
}

func TestB1_CriteriosDeResultadoAgemDepoisDoFDR(t *testing.T) {
	// Com critérios de produção sobre ruído, quem sobrevive ao FDR ainda pode
	// ser barrado pelos do doc 08 — e isso tem de aparecer DEPOIS no funil.
	for seed := int64(1); seed <= 40; seed++ {
		res, err := newTestEngine(noiseHistory(700, seed), &fakeStrategyRepo{}, DefaultCriteria()).
			RunLeague(context.Background(), 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		f := res.Funnel
		if f.RejectedSecondary > f.FDRSurvivors {
			t.Fatalf("seed %d: %d rejeitadas por critério secundário mas só %d sobreviveram ao FDR",
				seed, f.RejectedSecondary, f.FDRSurvivors)
		}
		if msg := f.Check(); msg != "" {
			t.Fatalf("seed %d: funil não fecha: %s", seed, msg)
		}
	}
}

func TestB1_QZeroPublicaZero(t *testing.T) {
	// Configuração nunca produz q = 0 (withDefaults devolve 0,10 — a correção
	// não é desligável). Forçado aqui por dentro para provar que o
	// procedimento, com q inválido, reprova o lote inteiro em vez de publicar.
	eng := newTestEngine(noiseHistory(700, 42), &fakeStrategyRepo{}, permissiveDocCriteria())
	eng.opts.Criteria.FDRq = 0
	res, err := eng.RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Funnel.TestedStatistically == 0 {
		t.Fatal("nenhuma hipótese testada — o teste não exercitaria o FDR")
	}
	if res.Published != 0 || res.Funnel.FDRSurvivors != 0 {
		t.Errorf("q = 0 publicou %d (sobreviventes %d); esperado 0", res.Published, res.Funnel.FDRSurvivors)
	}
	if res.Funnel.RejectedFDR != res.Funnel.TestedStatistically {
		t.Errorf("rejeitadas no FDR %d ≠ testadas %d", res.Funnel.RejectedFDR, res.Funnel.TestedStatistically)
	}
	if cfg := (Criteria{FDRq: 0}).withDefaults(); cfg.FDRq != DefaultFDRq {
		t.Errorf("withDefaults deixou q = %v; a correção não pode ser desligada por configuração", cfg.FDRq)
	}
}

// TESTE DE RUÍDO COM CRITÉRIOS DE PRODUÇÃO (obrigatório no REV-P4).
//
// O teste antigo (TestCicloSobreRuidoPuroNaoPublicaNada) usa critérios
// permissivos para exercitar o FDR, e por isso não enxergava o B1. Este usa os
// critérios DE PRODUÇÃO e 200 seeds.
//
// Garantias verificadas:
//   - nenhuma publicação em ruído puro (critério crítico);
//   - m médio compatível com "todas as elegíveis" (antes: 0,75);
//   - ciclos com falso "significativo" ≤ q nominal (BY com m correto garante
//     FWER ≤ q sob hipótese nula completa).
func TestB1_RuidoPuro_CriteriosDeProducao_200Seeds(t *testing.T) {
	if testing.Short() {
		t.Skip("200 ciclos completos; rode sem -short")
	}
	const seeds = 200
	crit := DefaultCriteria()
	var somaM, comSig, pub int
	for s := int64(1); s <= seeds; s++ {
		res, err := newTestEngine(noiseHistory(700, s), &fakeStrategyRepo{}, crit).
			RunLeague(context.Background(), 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if msg := res.Funnel.Check(); msg != "" {
			t.Fatalf("seed %d: funil não fecha: %s", s, msg)
		}
		somaM += res.Funnel.TestedStatistically
		if res.Funnel.FDRSurvivors > 0 {
			comSig++
		}
		pub += res.Published
	}
	mMedio := float64(somaM) / seeds
	t.Logf("ruído · critérios de produção · %d seeds × 700 partidas: m médio %.2f · "+
		"seeds com falso significativo %d (%.1f%%) · publicadas %d",
		seeds, mMedio, comSig, 100*float64(comSig)/seeds, pub)

	if pub != 0 {
		t.Fatalf("publicou %d estratégias sobre ruído puro — critério crítico", pub)
	}
	if mMedio < 10 {
		t.Errorf("m médio %.2f: o FDR voltou a receber um conjunto pré-selecionado (antes do REV-P4: 0,75)", mMedio)
	}
	if limite := int(crit.FDRq * seeds); comSig > limite {
		t.Errorf("%d de %d ciclos com falso significativo, acima do nominal q = %.0f%% (%d)",
			comSig, seeds, 100*crit.FDRq, limite)
	}
}

// =============================================================================
// B1c — ciclo interrompido não publica nem desativa
// =============================================================================

func TestB1c_CicloInterrompidoNaoDesativa(t *testing.T) {
	repo := &fakeStrategyRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // interrompido antes da primeira combinação
	res, err := newTestEngine(noiseHistory(700, 1), repo, permissiveDocCriteria()).RunLeague(ctx, 1, nil)
	if err == nil {
		t.Error("ciclo interrompido devolveu sucesso")
	}
	if !res.Funnel.Interrupted {
		t.Error("funil não marcou a interrupção")
	}
	if repo.deactivateCalls != 0 {
		t.Errorf("DeactivateDiscoveredExcept chamado %d vez(es) num ciclo interrompido — "+
			"retiraria do ar todas as descobertas vigentes da liga", repo.deactivateCalls)
	}
	if len(repo.published) != 0 {
		t.Errorf("publicou %d num ciclo interrompido", len(repo.published))
	}
}

func TestB1c_CicloCompletoDesativaNormalmente(t *testing.T) {
	repo := &fakeStrategyRepo{}
	if _, err := newTestEngine(noiseHistory(700, 1), repo, DefaultCriteria()).RunLeague(context.Background(), 1, nil); err != nil {
		t.Fatal(err)
	}
	if repo.deactivateCalls != 1 {
		t.Errorf("ciclo completo chamou DeactivateDiscoveredExcept %d vez(es), esperado 1", repo.deactivateCalls)
	}
}

// =============================================================================
// B3 — funil com a causa real
// =============================================================================

func semOddReal(hist []domain.Match, fonte string) []domain.Match {
	for i := range hist {
		hist[i].OddsSource = fonte
		if fonte == "" {
			hist[i].CornerOdds = nil
		}
	}
	return hist
}

// causasIndependentes reclassifica cada combinação FORA do motor, a partir do
// accounting do backtest: quantas faltariam amostra por falta de odd real (a
// amostra bastaria somando as observações excluídas por odd) e quantas faltam
// amostra de qualquer jeito.
func causasIndependentes(t *testing.T, hist []domain.Match, crit Criteria) (semOdd, semAmostra, resultadoSemOdd int) {
	t.Helper()
	train, _, ok := splitHistory(hist, crit.TrainFraction)
	if !ok {
		t.Fatal("histórico não divisível")
	}
	fu := usecase.NewFilterUsecase(&fakeMatchRepo{matches: hist}, &fakeTeamRepo{teams: teamList()}, fakeLeagueRepo{})
	for _, c := range generateCombos(teamList(), false) {
		r, err := fu.RunBacktest(context.Background(), 1, nil, usecase.FilterCriteria{
			LastNGames: c.window, HomeAway: c.homeAway, CornersThreshold: c.line,
			MaxOdds: c.maxOdds, Metric: c.effectiveMetric(), RequireRealOdds: true,
			DateFrom: &train.From, DateTo: train.To,
		}, 0)
		if err != nil || r.MatchCount >= crit.MinGames {
			continue
		}
		if r.MatchCount+r.Accounting.ExcludedNoOdd >= crit.MinGames {
			semOdd++
			if c.isResult() {
				resultadoSemOdd++
			}
		} else {
			semAmostra++
		}
	}
	return
}

func TestB3_SemOddReal_CausaCorreta(t *testing.T) {
	// Situação de PRODUÇÃO: nenhuma partida com odd de mercado. Antes, 162/162
	// apareciam como "amostra_insuficiente". Agora a causa é separada: as que
	// teriam amostra se houvesse odd real saem como "sem_odd_real"; as que não
	// teriam amostra nem assim (sobretudo as de janela curta) continuam como
	// "amostra_insuficiente" — que para elas é a verdade.
	for _, fonte := range []string{domain.OddsSourceSynthetic, "unknown", ""} {
		hist := semOddReal(noiseHistory(700, 42), fonte)
		crit := DefaultCriteria()
		res, err := newTestEngine(hist, &fakeStrategyRepo{}, crit).RunLeague(context.Background(), 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		f := res.Funnel
		semOdd, semAmostra, resSemOdd := causasIndependentes(t, hist, crit)
		if f.NoRealOdds != semOdd || f.InsufficientSample != semAmostra {
			t.Errorf("fonte %q: motor sem_odd_real=%d/amostra=%d, recálculo independente %d/%d",
				fonte, f.NoRealOdds, f.InsufficientSample, semOdd, semAmostra)
		}
		if f.NoRealOdds+f.InsufficientSample != 162 {
			t.Errorf("fonte %q: %d + %d ≠ 162", fonte, f.NoRealOdds, f.InsufficientSample)
		}
		if f.NoRealOdds <= f.InsufficientSample {
			t.Errorf("fonte %q: sem odd real (%d) deveria ser a causa DOMINANTE (amostra: %d)",
				fonte, f.NoRealOdds, f.InsufficientSample)
		}
		// Os mercados de resultado nunca têm odd real hoje; os que teriam amostra
		// precisam aparecer como "sem odd real". (Não são os 27: os de janela de
		// 10 jogos com mando fixo não têm amostra nem com odd.)
		if resSemOdd == 0 {
			t.Errorf("fonte %q: nenhum mercado de resultado classificado sem odd real", fonte)
		}
		if res.Published != 0 || f.TestedStatistically != 0 {
			t.Errorf("fonte %q: sem odd real publicou %d / testou %d", fonte, res.Published, f.TestedStatistically)
		}
		if msg := f.Check(); msg != "" {
			t.Errorf("fonte %q: funil não fecha: %s", fonte, msg)
		}
	}
}

func TestB3_ComOddDeEscanteio_SoResultadoFicaSemOdd(t *testing.T) {
	// Escanteios COM odd real e nenhuma result_odds (o que o sistema teria no
	// dia em que entrar odd de escanteio): nenhuma combinação de ESCANTEIOS pode
	// ser rotulada "sem odd real" — só as de resultado.
	hist := noiseHistory(700, 5)
	crit := DefaultCriteria()
	res, err := newTestEngine(hist, &fakeStrategyRepo{}, crit).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	semOdd, _, resSemOdd := causasIndependentes(t, hist, crit)
	if semOdd != resSemOdd {
		t.Errorf("%d combinações de escanteios rotuladas sem odd real, com odd real presente", semOdd-resSemOdd)
	}
	if semOdd == 0 || res.Funnel.NoRealOdds != semOdd {
		t.Errorf("sem_odd_real motor %d, recálculo %d", res.Funnel.NoRealOdds, semOdd)
	}
	if msg := res.Funnel.Check(); msg != "" {
		t.Errorf("funil não fecha: %s", msg)
	}
}

func TestB3_AmostraInsuficienteDeVerdade(t *testing.T) {
	// Histórico curto COM odd real de escanteio: nas de escanteio a causa é
	// mesmo a amostra; "sem odd real" só pode aparecer nas de resultado.
	hist := noiseHistory(120, 3)
	crit := DefaultCriteria()
	res, err := newTestEngine(hist, &fakeStrategyRepo{}, crit).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	semOdd, semAmostra, resSemOdd := causasIndependentes(t, hist, crit)
	if semOdd != resSemOdd {
		t.Errorf("%d combinações de ESCANTEIOS rotuladas sem odd real, com odd real presente", semOdd-resSemOdd)
	}
	if res.Funnel.NoRealOdds != semOdd || res.Funnel.InsufficientSample != semAmostra {
		t.Errorf("motor %d/%d, recálculo %d/%d", res.Funnel.NoRealOdds, res.Funnel.InsufficientSample, semOdd, semAmostra)
	}
	if res.Funnel.InsufficientSample == 0 {
		t.Error("histórico curto deveria produzir 'amostra insuficiente'")
	}
	if msg := res.Funnel.Check(); msg != "" {
		t.Errorf("funil não fecha: %s", msg)
	}
}

func TestB3_FunilFecha_ComPublicacao(t *testing.T) {
	// Caso com vantagem REAL (odds generosas): o funil tem de fechar também no
	// caminho que publica.
	hist := noiseHistory(700, 5)
	for i := range hist {
		hist[i].CornerOdds = map[string]float64{"6.5": 1.60, "7.5": 2.10, "8.5": 3.00, "9.5": 4.50, "10.5": 7.00}
	}
	res, err := newTestEngine(hist, &fakeStrategyRepo{}, permissiveDocCriteria()).RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg := res.Funnel.Check(); msg != "" {
		t.Fatalf("funil não fecha: %s — %+v", msg, res.Funnel)
	}
	if res.Funnel.Published != res.Published {
		t.Errorf("funil publicadas %d ≠ resultado %d", res.Funnel.Published, res.Published)
	}
	t.Logf("funil com vantagem real: %+v", res.Funnel)
}

func TestZeroDescobertasEEstadoValido(t *testing.T) {
	res, err := newTestEngine(semOddReal(noiseHistory(700, 9), ""), &fakeStrategyRepo{}, DefaultCriteria()).
		RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("zero descobertas não pode ser erro: %v", err)
	}
	if res.Published != 0 || res.Funnel.Generated != 162 {
		t.Errorf("esperado 162 geradas e 0 publicadas, veio %d e %d", res.Funnel.Generated, res.Published)
	}
}

// =============================================================================
// BH / BY — valores calculados à mão
// =============================================================================

func TestFDR_BH_E_BY_CalculadosAMao(t *testing.T) {
	// m = 5, q = 0,10. Limiares BH: i/5 · 0,10 = 0,02 · 0,04 · 0,06 · 0,08 · 0,10.
	ps := []float64{0.001, 0.030, 0.045, 0.200, 0.900}
	// BH: 0,001 ≤ 0,02 ✓ · 0,030 ≤ 0,04 ✓ · 0,045 ≤ 0,06 ✓ · 0,200 > 0,08 · 0,900 > 0,10
	// → maior i aprovado = 3 → limiar 0,045.
	bh, err := formulas.FDRThreshold(ps, 0.10, formulas.FDRBenjaminiHochberg)
	if err != nil || bh != 0.045 {
		t.Errorf("BH = %v (err %v), esperado 0,045", bh, err)
	}
	// BY: divide por H(5) = 1 + 1/2 + 1/3 + 1/4 + 1/5 = 2,2833…
	// Limiares: 0,00876 · 0,01752 · 0,02628 · … → só 0,001 passa → limiar 0,001.
	by, err := formulas.FDRThreshold(ps, 0.10, formulas.FDRBenjaminiYekutieli)
	if err != nil || by != 0.001 {
		t.Errorf("BY = %v (err %v), esperado 0,001", by, err)
	}
}

func TestB3_ResultadoAgregadoTrazFunilEMotivos(t *testing.T) {
	lr, err := newTestEngine(semOddReal(noiseHistory(700, 42), ""), &fakeStrategyRepo{}, DefaultCriteria()).
		RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := FromLeague(lr)
	if r.Funnel.Generated != 162 || r.Funnel.NoRealOdds != lr.Funnel.NoRealOdds {
		t.Errorf("funil agregado %+v não reflete a liga %+v", r.Funnel, lr.Funnel)
	}
	if r.Rejections[string(rejectNoRealOdds)] != lr.Funnel.NoRealOdds {
		t.Errorf("motivos no topo = %v; a tela lê `rejections` no topo do resultado", r.Rejections)
	}
	if msg := r.Funnel.Check(); msg != "" {
		t.Errorf("funil agregado não fecha: %s", msg)
	}
	// Duas ligas somadas continuam fechando.
	r.absorb(lr)
	if msg := r.Funnel.Check(); msg != "" || r.Funnel.Generated != 324 {
		t.Errorf("soma de duas ligas: %q, geradas %d", msg, r.Funnel.Generated)
	}
}
