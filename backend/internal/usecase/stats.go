package usecase

import (
	"math"
	"sort"
)

// StatSummary contém as principais medidas estatísticas descritivas usadas
// em todo o sistema (Módulo 1 e Módulo 9 - Estatísticas Avançadas).
type StatSummary struct {
	Count            int     `json:"count"`
	Mean             float64 `json:"mean"`
	Max              int     `json:"max"`
	Min              int     `json:"min"`
	StdDev           float64 `json:"std_dev"`
	Variance         float64 `json:"variance"`
	Median           float64 `json:"median"`
	Mode             []int   `json:"mode"`
	Total            int     `json:"total"`
	CoefficientOfVar float64 `json:"coefficient_of_variation"` // desvio padrão / média
	ConsistencyIndex float64 `json:"consistency_index"`        // 1 - CV, limitado a [0,1]
}

// Percentile calcula o percentil p (0-100) de uma série de valores (interpolação linear).
func Percentile(values []int, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	if p <= 0 {
		return float64(sorted[0])
	}
	if p >= 100 {
		return float64(sorted[len(sorted)-1])
	}
	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return float64(sorted[lower])
	}
	frac := rank - float64(lower)
	return float64(sorted[lower]) + frac*float64(sorted[upper]-sorted[lower])
}

// Summarize calcula o resumo estatístico completo de uma série de valores inteiros
// (ex.: escanteios por jogo). Todos os cálculos são determinísticos e reproduzíveis.
func Summarize(values []int) StatSummary {
	n := len(values)
	if n == 0 {
		return StatSummary{}
	}

	sorted := append([]int(nil), values...)
	sort.Ints(sorted)

	sum := 0
	for _, v := range values {
		sum += v
	}
	mean := float64(sum) / float64(n)

	var sqDiffSum float64
	for _, v := range values {
		d := float64(v) - mean
		sqDiffSum += d * d
	}
	variance := sqDiffSum / float64(n) // variância populacional
	stdDev := math.Sqrt(variance)

	var median float64
	if n%2 == 0 {
		median = float64(sorted[n/2-1]+sorted[n/2]) / 2.0
	} else {
		median = float64(sorted[n/2])
	}

	freq := map[int]int{}
	maxFreq := 0
	for _, v := range values {
		freq[v]++
		if freq[v] > maxFreq {
			maxFreq = freq[v]
		}
	}
	var mode []int
	if maxFreq > 1 { // só existe "moda" clássica se algum valor se repete
		for v, f := range freq {
			if f == maxFreq {
				mode = append(mode, v)
			}
		}
		sort.Ints(mode)
	}

	cv := 0.0
	if mean != 0 {
		cv = stdDev / mean
	}
	consistency := 1 - cv
	if consistency < 0 {
		consistency = 0
	}
	if consistency > 1 {
		consistency = 1
	}

	return StatSummary{
		Count:            n,
		Mean:             round2(mean),
		Max:              sorted[n-1],
		Min:              sorted[0],
		StdDev:           round2(stdDev),
		Variance:         round2(variance),
		Median:           round2(median),
		Mode:             mode,
		Total:            sum,
		CoefficientOfVar: round4(cv),
		ConsistencyIndex: round4(consistency),
	}
}

// FrequencyAboveThreshold calcula, para um conjunto de thresholds, quantos jogos
// ficaram acima (>) de cada valor e a respectiva porcentagem.
type FrequencyResult struct {
	Threshold int     `json:"threshold"`
	Count     int     `json:"count"`
	Total     int     `json:"total"`
	Pct       float64 `json:"pct"`
}

func FrequencyAboveThresholds(values []int, thresholds []int) []FrequencyResult {
	results := make([]FrequencyResult, 0, len(thresholds))
	total := len(values)
	for _, t := range thresholds {
		count := 0
		for _, v := range values {
			if v > t {
				count++
			}
		}
		pct := 0.0
		if total > 0 {
			pct = round2(100 * float64(count) / float64(total))
		}
		results = append(results, FrequencyResult{Threshold: t, Count: count, Total: total, Pct: pct})
	}
	return results
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
