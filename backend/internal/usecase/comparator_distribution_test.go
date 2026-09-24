package usecase

import "testing"

// REV-P2 — distribuição observada.
//
// A distribuição é montada sobre as MESMAS observações que alimentam média e
// frequências: a fatia que só contém partidas onde a métrica existe. Daí as duas
// propriedades que estes testes travam — ausência não vira zero, zero real
// entra — e o denominador ser metric_sample_size, não sample_size.

// Caso literal do enunciado: [4, 6, 6, 8] -> 4:1/4, 6:2/4, 8:1/4.
func TestDistribuicaoDeAmostraConhecida(t *testing.T) {
	d := buildDistribution([]int{4, 6, 6, 8})

	if len(d) != 3 {
		t.Fatalf("baldes = %d, esperado 3 (valores distintos 4, 6, 8)", len(d))
	}

	esperado := []DistributionBucket{
		{Value: 4, Count: 1, Sample: 4, Percentage: 25},
		{Value: 6, Count: 2, Sample: 4, Percentage: 50},
		{Value: 8, Count: 1, Sample: 4, Percentage: 25},
	}
	for i, e := range esperado {
		if d[i] != e {
			t.Errorf("balde %d = %+v, esperado %+v", i, d[i], e)
		}
	}
}

// Ordem crescente: o eixo precisa ser lido como escala, não como ordem de
// aparição no jogo.
func TestDistribuicaoSaiOrdenadaPorValor(t *testing.T) {
	d := buildDistribution([]int{9, 3, 7, 3})

	anterior := -1
	for _, b := range d {
		if b.Value <= anterior {
			t.Fatalf("valores fora de ordem: %d veio depois de %d", b.Value, anterior)
		}
		anterior = b.Value
	}
}

// ZERO REAL é observação: entra na distribuição como qualquer outro valor.
func TestZeroObservadoEntraNaDistribuicao(t *testing.T) {
	d := buildDistribution([]int{0, 0, 3})

	if len(d) != 2 {
		t.Fatalf("baldes = %d, esperado 2 (valores 0 e 3)", len(d))
	}
	if d[0].Value != 0 || d[0].Count != 2 {
		t.Errorf("balde do zero = %+v, esperado value=0 count=2", d[0])
	}
	if d[0].Percentage < 66.66 || d[0].Percentage > 66.67 {
		t.Errorf("percentual do zero = %v, esperado ~66.67", d[0].Percentage)
	}
}

// AUSÊNCIA não é observação: a partida sem a métrica nem chega ao buildDistribution,
// então não pode aparecer como um balde de valor 0.
//
// O teste percorre o caminho real — vistas -> metricValue -> valores -> distribuição —
// em vez de chamar buildDistribution direto, porque é justamente esse caminho que
// precisa descartar a partida.
func TestMetricaAusenteNaoViraBaldeZero(t *testing.T) {
	vistas := []struct {
		temDado bool
		valor   int
	}{
		{true, 2}, {false, 0}, {true, 2}, {false, 0}, {true, 5},
	}

	valores := []int{}
	for _, v := range vistas {
		var val *int
		if v.temDado {
			x := v.valor
			val = &x
		}
		if val != nil {
			valores = append(valores, *val)
		}
	}

	d := buildDistribution(valores)

	for _, b := range d {
		if b.Value == 0 {
			t.Fatalf("partida sem a métrica virou balde de valor 0 (count=%d) — "+
				"ausência foi contada como observação", b.Count)
		}
	}
	if len(d) != 2 {
		t.Fatalf("baldes = %d, esperado 2 (valores 2 e 5)", len(d))
	}
	// O denominador tem que ser 3 (metric_sample_size), NÃO 5 (sample_size).
	for _, b := range d {
		if b.Sample != 3 {
			t.Errorf("sample = %d, esperado 3 — o denominador é a amostra COM a métrica, "+
				"não a amostra de partidas", b.Sample)
		}
	}
}

// O denominador da distribuição bate com o que Summarize contou: se divergirem,
// a tela mostraria percentuais que não fecham com a média ao lado.
func TestDenominadorDaDistribuicaoBateComOResumo(t *testing.T) {
	valores := []int{4, 6, 6, 8, 0}

	resumo := Summarize(valores)
	d := buildDistribution(valores)

	for _, b := range d {
		if b.Sample != resumo.Count {
			t.Errorf("sample do balde %d = %d, mas Summarize contou %d",
				b.Value, b.Sample, resumo.Count)
		}
	}
}

// Amostra vazia devolve lista vazia — não um balde de zero fingindo observação.
func TestDistribuicaoSemAmostraNaoInventaBalde(t *testing.T) {
	if d := buildDistribution(nil); len(d) != 0 {
		t.Errorf("baldes = %d, esperado 0", len(d))
	}
}

// Perspectivas diferentes produzem distribuições diferentes e coerentes: com a
// equipe fazendo 6 e o adversário 4, produzido tem o balde 6, concedido o 4 e o
// total o 10.
func TestDistribuicoesPorPerspectivaSaoCoerentes(t *testing.T) {
	v := vista(true, 6, 4, 0, 0)

	casos := map[Perspective]int{
		PerspectiveProduzido: 6,
		PerspectiveConcedido: 4,
		PerspectiveTotal:     10,
	}
	for p, esperado := range casos {
		val := metricValue(v, MetricCorners, p)
		if val == nil {
			t.Fatalf("perspectiva %s devolveu nil", p)
		}
		d := buildDistribution([]int{*val})
		if len(d) != 1 || d[0].Value != esperado {
			t.Errorf("perspectiva %s: distribuição = %+v, esperado balde de valor %d", p, d, esperado)
		}
		if d[0].Percentage != 100 {
			t.Errorf("perspectiva %s: única observação deveria ser 100%%, veio %v", p, d[0].Percentage)
		}
	}
}
