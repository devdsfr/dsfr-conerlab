package usecase

import (
	"context"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P3 — validação local dos números OBSERVADOS em produção na Fase A.
//
// Lá o Simulador devolvia os pares 200/100 e 138/69: exatamente o dobro. Este
// teste reconstrói a forma do caso (100 partidas, 69 acertando a linha) e exige
// os números simples. Se a dupla contagem voltar, ele reproduz 200 e 138 e falha.
func TestFaseA_ParesDobradosNaoOcorremMais(t *testing.T) {
	var ms []domain.Match
	for i := 1; i <= 69; i++ {
		ms = append(ms, partida(int64(i), 6, 5)) // 11 escanteios — acerta a linha 8
	}
	for i := 70; i <= 100; i++ {
		ms = append(ms, partida(int64(i), 2, 1)) // 3 escanteios — erra
	}

	u := newFilterUsecaseWith(ms)
	res, err := u.RunBacktest(context.Background(), 1, nil,
		FilterCriteria{Metric: "corners", CornersThreshold: 8, Stake: 10, AllowMissingOdds: true}, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	if res.MatchCount == 200 {
		t.Fatal("MatchCount = 200 para 100 partidas — a dupla contagem da Fase A voltou")
	}
	if res.Hits == 138 {
		t.Fatal("Hits = 138 para 69 partidas acertando — a dupla contagem da Fase A voltou")
	}
	if res.MatchCount != 100 {
		t.Errorf("MatchCount = %d, esperado 100", res.MatchCount)
	}
	if res.Hits != 69 {
		t.Errorf("Hits = %d, esperado 69", res.Hits)
	}
	if res.Misses != 31 {
		t.Errorf("Misses = %d, esperado 31", res.Misses)
	}
}
