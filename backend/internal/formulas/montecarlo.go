package formulas

import (
	"math/rand"
	"sort"
)

// MonteCarloParams parametriza a simulação (Catálogo 19).
type MonteCarloParams struct {
	WinProbability float64 // fração [0,1]
	Odd            float64 // odd decimal > 1
	Stake          float64 // stake fixa por aposta
	InitialBank    float64 // banca inicial
	BetsPerRun     int     // apostas por simulação
	Runs           int     // nº de simulações (catálogo: 10000+)
	Seed           int64   // semente — resultado reproduzível (critério do catálogo)
}

// MonteCarloResult resume a distribuição final das simulações.
type MonteCarloResult struct {
	MeanFinalBank   float64 // média da banca final
	Percentiles     map[int]float64
	MeanMaxDrawdown float64 // média (fração) do drawdown máximo por run
	RuinProbability float64 // fração de runs que zeraram a banca
}

// RunMonteCarlo (Catálogo 19) — executa Runs simulações de BetsPerRun apostas
// com stake fixa e retorna média, percentis (5/25/50/75/95), drawdown médio e
// probabilidade de ruína. Mesma Seed ⇒ mesmo resultado (reproduzível).
func RunMonteCarlo(p MonteCarloParams) (MonteCarloResult, error) {
	var zero MonteCarloResult
	if p.Odd <= 1 {
		return zero, ErrInvalidOdd
	}
	if p.WinProbability < 0 || p.WinProbability > 1 {
		return zero, ErrInvalidProbability
	}
	if p.Stake <= 0 || p.InitialBank <= 0 || p.BetsPerRun <= 0 || p.Runs <= 0 {
		return zero, ErrInvalidInput
	}

	rng := rand.New(rand.NewSource(p.Seed))
	finals := make([]float64, 0, p.Runs)
	sumDD := 0.0
	ruins := 0

	for r := 0; r < p.Runs; r++ {
		bank := p.InitialBank
		peak := bank
		maxDD := 0.0
		for b := 0; b < p.BetsPerRun; b++ {
			if bank < p.Stake {
				// banca insuficiente para a próxima stake — ruína prática
				break
			}
			if rng.Float64() < p.WinProbability {
				bank += p.Stake * (p.Odd - 1)
			} else {
				bank -= p.Stake
			}
			if bank > peak {
				peak = bank
			}
			if peak > 0 {
				if dd := (peak - bank) / peak; dd > maxDD {
					maxDD = dd
				}
			}
		}
		if bank < p.Stake {
			ruins++
		}
		finals = append(finals, bank)
		sumDD += maxDD
	}

	sort.Float64s(finals)
	percentile := func(q int) float64 {
		if len(finals) == 1 {
			return finals[0]
		}
		idx := float64(q) / 100 * float64(len(finals)-1)
		lo := int(idx)
		hi := lo + 1
		if hi >= len(finals) {
			return finals[lo]
		}
		frac := idx - float64(lo)
		return finals[lo]*(1-frac) + finals[hi]*frac
	}

	mean := 0.0
	for _, f := range finals {
		mean += f
	}
	mean /= float64(len(finals))

	return MonteCarloResult{
		MeanFinalBank: mean,
		Percentiles: map[int]float64{
			5:  percentile(5),
			25: percentile(25),
			50: percentile(50),
			75: percentile(75),
			95: percentile(95),
		},
		MeanMaxDrawdown: sumDD / float64(p.Runs),
		RuinProbability: float64(ruins) / float64(p.Runs),
	}, nil
}
