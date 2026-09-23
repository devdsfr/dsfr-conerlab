package usecase

import "testing"

// REV-P2 — Comparador, fatia de integridade.
//
// Caso medido em produção em 23/09/2026 (Celta Vigo · La Liga · limit 20), com o
// Comparador ainda sem `season_id`:
//
//	Comparador (sem temporada) : 20 jogos, média do total = 7.9
//	Dashboard · temporada 2026 :  7 jogos, média = 8.0
//	Dashboard · temporada 2025 : 20 jogos, média = 8.15
//
// Os 20 jogos eram 7 de 2026 + 13 de 2025. A média 7.9 não corresponde a nenhuma
// das temporadas: é uma mistura apresentada como recorte único.

// Períodos por lado: dois times com amostras diferentes não podem aparecer sob a
// mesma frase. Antes havia um `period` único com a janela PEDIDA.
func TestCadaLadoDescreveASuaPropriaAmostra(t *testing.T) {
	ladoCheio := describePeriod(20, 20)
	ladoCurto := describePeriod(13, 20)

	if ladoCheio == ladoCurto {
		t.Fatalf("os dois lados receberam a mesma descrição (%q) apesar de amostras "+
			"diferentes — é assim que 13 jogos passam por 20", ladoCheio)
	}
	if ladoCurto != "Últimos 13 jogos (de 20 pedidos)" {
		t.Errorf("lado curto = %q", ladoCurto)
	}
}

func TestLadoSemPartidasNaoAnunciaJanela(t *testing.T) {
	if got, want := describePeriod(0, 20), "Nenhuma partida encontrada"; got != want {
		t.Errorf("describePeriod(0,20) = %q, esperado %q", got, want)
	}
}

// --- Conferência aritmética com resultado conhecido -------------------------
//
// O enunciado do REV-P2 pede casos pequenos com resultado calculável à mão, para
// pegar erro de cálculo sem depender da base real.

func TestMediaDeAmostraConhecida(t *testing.T) {
	// 4, 6, 8 -> média 6
	s := Summarize([]int{4, 6, 8})
	if s.Mean != 6 {
		t.Errorf("média = %v, esperado 6", s.Mean)
	}
	if s.Count != 3 {
		t.Errorf("count = %d, esperado 3", s.Count)
	}
	if s.Total != 18 {
		t.Errorf("total = %d, esperado 18", s.Total)
	}
}

func TestFrequenciaDeAmostraConhecida(t *testing.T) {
	// 4, 6, 8 — "acima de 5" conta valores estritamente maiores: 6 e 8 -> 2 de 3.
	f := FrequencyAboveThresholds([]int{4, 6, 8}, []int{5})[0]

	if f.Count != 2 {
		t.Errorf("ocorrências = %d, esperado 2", f.Count)
	}
	if f.Total != 3 {
		t.Errorf("denominador = %d, esperado 3 — a amostra observada", f.Total)
	}
	if f.Pct < 66.66 || f.Pct > 66.67 {
		t.Errorf("percentual = %v, esperado ~66.67", f.Pct)
	}
}

// Produzido, concedido e total da partida são grandezas distintas. O teste fixa a
// aritmética do exemplo do enunciado: equipe faz 6, adversário faz 4.
func TestProduzidoConcedidoETotalNaoSeConfundem(t *testing.T) {
	produzidos := []int{6}
	concedidos := []int{4}
	totalPartida := []int{produzidos[0] + concedidos[0]}

	if m := Summarize(produzidos).Mean; m != 6 {
		t.Errorf("produzidos = %v, esperado 6", m)
	}
	if m := Summarize(concedidos).Mean; m != 4 {
		t.Errorf("concedidos = %v, esperado 4", m)
	}
	if m := Summarize(totalPartida).Mean; m != 10 {
		t.Errorf("total da partida = %v, esperado 10", m)
	}
}

// Zero observado continua zero: uma equipe que não conquistou escanteio em três
// jogos tem média 0 com amostra 3 — diferente de amostra 0.
func TestZeroObservadoNoComparador(t *testing.T) {
	observado := Summarize([]int{0, 0, 0})
	ausente := Summarize(nil)

	if observado.Count != 3 || observado.Mean != 0 {
		t.Errorf("zero observado: count=%d mean=%v, esperado 3 e 0", observado.Count, observado.Mean)
	}
	if ausente.Count != 0 {
		t.Errorf("ausência: count=%d, esperado 0 — é o que distingue de zero real", ausente.Count)
	}
}
