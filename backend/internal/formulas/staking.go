package formulas

import "math"

// Kelly (Catálogo 14) — fração ótima da banca a arriscar.
//
//	Kelly = (b·p − q) / b
//	b = odd − 1;  p = win rate (fração);  q = 1 − p
//
// Retorna fração da banca (pode ser negativa quando não há edge — o chamador
// decide truncar em zero ou usar Kelly fracionado).
func Kelly(odd, winRate float64) (float64, error) {
	if odd <= 1 {
		return 0, ErrInvalidOdd
	}
	if winRate < 0 || winRate > 1 {
		return 0, ErrInvalidProbability
	}
	b := odd - 1
	q := 1 - winRate
	return (b*winRate - q) / b, nil
}

// StakePercent (Catálogo 15) — stake proporcional à banca.
//
//	stake = banca × percentual
//
// percent como fração (0.02 = 2% da banca).
func StakePercent(bankroll, percent float64) (float64, error) {
	if bankroll < 0 || percent < 0 {
		return 0, ErrNegativeValue
	}
	if percent > 1 {
		return 0, ErrInvalidInput
	}
	return bankroll * percent, nil
}

// StakeFixed (Catálogo 16) — stake sempre igual, independente da banca.
func StakeFixed(stake float64) (float64, error) {
	if stake < 0 {
		return 0, ErrNegativeValue
	}
	return stake, nil
}

// Reinvest (Catálogo 17) — reinvestimento do lucro na próxima stake.
//
//	nova stake = stake + lucro
//
// Lucro negativo (prejuízo) reduz a stake; nunca abaixo de zero.
func Reinvest(stake, profit float64) (float64, error) {
	if stake < 0 {
		return 0, ErrNegativeValue
	}
	next := stake + profit
	if next < 0 {
		next = 0
	}
	return next, nil
}

// CompoundInterest (Catálogo 18) — crescimento composto da banca.
//
//	capital final = capital inicial × (1 + r)^n
//
// rate como fração por período; periods ≥ 0.
func CompoundInterest(principal, rate float64, periods int) (float64, error) {
	if principal < 0 {
		return 0, ErrNegativeValue
	}
	if periods < 0 || rate < -1 {
		return 0, ErrInvalidInput
	}
	return principal * math.Pow(1+rate, float64(periods)), nil
}

// CAGR (Catálogo 38) — taxa composta de crescimento anual da banca.
//
//	CAGR = (capital final / capital inicial)^(1/anos) − 1
func CAGR(finalCapital, initialCapital, years float64) (float64, error) {
	if initialCapital == 0 || years == 0 {
		return 0, ErrDivisionByZero
	}
	if initialCapital < 0 || finalCapital < 0 || years < 0 {
		return 0, ErrNegativeValue
	}
	return math.Pow(finalCapital/initialCapital, 1/years) - 1, nil
}
