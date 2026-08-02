package formulas

// Outcome é o resultado de uma entrada no histórico (para rates e sequências).
type Outcome int

const (
	Loss Outcome = iota
	Win
	Push // devolução/anulada — não conta como win nem loss
)

// WinRatePercent (Catálogo 09) — wins / total × 100.
func WinRatePercent(wins, total int) (float64, error) {
	return ratePercent(wins, total)
}

// LossRatePercent (Catálogo 10) — losses / total × 100.
func LossRatePercent(losses, total int) (float64, error) {
	return ratePercent(losses, total)
}

// PushRatePercent (Catálogo 11) — pushes / total × 100.
func PushRatePercent(pushes, total int) (float64, error) {
	return ratePercent(pushes, total)
}

func ratePercent(part, total int) (float64, error) {
	if total == 0 {
		return 0, ErrDivisionByZero
	}
	if part < 0 || total < 0 || part > total {
		return 0, ErrInvalidInput
	}
	return float64(part) / float64(total) * 100, nil
}

// MaxConsecutiveLosses (Catálogo 39) — maior sequência de derrotas seguidas.
// Pushes não interrompem nem estendem a sequência (são neutros).
func MaxConsecutiveLosses(outcomes []Outcome) int {
	return maxStreak(outcomes, Loss)
}

// MaxConsecutiveWins (Catálogo 40) — maior sequência de vitórias seguidas.
// Pushes não interrompem nem estendem a sequência (são neutros).
func MaxConsecutiveWins(outcomes []Outcome) int {
	return maxStreak(outcomes, Win)
}

func maxStreak(outcomes []Outcome, target Outcome) int {
	best, current := 0, 0
	for _, o := range outcomes {
		switch {
		case o == target:
			current++
			if current > best {
				best = current
			}
		case o == Push:
			// neutro: mantém a sequência corrente
		default:
			current = 0
		}
	}
	return best
}
