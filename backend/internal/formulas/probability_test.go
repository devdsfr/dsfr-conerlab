package formulas

import "testing"

func TestProbability(t *testing.T) {
	cases := []struct {
		success, total int
		want           float64
	}{
		{8, 10, 0.80},  // exemplo do catálogo
		{0, 10, 0.00},  // borda 0%
		{10, 10, 1.00}, // borda 100%
		{1, 2, 0.50},
		{1, 3, 0.3333},
		{7, 9, 0.7778},
		{25, 100, 0.25},
		{99, 100, 0.99},
		{1, 1000, 0.001},
		{333, 1000, 0.333},
	}
	for _, c := range cases {
		got, err := Probability(c.success, c.total)
		noErr(t, err, "Probability")
		approx(t, got, c.want, "Probability")
		if got < 0 || got > 1 {
			t.Errorf("Probability fora de [0,1]: %v", got) // critério do catálogo
		}
	}

	if _, err := Probability(1, 0); err == nil {
		t.Error("total=0 deveria falhar")
	}
	if _, err := Probability(11, 10); err == nil {
		t.Error("success>total deveria falhar")
	}
	if _, err := Probability(-1, 10); err == nil {
		t.Error("success negativo deveria falhar")
	}
}

func TestImpliedProbability(t *testing.T) {
	cases := []struct{ odd, want float64 }{
		{1.50, 0.6667}, // exemplo do catálogo
		{2.00, 0.50},   // teste do catálogo
		{1.01, 0.9901}, // borda odd mínima
		{1.25, 0.80},
		{1.60, 0.625},
		{3.00, 0.3333},
		{4.00, 0.25},
		{10.0, 0.10},
		{100.0, 0.01}, // odd alta
		{1.10, 0.9091},
	}
	for _, c := range cases {
		got, err := ImpliedProbability(c.odd)
		noErr(t, err, "ImpliedProbability")
		approx(t, got, c.want, "ImpliedProbability")
	}
	for _, bad := range []float64{1.0, 0.99, 0, -2} {
		if _, err := ImpliedProbability(bad); err == nil {
			t.Errorf("odd %v deveria falhar", bad)
		}
	}
}

func TestFairOdds(t *testing.T) {
	cases := []struct{ p, want float64 }{
		{0.80, 1.25}, // exemplo do catálogo
		{0.50, 2.00},
		{0.25, 4.00},
		{1.00, 1.01}, // critério: nunca inferior a 1.01
		{0.9999, 1.01},
		{0.10, 10.0},
		{0.6667, 1.5},
		{0.90, 1.1111},
		{0.01, 100.0},
		{0.75, 1.3333},
	}
	for _, c := range cases {
		got, err := FairOdds(c.p)
		noErr(t, err, "FairOdds")
		approx(t, got, c.want, "FairOdds")
		if got < 1.01 {
			t.Errorf("FairOdds abaixo de 1.01: %v", got)
		}
	}
	for _, bad := range []float64{0, -0.5, 1.5} {
		if _, err := FairOdds(bad); err == nil {
			t.Errorf("probabilidade %v deveria falhar", bad)
		}
	}
}

func TestBreakEven(t *testing.T) {
	cases := []struct{ odd, want float64 }{
		{1.60, 0.625}, // exemplo do catálogo: 62.50%
		{2.00, 0.50},
		{1.50, 0.6667},
		{1.01, 0.9901},
		{4.00, 0.25},
		{1.25, 0.80},
		{3.00, 0.3333},
		{10.0, 0.10},
		{1.90, 0.5263},
		{5.00, 0.20},
	}
	for _, c := range cases {
		got, err := BreakEven(c.odd)
		noErr(t, err, "BreakEven")
		approx(t, got, c.want, "BreakEven")
	}
	if _, err := BreakEven(1.0); err == nil {
		t.Error("odd 1.0 deveria falhar")
	}
}

func TestEdge(t *testing.T) {
	cases := []struct{ p, odd, want float64 }{
		{0.80, 1.50, 0.1333}, // ~exemplo do catálogo (80% − 66.67%)
		{0.50, 2.00, 0.00},   // sem edge
		{0.60, 2.00, 0.10},
		{0.40, 2.00, -0.10}, // edge negativo
		{0.625, 1.60, 0.00},
		{0.70, 1.60, 0.075},
		{1.00, 1.01, 0.0099}, // bordas
		{0.00, 2.00, -0.50},
		{0.90, 1.25, 0.10},
		{0.30, 4.00, 0.05},
	}
	for _, c := range cases {
		got, err := Edge(c.p, c.odd)
		noErr(t, err, "Edge")
		approx(t, got, c.want, "Edge")
	}
	if _, err := Edge(0.5, 1.0); err == nil {
		t.Error("odd inválida deveria falhar")
	}
	if _, err := Edge(1.5, 2.0); err == nil {
		t.Error("probabilidade > 1 deveria falhar")
	}
}
