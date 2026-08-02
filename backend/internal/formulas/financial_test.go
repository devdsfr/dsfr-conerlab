package formulas

import "testing"

func TestExpectedValue(t *testing.T) {
	cases := []struct{ stake, odd, p, want float64 }{
		{100, 1.60, 0.75, 20}, // exemplo oficial do catálogo → EV = 20
		{100, 2.00, 0.50, 0},  // jogo justo
		{100, 2.00, 0.60, 20},
		{100, 2.00, 0.40, -20},     // EV negativo
		{10, 1.50, 0.6667, 0.0005}, // ~breakeven
		{100, 1.01, 1.00, 1},       // borda: sempre ganha na odd mínima
		{100, 10.0, 0.10, 0},       // breakeven em odd alta
		{50, 3.00, 0.40, 10},
		{100, 1.80, 0.60, 8},
		{0, 2.00, 0.50, 0}, // stake zero → EV zero
	}
	for _, c := range cases {
		got, err := ExpectedValue(c.stake, c.odd, c.p)
		noErr(t, err, "ExpectedValue")
		approx(t, got, c.want, "ExpectedValue")
	}
	if _, err := ExpectedValue(100, 1.0, 0.5); err == nil {
		t.Error("odd 1.0 deveria falhar")
	}
	if _, err := ExpectedValue(100, 2.0, 1.5); err == nil {
		t.Error("p>1 deveria falhar")
	}
	if _, err := ExpectedValue(-10, 2.0, 0.5); err == nil {
		t.Error("stake negativa deveria falhar")
	}
}

func TestROIPercent(t *testing.T) {
	cases := []struct{ profit, inv, want float64 }{
		{500, 2000, 25}, // exemplo do catálogo
		{0, 1000, 0},
		{-100, 1000, -10}, // prejuízo
		{1000, 1000, 100},
		{50, 200, 25},
		{1, 10000, 0.01},
		{2500, 10000, 25},
		{-500, 500, -100}, // perdeu tudo
		{10, 40, 25},
		{333, 1000, 33.3},
	}
	for _, c := range cases {
		got, err := ROIPercent(c.profit, c.inv)
		noErr(t, err, "ROIPercent")
		approx(t, got, c.want, "ROIPercent")
	}
	if _, err := ROIPercent(100, 0); err == nil {
		t.Error("investimento 0 deveria falhar")
	}
	if _, err := ROIPercent(100, -50); err == nil {
		t.Error("investimento negativo deveria falhar")
	}
}

func TestYieldPercent(t *testing.T) {
	cases := []struct{ profit, vol, want float64 }{
		{100, 1000, 10},
		{0, 500, 0},
		{-50, 1000, -5},
		{500, 2000, 25},
		{5, 100, 5},
		{75, 300, 25},
		{-100, 200, -50},
		{1, 1000, 0.1},
		{250, 500, 50},
		{60, 240, 25},
	}
	for _, c := range cases {
		got, err := YieldPercent(c.profit, c.vol)
		noErr(t, err, "YieldPercent")
		approx(t, got, c.want, "YieldPercent")
	}
	if _, err := YieldPercent(10, 0); err == nil {
		t.Error("volume 0 deveria falhar")
	}
}

func TestProfitFactor(t *testing.T) {
	cases := []struct{ gp, gl, want float64 }{
		{200, 100, 2.0},
		{100, 100, 1.0},
		{50, 100, 0.5},
		{300, 150, 2.0},
		{0, 100, 0},
		{1000, 250, 4.0},
		{75, 300, 0.25},
		{999, 333, 3.0},
		{1, 1000, 0.001},
		{500, 125, 4.0},
	}
	for _, c := range cases {
		got, err := ProfitFactor(c.gp, c.gl)
		noErr(t, err, "ProfitFactor")
		approx(t, got, c.want, "ProfitFactor")
	}
	if _, err := ProfitFactor(100, 0); err == nil {
		t.Error("prejuízo 0 deveria falhar (divisão por zero)")
	}
	if _, err := ProfitFactor(-1, 100); err == nil {
		t.Error("lucro bruto negativo deveria falhar")
	}
}

func TestRecoveryFactor(t *testing.T) {
	cases := []struct{ net, dd, want float64 }{
		{200, 100, 2.0},
		{100, 50, 2.0},
		{0, 100, 0},
		{-50, 100, -0.5}, // ainda no prejuízo
		{500, 100, 5.0},
		{100, 400, 0.25},
		{1000, 200, 5.0},
		{30, 60, 0.5},
		{75, 25, 3.0},
		{1, 1000, 0.001},
	}
	for _, c := range cases {
		got, err := RecoveryFactor(c.net, c.dd)
		noErr(t, err, "RecoveryFactor")
		approx(t, got, c.want, "RecoveryFactor")
	}
	if _, err := RecoveryFactor(100, 0); err == nil {
		t.Error("drawdown 0 deveria falhar")
	}
}

func TestExpectancy(t *testing.T) {
	cases := []struct{ wr, aw, lr, al, want float64 }{
		{0.60, 50, 0.40, 50, 10},
		{0.50, 100, 0.50, 100, 0},
		{0.75, 60, 0.25, 100, 20}, // espelha o EV do catálogo
		{0.40, 150, 0.60, 100, 0},
		{0.80, 25, 0.20, 100, 0},
		{0.30, 300, 0.70, 100, 20},
		{1.00, 50, 0.00, 0, 50},  // borda 100%
		{0.00, 0, 1.00, 50, -50}, // borda 0%
		{0.55, 90, 0.45, 110, 0},
		{0.65, 80, 0.35, 120, 10},
	}
	for _, c := range cases {
		got, err := Expectancy(c.wr, c.aw, c.lr, c.al)
		noErr(t, err, "Expectancy")
		approx(t, got, c.want, "Expectancy")
	}
	if _, err := Expectancy(1.5, 10, 0.5, 10); err == nil {
		t.Error("winRate>1 deveria falhar")
	}
	if _, err := Expectancy(0.5, -10, 0.5, 10); err == nil {
		t.Error("ganho médio negativo deveria falhar")
	}
}

func TestExpectancyPercent(t *testing.T) {
	cases := []struct{ e, stake, want float64 }{
		{10, 100, 10},
		{0, 50, 0},
		{-5, 100, -5},
		{20, 80, 25},
		{50, 200, 25},
		{1, 1000, 0.1},
		{-25, 50, -50},
		{100, 100, 100},
		{2.5, 10, 25},
		{7.5, 30, 25},
	}
	for _, c := range cases {
		got, err := ExpectancyPercent(c.e, c.stake)
		noErr(t, err, "ExpectancyPercent")
		approx(t, got, c.want, "ExpectancyPercent")
	}
	if _, err := ExpectancyPercent(10, 0); err == nil {
		t.Error("stake média 0 deveria falhar")
	}
}
