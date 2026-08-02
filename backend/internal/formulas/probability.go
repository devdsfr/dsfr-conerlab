package formulas

// Probability (Catálogo 01) — chance de ocorrência de um evento.
//
//	P = eventos favoráveis / eventos possíveis
//
// Retorna fração em [0,1]. total == 0 é inválido; success > total é inválido.
func Probability(success, total int) (float64, error) {
	if total == 0 {
		return 0, ErrDivisionByZero
	}
	if success < 0 || total < 0 || success > total {
		return 0, ErrInvalidInput
	}
	return float64(success) / float64(total), nil
}

// ImpliedProbability (Catálogo 02) — probabilidade embutida numa odd decimal.
//
//	P = 1 / odd
func ImpliedProbability(odd float64) (float64, error) {
	if odd <= 1 {
		return 0, ErrInvalidOdd
	}
	return 1 / odd, nil
}

// FairOdds (Catálogo 03) — odd justa para uma probabilidade.
//
//	odd = 1 / P
//
// Critério do catálogo: nunca inferior a 1.01.
func FairOdds(probability float64) (float64, error) {
	if probability <= 0 || probability > 1 {
		return 0, ErrInvalidProbability
	}
	odd := 1 / probability
	if odd < 1.01 {
		odd = 1.01
	}
	return odd, nil
}

// BreakEven (Catálogo 04) — taxa de acerto mínima para não perder dinheiro
// apostando sempre na mesma odd.
//
//	BE = 1 / odd
//
// Retorna fração em [0,1] (0.625 = 62.5%).
func BreakEven(odd float64) (float64, error) {
	if odd <= 1 {
		return 0, ErrInvalidOdd
	}
	return 1 / odd, nil
}

// Edge (Catálogo 05) — vantagem matemática sobre a odd oferecida.
//
//	Edge = probabilidade real − probabilidade implícita
//
// Positivo = valor a favor; negativo = a odd paga menos do que deveria.
func Edge(realProbability, odd float64) (float64, error) {
	if realProbability < 0 || realProbability > 1 {
		return 0, ErrInvalidProbability
	}
	implied, err := ImpliedProbability(odd)
	if err != nil {
		return 0, err
	}
	return realProbability - implied, nil
}
