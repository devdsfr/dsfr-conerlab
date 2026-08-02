package formulas

import "testing"

func TestRates(t *testing.T) {
	cases := []struct {
		part, total int
		want        float64
	}{
		{8, 10, 80},
		{0, 10, 0},    // borda 0%
		{10, 10, 100}, // borda 100%
		{1, 3, 33.3333},
		{2, 3, 66.6667},
		{1, 2, 50},
		{45, 60, 75},
		{1, 1000, 0.1},
		{999, 1000, 99.9},
		{7, 28, 25},
	}
	for _, c := range cases {
		for name, fn := range map[string]func(int, int) (float64, error){
			"WinRatePercent":  WinRatePercent,
			"LossRatePercent": LossRatePercent,
			"PushRatePercent": PushRatePercent,
		} {
			got, err := fn(c.part, c.total)
			noErr(t, err, name)
			approx(t, got, c.want, name)
		}
	}
	if _, err := WinRatePercent(1, 0); err == nil {
		t.Error("total 0 deveria falhar")
	}
	if _, err := WinRatePercent(11, 10); err == nil {
		t.Error("part > total deveria falhar")
	}
	if _, err := LossRatePercent(-1, 10); err == nil {
		t.Error("part negativa deveria falhar")
	}
}

func TestStreaks(t *testing.T) {
	W, L, P := Win, Loss, Push
	cases := []struct {
		seq          []Outcome
		wantW, wantL int
	}{
		{[]Outcome{}, 0, 0},
		{[]Outcome{W}, 1, 0},
		{[]Outcome{L}, 0, 1},
		{[]Outcome{W, W, W}, 3, 0},
		{[]Outcome{L, L, L, L}, 0, 4},
		{[]Outcome{W, L, W, L, W}, 1, 1},
		{[]Outcome{W, W, L, L, L, W}, 2, 3},
		{[]Outcome{W, P, W, W}, 3, 0},    // push não interrompe
		{[]Outcome{L, P, L, W, L}, 1, 2}, // push neutro no meio da sequência
		{[]Outcome{P, P, P}, 0, 0},       // só pushes
		{[]Outcome{W, W, P, L, L, P, L}, 2, 3},
	}
	for i, c := range cases {
		if got := MaxConsecutiveWins(c.seq); got != c.wantW {
			t.Errorf("caso %d wins: got %d want %d", i, got, c.wantW)
		}
		if got := MaxConsecutiveLosses(c.seq); got != c.wantL {
			t.Errorf("caso %d losses: got %d want %d", i, got, c.wantL)
		}
	}
}
