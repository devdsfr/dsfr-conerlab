package usecase

import "testing"

// REV-P1 — Dashboard / Visão Geral.
//
// Os casos abaixo vêm do que foi medido em produção em 15/09/2026, não de
// hipótese:
//
//	GET /dashboard?team_id=455&league_id=8&season_id=33&limit=20
//	  → sample_size = 6, period = "Últimos 20 jogos"      ← contradição
//
//	GET /dashboard?team_id=442&league_id=8&season_id=33&limit=20
//	  → sample_size = 0, mean = 0, max = 0, freq = 0%      ← ausência virando zero
//
// A primeira linha da tela dizia, ao mesmo tempo, "Últimos 20 jogos" e "amostra
// de 6 jogos". Anunciar a janela pedida como se fosse a evidência disponível é
// afirmar mais do que os dados sustentam.

func TestPeriodoDescreveAAmostraRealENaoAPedida(t *testing.T) {
	// Caso Celta Vigo / La Liga 2026, reproduzido em produção.
	got := describePeriod(6, 20)

	if got == "Últimos 20 jogos" {
		t.Fatal("period voltou a anunciar a janela pedida como se fosse a amostra — " +
			"seis jogos sendo apresentados como vinte")
	}
	if want := "Últimos 6 jogos (de 20 pedidos)"; got != want {
		t.Errorf("describePeriod(6, 20) = %q, esperado %q", got, want)
	}
}

func TestPeriodoSemPartidasNaoDizUltimosNJogos(t *testing.T) {
	got := describePeriod(0, 20)

	if want := "Nenhuma partida encontrada"; got != want {
		t.Errorf("describePeriod(0, 20) = %q, esperado %q — com zero partidas não "+
			"existe \"últimos N jogos\"", got, want)
	}
}

func TestPeriodoComAmostraCompletaNaoPoluiComRessalva(t *testing.T) {
	// Quando a amostra cobre a janela pedida, a frase volta a ser simples: a
	// ressalva só existe para o caso em que ela é necessária.
	if want, got := "Últimos 20 jogos", describePeriod(20, 20); got != want {
		t.Errorf("describePeriod(20, 20) = %q, esperado %q", got, want)
	}
}

// Amostra maior que o pedido não deveria acontecer (o repositório aplica LIMIT),
// mas se acontecer a frase precisa continuar coerente com o que foi analisado.
func TestPeriodoNuncaAnunciaMenosDoQueAnalisou(t *testing.T) {
	if want, got := "Últimos 25 jogos", describePeriod(25, 20); got != want {
		t.Errorf("describePeriod(25, 20) = %q, esperado %q", got, want)
	}
}

// Amostra vazia precisa continuar produzindo resumo vazio — e não um resumo de
// zeros que pareça observação. Summarize já fazia isso; o teste existe para que
// continue fazendo, já que a interface agora depende de Count == 0 para decidir
// entre "sem dados" e "zero real".
func TestResumoDeAmostraVaziaNaoInventaObservacao(t *testing.T) {
	s := Summarize(nil)

	if s.Count != 0 {
		t.Errorf("Count = %d, esperado 0", s.Count)
	}
	if s.Mode == nil {
		t.Error("Mode = nil: o frontend chama .join() direto e quebraria com null")
	}
	if len(s.Mode) != 0 {
		t.Errorf("Mode = %v, esperado vazio", s.Mode)
	}
}

// Zero REAL continua sendo zero: um time que de fato não fez escanteio nenhum em
// três jogos tem média 0 com amostra 3. A correção não pode transformar isso em
// "sem dados".
func TestZeroObservadoContinuaSendoZero(t *testing.T) {
	s := Summarize([]int{0, 0, 0})

	if s.Count != 3 {
		t.Fatalf("Count = %d, esperado 3 — três jogos foram observados", s.Count)
	}
	if s.Mean != 0 {
		t.Errorf("Mean = %v, esperado 0", s.Mean)
	}

	freqs := FrequencyAboveThresholds([]int{0, 0, 0}, []int{4})
	if freqs[0].Total != 3 {
		t.Errorf("Total = %d, esperado 3: o denominador é a amostra observada", freqs[0].Total)
	}
	if freqs[0].Count != 0 || freqs[0].Pct != 0 {
		t.Errorf("acima de 4 em [0,0,0] deveria ser 0 ocorrências / 0%%, veio %d / %v",
			freqs[0].Count, freqs[0].Pct)
	}
}

// E o oposto: amostra vazia tem denominador zero, não três.
func TestFrequenciaSemAmostraNaoInventaDenominador(t *testing.T) {
	freqs := FrequencyAboveThresholds(nil, []int{4, 5})

	for _, f := range freqs {
		if f.Total != 0 {
			t.Errorf("threshold %d: Total = %d, esperado 0", f.Threshold, f.Total)
		}
		if f.Pct != 0 {
			t.Errorf("threshold %d: Pct = %v, esperado 0", f.Threshold, f.Pct)
		}
	}
}
