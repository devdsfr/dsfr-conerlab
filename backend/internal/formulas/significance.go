package formulas

import "math"

// Significância estatística e correção para testes múltiplos (Catálogo v1.1,
// itens 41–43 — acrescentados pela correção do AUD-003).
//
// POR QUE ISTO EXISTE. O Discovery Engine testa milhares de combinações contra o
// mesmo histórico. Testar muito e publicar o melhor é um gerador de falsos
// positivos: com 5.940 testes independentes a 5% de significância, ~297
// combinações passariam por puro acaso mesmo que nenhuma tivesse vantagem
// nenhuma. Sem medir isso, "a estratégia acerta 80%" não distingue vantagem de
// ruído.
//
// A pergunta certa NÃO é "a taxa de acerto é alta?" — uma linha de escanteios
// baixa acerta 90% e paga odd 1.05, o que é prejuízo. A pergunta é: **a taxa de
// acerto observada é maior do que a odd oferecida já embutia?** A odd implícita
// (1/odd, Catálogo 02) é a hipótese nula: se a casa precifica corretamente, a
// frequência de acerto no longo prazo é 1/odd. Só bate o mercado quem acerta
// significativamente mais do que isso.

// BinomialAtLeast devolve P(X >= k) para X ~ Binomial(n, p) — o p-valor
// unilateral de "observei k acertos em n tentativas; a chance de isso acontecer
// por acaso, se a probabilidade real fosse p".
//
// Calculado por soma exata em espaço logarítmico (log-gama), não por
// aproximação normal: com n pequeno (dezenas de apostas) e p perto de 1, a
// aproximação normal erra justamente na cauda que interessa. Soma a partir da
// extremidade menor para não perder precisão.
//
// k <= 0 devolve 1 (é certo observar pelo menos zero acertos).
func BinomialAtLeast(n, k int, p float64) (float64, error) {
	if n < 0 || k > n {
		return 0, ErrInvalidInput
	}
	if p < 0 || p > 1 {
		return 0, ErrInvalidProbability
	}
	if k <= 0 {
		return 1, nil
	}
	if n == 0 {
		return 0, nil
	}
	// Casos degenerados de p: sem eles, log(0) contamina a soma.
	if p == 0 {
		return 0, nil // impossível acertar; P(X >= k >= 1) = 0
	}
	if p == 1 {
		return 1, nil // acerta sempre; X = n >= k
	}

	logP, logQ := math.Log(p), math.Log(1-p)
	sum := 0.0
	for i := k; i <= n; i++ {
		logTerm := logChoose(n, i) + float64(i)*logP + float64(n-i)*logQ
		sum += math.Exp(logTerm)
	}
	if sum > 1 {
		sum = 1 // resíduo de ponto flutuante
	}
	return sum, nil
}

// logChoose devolve ln(C(n,k)) via log-gama — evita estourar o float64 em
// fatoriais de n grande.
func logChoose(n, k int) float64 {
	return lgamma(float64(n)+1) - lgamma(float64(k)+1) - lgamma(float64(n-k)+1)
}

func lgamma(x float64) float64 {
	v, _ := math.Lgamma(x)
	return v
}

// MultipleTestingMethod escolhe o procedimento de correção.
type MultipleTestingMethod string

const (
	// FDRBenjaminiHochberg controla a taxa de falsas descobertas assumindo
	// testes independentes ou com dependência positiva.
	FDRBenjaminiHochberg MultipleTestingMethod = "bh"

	// FDRBenjaminiYekutieli controla a FDR sob dependência ARBITRÁRIA, ao custo
	// de ser mais conservador (divide o limiar por H(m) = 1 + 1/2 + ... + 1/m).
	//
	// É o padrão do CornerLab. As combinações do Discovery compartilham
	// fortemente as mesmas partidas — "escanteios 8.5+ casa e fora" e
	// "escanteios 8.5+ mandante" são o mesmo jogo visto duas vezes —, então a
	// hipótese de dependência positiva do BH não é demonstrável aqui. Preferir o
	// procedimento conservador é a escolha honesta quando a estrutura de
	// dependência é desconhecida: erra publicando de menos, não de mais.
	FDRBenjaminiYekutieli MultipleTestingMethod = "by"
)

// FDRThreshold devolve o maior p-valor que ainda é considerado significativo
// dado o conjunto de p-valores testados, controlando a taxa de falsas
// descobertas em q (Catálogo 42/43).
//
// Procedimento (Benjamini–Hochberg): ordena os m p-valores em ordem crescente,
// encontra o maior i tal que p(i) <= (i/m)·q·c e devolve p(i). Em
// Benjamini–Yekutieli, c = 1/H(m); em BH, c = 1.
//
// Devolve 0 quando NENHUM p-valor sobrevive — o que significa "não publique
// nada", e não "publique tudo". O chamador precisa tratar 0 como limiar
// impossível de atingir (nenhum p-valor real é <= 0).
//
// pvalues não é modificado.
func FDRThreshold(pvalues []float64, q float64, method MultipleTestingMethod) (float64, error) {
	if q <= 0 || q > 1 {
		return 0, ErrInvalidProbability
	}
	m := len(pvalues)
	if m == 0 {
		return 0, nil
	}

	sorted := make([]float64, m)
	copy(sorted, pvalues)
	insertionSortFloats(sorted)

	c := 1.0
	if method == FDRBenjaminiYekutieli {
		c = 1 / harmonic(m)
	}

	threshold := 0.0
	for i := 1; i <= m; i++ {
		limit := float64(i) / float64(m) * q * c
		if sorted[i-1] <= limit {
			threshold = sorted[i-1]
		}
	}
	return threshold, nil
}

// harmonic devolve H(m) = 1 + 1/2 + ... + 1/m.
func harmonic(m int) float64 {
	h := 0.0
	for i := 1; i <= m; i++ {
		h += 1 / float64(i)
	}
	return h
}

// insertionSortFloats ordena in-place em ordem crescente. Insertion sort porque
// a lista aqui tem no máximo alguns milhares de elementos e a dependência do
// pacote formulas precisa continuar sendo só a stdlib matemática — sem "sort"
// para manter o pacote auditável linha a linha, como pede o catálogo.
func insertionSortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		x := v[i]
		j := i - 1
		for j >= 0 && v[j] > x {
			v[j+1] = v[j]
			j--
		}
		v[j+1] = x
	}
}
