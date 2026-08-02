package formulas

import "math"

// Variance (Catálogo 20) — variância populacional.
//
//	σ² = Σ(x−μ)² / N
func Variance(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, ErrEmptySeries
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return sum / float64(len(values)), nil
}

// StdDev (Catálogo 21) — desvio padrão populacional.
//
//	σ = √variância
func StdDev(values []float64) (float64, error) {
	v, err := Variance(values)
	if err != nil {
		return 0, err
	}
	return math.Sqrt(v), nil
}

// MaxDrawdown (Catálogo 12) — maior queda relativa entre um pico e o vale
// seguinte na curva de banca/equity.
//
//	DD = (pico − vale) / pico
//
// equity é a série acumulada (ex.: banca após cada aposta). Retorna fração em
// [0,1] (0.25 = queda de 25% do pico). Série vazia é inválida; valores devem
// ser positivos (banca não-negativa) para a razão fazer sentido.
func MaxDrawdown(equity []float64) (float64, error) {
	if len(equity) == 0 {
		return 0, ErrEmptySeries
	}
	peak := equity[0]
	maxDD := 0.0
	for _, v := range equity {
		if v < 0 {
			return 0, ErrNegativeValue
		}
		if v > peak {
			peak = v
		}
		if peak > 0 {
			dd := (peak - v) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD, nil
}

// MaxDrawdownAbs — variação absoluta do drawdown (pico − vale), na mesma
// unidade da série (unidades de stake ou moeda). Não faz parte do índice do
// catálogo, mas é a forma exibida hoje no Simulador; documentada aqui para
// haver uma única implementação.
func MaxDrawdownAbs(equity []float64) (float64, error) {
	if len(equity) == 0 {
		return 0, ErrEmptySeries
	}
	peak := equity[0]
	maxDD := 0.0
	for _, v := range equity {
		if v > peak {
			peak = v
		}
		if dd := peak - v; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD, nil
}

// SharpeAdapted (Catálogo 34) — retorno ajustado ao risco, adaptado para
// estratégias esportivas.
//
//	Sharpe = (ROI médio − ROI livre de risco) / desvio padrão dos ROIs
//
// Como não existe taxa livre de risco equivalente no esporte, riskFreeROI é
// configurável (o catálogo permite omitir usando 0).
func SharpeAdapted(meanROI, riskFreeROI, stdDevROI float64) (float64, error) {
	if stdDevROI == 0 {
		return 0, ErrDivisionByZero
	}
	if stdDevROI < 0 {
		return 0, ErrNegativeValue
	}
	return (meanROI - riskFreeROI) / stdDevROI, nil
}

// CalmarAdapted (Catálogo 35) — retorno relativo ao pior momento.
//
//	Calmar = ROI / drawdown máximo
//
// maxDrawdown como fração ou magnitude positiva — consistente com o ROI usado.
func CalmarAdapted(roi, maxDrawdown float64) (float64, error) {
	if maxDrawdown == 0 {
		return 0, ErrDivisionByZero
	}
	if maxDrawdown < 0 {
		return 0, ErrNegativeValue
	}
	return roi / maxDrawdown, nil
}
