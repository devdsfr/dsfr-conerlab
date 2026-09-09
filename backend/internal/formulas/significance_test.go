package formulas

import (
	"math"
	"testing"
)

// Testes do AUD-003 — significância e correção para testes múltiplos.

func TestBinomialAtLeast_CasosConhecidos(t *testing.T) {
	casos := []struct {
		nome    string
		n, k    int
		p, want float64
	}{
		// Moeda honesta: P(X >= 0) = 1; P(X >= n) = p^n.
		{"pelo menos zero é certo", 10, 0, 0.5, 1},
		{"todos os 10 numa moeda honesta", 10, 10, 0.5, math.Pow(0.5, 10)},
		{"pelo menos 1 em 1 lançamento", 1, 1, 0.5, 0.5},
		// Simetria da binomial com p=0.5: P(X >= 6 | n=10) = 0.376953125.
		{"6 ou mais em 10", 10, 6, 0.5, 0.376953125},
		// p extremos.
		{"p=0 nunca acerta", 5, 1, 0, 0},
		{"p=1 sempre acerta", 5, 5, 1, 1},
	}
	for _, c := range casos {
		got, err := BinomialAtLeast(c.n, c.k, c.p)
		if err != nil {
			t.Errorf("%s: erro inesperado %v", c.nome, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: BinomialAtLeast(%d,%d,%.2f) = %.10f, esperado %.10f",
				c.nome, c.n, c.k, c.p, got, c.want)
		}
	}
}

func TestBinomialAtLeast_SomaDaDistribuicaoEUm(t *testing.T) {
	// P(X >= 0) = 1 e P(X >= k) é monotonicamente decrescente em k.
	const n = 40
	p := 0.62
	anterior := 2.0
	for k := 0; k <= n; k++ {
		got, err := BinomialAtLeast(n, k, p)
		if err != nil {
			t.Fatalf("k=%d: %v", k, err)
		}
		if got > anterior+1e-12 {
			t.Fatalf("P(X>=%d)=%v cresceu em relação a P(X>=%d)=%v", k, got, k-1, anterior)
		}
		if got < 0 || got > 1 {
			t.Fatalf("k=%d: p-valor fora de [0,1]: %v", k, got)
		}
		anterior = got
	}
}

func TestBinomialAtLeast_EntradasInvalidas(t *testing.T) {
	if _, err := BinomialAtLeast(10, 11, 0.5); err == nil {
		t.Error("k > n deveria ser inválido")
	}
	if _, err := BinomialAtLeast(10, 5, 1.5); err == nil {
		t.Error("p > 1 deveria ser inválido")
	}
	if _, err := BinomialAtLeast(-1, 0, 0.5); err == nil {
		t.Error("n negativo deveria ser inválido")
	}
}

// O ponto central da correção: o limiar aperta conforme o número de testes cresce.
func TestFDRThreshold_ApertaComMaisTestes(t *testing.T) {
	// Um p-valor bom no meio de poucos testes...
	poucos := []float64{0.004, 0.4, 0.6}
	// ...e o MESMO p-valor no meio de muitos.
	muitos := make([]float64, 1000)
	muitos[0] = 0.004
	for i := 1; i < len(muitos); i++ {
		muitos[i] = 0.5
	}

	tPoucos, err := FDRThreshold(poucos, 0.10, FDRBenjaminiYekutieli)
	if err != nil {
		t.Fatal(err)
	}
	tMuitos, err := FDRThreshold(muitos, 0.10, FDRBenjaminiYekutieli)
	if err != nil {
		t.Fatal(err)
	}

	if !(0.004 <= tPoucos) {
		t.Errorf("com 3 testes, p=0.004 deveria sobreviver (limiar %v)", tPoucos)
	}
	if 0.004 <= tMuitos {
		t.Errorf("com 1000 testes, p=0.004 NÃO deveria sobreviver (limiar %v)", tMuitos)
	}
}

func TestFDRThreshold_BYEMaisConservadorQueBH(t *testing.T) {
	pvalues := []float64{0.001, 0.008, 0.02, 0.05, 0.2, 0.5, 0.7, 0.9}

	bh, err := FDRThreshold(pvalues, 0.10, FDRBenjaminiHochberg)
	if err != nil {
		t.Fatal(err)
	}
	by, err := FDRThreshold(pvalues, 0.10, FDRBenjaminiYekutieli)
	if err != nil {
		t.Fatal(err)
	}
	if by > bh {
		t.Errorf("Benjamini–Yekutieli (%v) não pode ser mais permissivo que Benjamini–Hochberg (%v)", by, bh)
	}
}

// Se nada é significativo, o limiar tem que ser 0 — e 0 significa "não publique
// nada", nunca "sem limiar, publique tudo".
func TestFDRThreshold_NadaSignificativoDevolveZero(t *testing.T) {
	ruido := []float64{0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95}
	got, err := FDRThreshold(ruido, 0.10, FDRBenjaminiYekutieli)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("limiar = %v, esperado 0 quando nenhum p-valor sobrevive", got)
	}
	// E nenhum p-valor real passa na comparação `p <= 0`.
	for _, p := range ruido {
		if p <= got {
			t.Errorf("p=%v passou num limiar de %v", p, got)
		}
	}
}

func TestFDRThreshold_NaoModificaEntrada(t *testing.T) {
	original := []float64{0.9, 0.1, 0.5}
	copia := append([]float64(nil), original...)
	if _, err := FDRThreshold(original, 0.10, FDRBenjaminiHochberg); err != nil {
		t.Fatal(err)
	}
	for i := range original {
		if original[i] != copia[i] {
			t.Fatalf("FDRThreshold reordenou a entrada: %v", original)
		}
	}
}

func TestFDRThreshold_ListaVaziaEqValido(t *testing.T) {
	got, err := FDRThreshold(nil, 0.10, FDRBenjaminiYekutieli)
	if err != nil || got != 0 {
		t.Errorf("lista vazia: got=%v err=%v, esperado 0/nil", got, err)
	}
	if _, err := FDRThreshold([]float64{0.1}, 0, FDRBenjaminiYekutieli); err == nil {
		t.Error("q = 0 deveria ser inválido")
	}
}
