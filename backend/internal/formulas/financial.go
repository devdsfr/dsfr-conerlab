package formulas

// ExpectedValue (Catálogo 06) — lucro médio esperado de uma aposta.
//
//	EV = (Pwin × lucro) − (Ploss × perda)
//	lucro = stake × (odd − 1);  perda = stake;  Ploss = 1 − Pwin
//
// Exemplo do catálogo: stake 100, odd 1.60, Pwin 75% → EV = 20.
func ExpectedValue(stake, odd, winProbability float64) (float64, error) {
	if odd <= 1 {
		return 0, ErrInvalidOdd
	}
	if winProbability < 0 || winProbability > 1 {
		return 0, ErrInvalidProbability
	}
	if stake < 0 {
		return 0, ErrNegativeValue
	}
	profit := stake * (odd - 1)
	loss := stake
	return winProbability*profit - (1-winProbability)*loss, nil
}

// ROIPercent (Catálogo 07) — retorno sobre investimento, em %.
//
//	ROI = lucro / investimento × 100
//
// Exemplo do catálogo: lucro 500, investimento 2000 → 25%.
func ROIPercent(profit, investment float64) (float64, error) {
	if investment == 0 {
		return 0, ErrDivisionByZero
	}
	if investment < 0 {
		return 0, ErrNegativeValue
	}
	return profit / investment * 100, nil
}

// YieldPercent (Catálogo 08) — eficiência por volume apostado, em %.
//
//	Yield = lucro / volume apostado × 100
func YieldPercent(profit, totalStaked float64) (float64, error) {
	if totalStaked == 0 {
		return 0, ErrDivisionByZero
	}
	if totalStaked < 0 {
		return 0, ErrNegativeValue
	}
	return profit / totalStaked * 100, nil
}

// ProfitFactor (Catálogo 13) — razão entre lucro bruto e prejuízo bruto.
//
//	PF = lucro bruto / prejuízo bruto
//
// grossLoss deve ser informado como valor positivo (magnitude das perdas).
// Sem perdas (grossLoss == 0) é divisão por zero — o chamador decide como
// apresentar (normalmente "∞" ou o próprio lucro).
func ProfitFactor(grossProfit, grossLoss float64) (float64, error) {
	if grossLoss == 0 {
		return 0, ErrDivisionByZero
	}
	if grossProfit < 0 || grossLoss < 0 {
		return 0, ErrNegativeValue
	}
	return grossProfit / grossLoss, nil
}

// RecoveryFactor (Catálogo 33) — capacidade de recuperação após perdas.
//
//	RF = lucro líquido / drawdown máximo
//
// maxDrawdown em valor absoluto (> 0).
func RecoveryFactor(netProfit, maxDrawdown float64) (float64, error) {
	if maxDrawdown == 0 {
		return 0, ErrDivisionByZero
	}
	if maxDrawdown < 0 {
		return 0, ErrNegativeValue
	}
	return netProfit / maxDrawdown, nil
}

// Expectancy (Catálogo 36) — expectativa média por aposta em unidades
// monetárias.
//
//	E = (winRate × ganho médio) − (lossRate × perda média)
//
// winRate e lossRate como frações [0,1]; avgLoss como magnitude positiva.
func Expectancy(winRate, avgWin, lossRate, avgLoss float64) (float64, error) {
	if winRate < 0 || winRate > 1 || lossRate < 0 || lossRate > 1 {
		return 0, ErrInvalidProbability
	}
	if avgWin < 0 || avgLoss < 0 {
		return 0, ErrNegativeValue
	}
	return winRate*avgWin - lossRate*avgLoss, nil
}

// ExpectancyPercent (Catálogo 37) — expectancy relativa à stake média, em %.
//
//	E% = expectancy / stake média × 100
func ExpectancyPercent(expectancy, avgStake float64) (float64, error) {
	if avgStake == 0 {
		return 0, ErrDivisionByZero
	}
	if avgStake < 0 {
		return 0, ErrNegativeValue
	}
	return expectancy / avgStake * 100, nil
}
