package formulas

import "errors"

// Erros de validação de entrada (casos inválidos obrigatórios do catálogo).
var (
	ErrInvalidOdd         = errors.New("formulas: odd deve ser maior que 1.0")
	ErrInvalidProbability = errors.New("formulas: probabilidade deve estar entre 0 e 1")
	ErrDivisionByZero     = errors.New("formulas: divisão por zero")
	ErrNegativeValue      = errors.New("formulas: valor negativo não permitido")
	ErrEmptySeries        = errors.New("formulas: série vazia")
	ErrInvalidInput       = errors.New("formulas: entrada inválida")
)

// clamp01 limita v ao intervalo [0,1] — usado pelos scores compostos para que
// componentes normalizados fora da faixa não estourem a escala 0..100.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
