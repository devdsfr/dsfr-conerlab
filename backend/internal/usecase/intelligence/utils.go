package intelligence

import (
	"encoding/json"
	"math"
)

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// extractBacktestSummary lê apenas os campos roi/profit/hit_rate do JSON armazenado
// em filter_backtests.result, sem depender do pacote usecase (evita import cíclico),
// já que o formato é estável (ver usecase.BacktestResult).
func extractBacktestSummary(resultJSON string) (roi float64, profit float64, hitRate float64) {
	var parsed struct {
		ROI     float64 `json:"roi"`
		Profit  float64 `json:"profit"`
		HitRate float64 `json:"hit_rate"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &parsed); err != nil {
		return 0, 0, 0
	}
	return parsed.ROI, parsed.Profit, parsed.HitRate
}
