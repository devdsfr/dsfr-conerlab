package strategyengine

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/formulas"
)

// Testes de AUD-002 — ROI, Yield e EV eram o mesmo número ocupando três slots.
//
// O teste obrigatório definido na auditoria: "cenário com ROI fixo e demais
// componentes variando deve mover o DSFR proporcionalmente ao peso declarado".
// Ele é feito aqui nos dois sentidos, porque só o par flagra a regressão:
//
//   - variando SÓ o retorno, o DSFR não pode se mover mais do que o peso
//     declarado do ROI (sob a v1.0 andava ~50 pontos com peso nominal de 20);
//   - com o retorno FIXO, mexer nos demais componentes tem que mover o DSFR —
//     caso contrário o score voltou a ser função de uma variável só.
//
// Os testes existentes de engine_test.go são direcionais ("forte > fraca") e
// passavam igual antes e depois da correção; por isso não substituem estes.

const tol = 1e-9

// dsfrVariandoApenasRetorno devolve o DSFR de dois cenários idênticos exceto
// pelo retorno: um com ROI 0 e outro com ROI acima do teto de normalização
// (roiCapPct), isto é, roiNorm indo de 0 a 1.
func dsfrVariandoApenasRetorno(games, hits int, dd float64) (baixo, alto float64) {
	// yield acompanha o ROI porque no motor eles SÃO o mesmo número — é
	// justamente isso que a v1.0 contava três vezes.
	lo := scoresRow(1, result(games, hits, 0, 0, dd), 50, 0)
	hi := scoresRow(2, result(games, hits, roiCapPct, yieldCapPct, dd), 50, 0)
	return lo.DSFRScore, hi.DSFRScore
}

func TestDSFR_RetornoNaoPodeUltrapassarSeuPesoDeclarado(t *testing.T) {
	baixo, alto := dsfrVariandoApenasRetorno(50, 35, 2)
	delta := alto - baixo

	esperado := formulas.DSFRv11WROI * 100 // 30 pontos
	if math.Abs(delta-esperado) > 1e-6 {
		t.Fatalf("variar só o retorno moveu o DSFR em %.4f pontos; o peso declarado do ROI é %.4f.\n"+
			"Se o delta for ~50, ROI/EV/Yield voltaram a ocupar três slots (AUD-002).", delta, esperado)
	}
}

func TestDSFR_ComRetornoFixoOsDemaisComponentesMovemOScore(t *testing.T) {
	// Retorno idêntico nos dois cenários; muda amostra, acerto e drawdown.
	fraco := scoresRow(1, result(10, 6, 12, 12, 9), 50, 0)
	forte := scoresRow(2, result(50, 45, 12, 12, 0), 50, 0)

	if forte.DSFRScore <= fraco.DSFRScore {
		t.Fatalf("com o mesmo retorno, DSFR não reagiu aos demais componentes: forte=%.2f fraco=%.2f",
			forte.DSFRScore, fraco.DSFRScore)
	}

	// E o movimento tem que caber no peso somado dos componentes que variaram
	// (win rate + drawdown + amostra + consistência + variância = 70).
	delta := forte.DSFRScore - fraco.DSFRScore
	maxPossivel := (1 - formulas.DSFRv11WROI) * 100
	if delta > maxPossivel+1e-6 {
		t.Fatalf("delta %.4f excede o peso somado dos componentes não-retorno (%.4f)", delta, maxPossivel)
	}
}

func TestDSFRV11_PesosSomamUmECadaSlotValeOQueDeclara(t *testing.T) {
	soma := formulas.DSFRv11WROI + formulas.DSFRv11WWinRate + formulas.DSFRv11WDrawdown +
		formulas.DSFRv11WSampleSize + formulas.DSFRv11WConsistency + formulas.DSFRv11WVariance
	if math.Abs(soma-1) > tol {
		t.Fatalf("pesos do DSFR v1.1 somam %v, deveriam somar 1", soma)
	}

	casos := []struct {
		nome string
		in   formulas.DSFRInputsV11
		want float64
	}{
		{"ROI", formulas.DSFRInputsV11{ROI: 1}, formulas.DSFRv11WROI * 100},
		{"WinRate", formulas.DSFRInputsV11{WinRate: 1}, formulas.DSFRv11WWinRate * 100},
		{"InvDrawdown", formulas.DSFRInputsV11{InvDrawdown: 1}, formulas.DSFRv11WDrawdown * 100},
		{"SampleSize", formulas.DSFRInputsV11{SampleSize: 1}, formulas.DSFRv11WSampleSize * 100},
		{"Consistency", formulas.DSFRInputsV11{Consistency: 1}, formulas.DSFRv11WConsistency * 100},
		{"InvVariance", formulas.DSFRInputsV11{InvVariance: 1}, formulas.DSFRv11WVariance * 100},
	}
	for _, c := range casos {
		if got := formulas.DSFRScoreV11(c.in); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("DSFR só %s = %.4f, esperado %.4f", c.nome, got, c.want)
		}
	}
	if got := formulas.DSFRScoreV11(formulas.DSFRInputsV11{}); got != 0 {
		t.Errorf("DSFR zerado = %v", got)
	}
	perfeito := formulas.DSFRInputsV11{
		ROI: 1, WinRate: 1, InvDrawdown: 1, SampleSize: 1, Consistency: 1, InvVariance: 1,
	}
	if got := formulas.DSFRScoreV11(perfeito); math.Abs(got-100) > 1e-9 {
		t.Errorf("DSFR perfeito = %v, esperado 100", got)
	}
}

// A v1.0 tem que continuar reproduzindo exatamente o que produzia: linhas
// antigas de `backtests` carregam algorithm_version = "1.0" e só fazem sentido
// sob a fórmula da época.
func TestDSFRV10_PreservadaParaHistorico(t *testing.T) {
	if got := formulas.DSFRScore(formulas.DSFRInputs{ROI: 1}); math.Abs(got-20) > tol {
		t.Errorf("DSFR v1.0 só ROI = %v, esperado 20", got)
	}
	if got := formulas.DSFRScore(formulas.DSFRInputs{EV: 1}); math.Abs(got-20) > tol {
		t.Errorf("DSFR v1.0 só EV = %v, esperado 20", got)
	}
	if got := formulas.DSFRScore(formulas.DSFRInputs{Yield: 1}); math.Abs(got-10) > tol {
		t.Errorf("DSFR v1.0 só Yield = %v, esperado 10", got)
	}
}

// Demonstra o defeito original de forma executável: sob a v1.0, uma única
// quantidade (o retorno) valia 50% do score. Serve de referência para o número
// citado na auditoria — e falha se alguém reintroduzir os slots duplicados.
func TestDSFRV10_RetornoValia50PorCento(t *testing.T) {
	// roiNorm e yieldNorm derivam do MESMO número, com tetos 20 e 15.
	roiNorm, yieldNorm := 1.0, 1.0
	so := formulas.DSFRScore(formulas.DSFRInputs{ROI: roiNorm, EV: yieldNorm, Yield: yieldNorm})
	if math.Abs(so-50) > tol {
		t.Fatalf("v1.0 com só o retorno preenchido = %.4f, esperado 50 (20 ROI + 20 EV + 10 Yield)", so)
	}
}

func TestRankingV11_SemYieldEPesosSomamUm(t *testing.T) {
	soma := formulas.Rankingv11WDSFR + formulas.Rankingv11WHealth +
		formulas.Rankingv11WROI + formulas.Rankingv11WConfidence
	if math.Abs(soma-1) > tol {
		t.Fatalf("pesos do Ranking v1.1 somam %v", soma)
	}
	if got := formulas.RankingScoreV11(100, 0, 0, 0); math.Abs(got-40) > 1e-9 {
		t.Errorf("ranking só DSFR = %v, esperado 40", got)
	}
	if got := formulas.RankingScoreV11(100, 100, 1, 100); math.Abs(got-100) > 1e-9 {
		t.Errorf("ranking máximo = %v, esperado 100", got)
	}
}

func TestHealthV11_TresDeltasNaoQuatro(t *testing.T) {
	// Só o drawdown melhora (delta negativo = drawdown caindo = saúde subindo).
	// Com 3 deltas, o efeito é 50 × (1/3); com 4 seria 50 × (1/4).
	got := formulas.HealthScoreV11(0, -1, 0)
	want := 50 + 50.0/3
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("HealthScoreV11(0,-1,0) = %.4f, esperado %.4f (média de 3 deltas)", got, want)
	}
	if got := formulas.HealthScoreV11(0, 0, 0); math.Abs(got-50) > tol {
		t.Errorf("sem variação deveria ser 50 (estável): %v", got)
	}
	if got := formulas.HealthScoreV11(1, -1, 1); math.Abs(got-100) > tol {
		t.Errorf("tudo melhorando deveria ser 100: %v", got)
	}
}

// AUD-002: o campo EV não pode mais receber o yield emprestado.
func TestBacktestRow_EVFicaNulo(t *testing.T) {
	bt := backtestRow(7, result(40, 30, 12.5, 12.5, 4.0))
	if bt.EV != nil {
		t.Fatalf("EV deveria ser NULL (não é calculado em lugar nenhum), veio %v", *bt.EV)
	}
	// ROI e Yield continuam gravados: são cálculos corretos do Catálogo 07 e 08,
	// apenas não são independentes entre si.
	if bt.ROI == nil || bt.Yield == nil {
		t.Fatal("ROI e Yield deveriam continuar preenchidos")
	}
	if bt.AlgorithmVersion != formulas.Version {
		t.Errorf("algorithm_version = %q, esperado %q", bt.AlgorithmVersion, formulas.Version)
	}
}

// Os componentes exibidos na tela de detalhe não podem mais listar "ev" e
// "yield" como evidências separadas — eram o roiNorm reescalado.
func TestScoresRow_ComponentesSemEVeYield(t *testing.T) {
	s := scoresRow(1, result(50, 40, 20, 15, 1), 75, 0.3)

	var comp map[string]float64
	if err := json.Unmarshal([]byte(s.Components), &comp); err != nil {
		t.Fatalf("components não é JSON válido: %v", err)
	}
	for _, proibido := range []string{"ev", "yield"} {
		if _, ok := comp[proibido]; ok {
			t.Errorf("components ainda expõe %q como dimensão do score", proibido)
		}
	}
	for _, esperado := range []string{"roi", "win_rate", "inv_drawdown", "sample", "consistency", "inv_variance"} {
		if _, ok := comp[esperado]; !ok {
			t.Errorf("components não traz %q", esperado)
		}
	}
	if len(comp) != 6 {
		t.Errorf("components tem %d chaves, esperado 6 (uma por slot do DSFR v1.1)", len(comp))
	}
}

func TestHealthRow_VariacaoSemChaveEV(t *testing.T) {
	prevROI, prevYield, prevDD := 5.0, 5.0, 5.0
	prev := &domain.Backtest{Games: 40, Wins: 24, ROI: &prevROI, Yield: &prevYield, Drawdown: &prevDD}

	h := healthRow(1, result(40, 30, 15, 15, 3), prev)

	var v map[string]float64
	if err := json.Unmarshal([]byte(h.Variation), &v); err != nil {
		t.Fatalf("variation não é JSON válido: %v", err)
	}
	if _, ok := v["ev"]; ok {
		t.Error("variation ainda expõe delta de \"ev\" — era o mesmo ΔROI reescalado")
	}
	if len(v) != 3 {
		t.Errorf("variation tem %d deltas, esperado 3 (roi, drawdown, consistency)", len(v))
	}
}
