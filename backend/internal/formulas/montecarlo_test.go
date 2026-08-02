package formulas

import (
	"math"
	"testing"
)

func TestMonteCarloReproducible(t *testing.T) {
	p := MonteCarloParams{
		WinProbability: 0.55, Odd: 2.0, Stake: 10,
		InitialBank: 1000, BetsPerRun: 100, Runs: 2000, Seed: 42,
	}
	a, err := RunMonteCarlo(p)
	noErr(t, err, "MonteCarlo run A")
	b, err := RunMonteCarlo(p)
	noErr(t, err, "MonteCarlo run B")
	// Critério do catálogo: resultado reproduzível (mesma seed = mesmo resultado).
	if a.MeanFinalBank != b.MeanFinalBank || a.RuinProbability != b.RuinProbability {
		t.Errorf("mesma seed deveria reproduzir: %v vs %v", a.MeanFinalBank, b.MeanFinalBank)
	}
	for q, v := range a.Percentiles {
		if b.Percentiles[q] != v {
			t.Errorf("percentil %d difere entre execuções", q)
		}
	}
}

func TestMonteCarloStatistics(t *testing.T) {
	// Edge positivo (p=0.55 na odd 2.0): a banca média final deve superar a inicial.
	pos, err := RunMonteCarlo(MonteCarloParams{
		WinProbability: 0.55, Odd: 2.0, Stake: 10,
		InitialBank: 1000, BetsPerRun: 200, Runs: 5000, Seed: 7,
	})
	noErr(t, err, "MonteCarlo positivo")
	// esperado teórico: 1000 + 200 apostas × EV(10, 2.0, 0.55)=1 → ~1200
	if pos.MeanFinalBank < 1100 || pos.MeanFinalBank > 1300 {
		t.Errorf("banca média com edge positivo fora do esperado: %v", pos.MeanFinalBank)
	}
	// Edge negativo: banca média final deve ficar abaixo da inicial.
	neg, err := RunMonteCarlo(MonteCarloParams{
		WinProbability: 0.45, Odd: 2.0, Stake: 10,
		InitialBank: 1000, BetsPerRun: 200, Runs: 5000, Seed: 7,
	})
	noErr(t, err, "MonteCarlo negativo")
	if neg.MeanFinalBank >= 1000 {
		t.Errorf("banca média com edge negativo deveria cair: %v", neg.MeanFinalBank)
	}
	// Percentis ordenados.
	if !(pos.Percentiles[5] <= pos.Percentiles[25] &&
		pos.Percentiles[25] <= pos.Percentiles[50] &&
		pos.Percentiles[50] <= pos.Percentiles[75] &&
		pos.Percentiles[75] <= pos.Percentiles[95]) {
		t.Error("percentis fora de ordem")
	}
	// Drawdown médio dentro de [0,1]; ruína dentro de [0,1].
	for _, v := range []float64{pos.MeanMaxDrawdown, pos.RuinProbability} {
		if v < 0 || v > 1 || math.IsNaN(v) {
			t.Errorf("métrica fora de [0,1]: %v", v)
		}
	}
}

func TestMonteCarloCertainties(t *testing.T) {
	// p=1: nunca perde — banca final determinística e sem drawdown.
	sure, err := RunMonteCarlo(MonteCarloParams{
		WinProbability: 1, Odd: 1.5, Stake: 10,
		InitialBank: 100, BetsPerRun: 10, Runs: 100, Seed: 1,
	})
	noErr(t, err, "MonteCarlo p=1")
	approx(t, sure.MeanFinalBank, 100+10*10*0.5, "banca com p=1")
	approx(t, sure.MeanMaxDrawdown, 0, "drawdown com p=1")
	approx(t, sure.RuinProbability, 0, "ruína com p=1")

	// p=0: perde tudo até não cobrir a stake — ruína garantida.
	doom, err := RunMonteCarlo(MonteCarloParams{
		WinProbability: 0, Odd: 2.0, Stake: 10,
		InitialBank: 50, BetsPerRun: 10, Runs: 100, Seed: 1,
	})
	noErr(t, err, "MonteCarlo p=0")
	approx(t, doom.RuinProbability, 1, "ruína com p=0")
	approx(t, doom.MeanFinalBank, 0, "banca com p=0")
}

func TestMonteCarloValidation(t *testing.T) {
	base := MonteCarloParams{WinProbability: 0.5, Odd: 2, Stake: 10, InitialBank: 100, BetsPerRun: 10, Runs: 10, Seed: 1}
	bad := []MonteCarloParams{
		{WinProbability: 0.5, Odd: 1.0, Stake: 10, InitialBank: 100, BetsPerRun: 10, Runs: 10},
		{WinProbability: 1.5, Odd: 2, Stake: 10, InitialBank: 100, BetsPerRun: 10, Runs: 10},
		{WinProbability: 0.5, Odd: 2, Stake: 0, InitialBank: 100, BetsPerRun: 10, Runs: 10},
		{WinProbability: 0.5, Odd: 2, Stake: 10, InitialBank: 0, BetsPerRun: 10, Runs: 10},
		{WinProbability: 0.5, Odd: 2, Stake: 10, InitialBank: 100, BetsPerRun: 0, Runs: 10},
		{WinProbability: 0.5, Odd: 2, Stake: 10, InitialBank: 100, BetsPerRun: 10, Runs: 0},
	}
	if _, err := RunMonteCarlo(base); err != nil {
		t.Fatalf("base válida falhou: %v", err)
	}
	for i, p := range bad {
		if _, err := RunMonteCarlo(p); err == nil {
			t.Errorf("caso inválido %d deveria falhar", i)
		}
	}
}
