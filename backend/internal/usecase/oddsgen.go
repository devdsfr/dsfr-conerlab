package usecase

import (
	"fmt"
	"math"
)

// SyntheticCornerOdds gera odds sintéticas para as linhas 4.5 a 10.5 escanteios a
// partir de uma aproximação normal (média = totalMu, desvio padrão = 3.0) com uma
// margem de casa de apostas de ~8%. Usado tanto pelo seed de dados de exemplo quanto
// pela sincronização com provedores reais (cmd/sync) quando o provedor não fornece
// odds históricas — o CornerLab não tem, nesta versão, uma fonte paga de odds de
// mercado, então este valor serve apenas para permitir o cálculo de ROI/yield
// hipotético no Simulador de Filtros, nunca como odds real de mercado.
func SyntheticCornerOdds(totalMu float64) map[string]float64 {
	stdDev := 3.0
	margin := 1.08
	odds := map[string]float64{}
	for _, line := range []float64{4.5, 5.5, 6.5, 7.5, 8.5, 9.5, 10.5} {
		prob := 1 - normalCDF(line, totalMu, stdDev)
		if prob < 0.03 {
			prob = 0.03
		}
		if prob > 0.97 {
			prob = 0.97
		}
		odd := math.Round((1/prob)*margin*100) / 100
		odds[fmt.Sprintf("%.1f", line)] = odd
	}
	return odds
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}
