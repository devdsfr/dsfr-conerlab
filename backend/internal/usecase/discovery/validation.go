package discovery

import (
	"sort"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

// Validação fora da amostra e controle de testes múltiplos — correção do AUD-003.
//
// O DEFEITO ORIGINAL. O ciclo testava ~5.940 combinações contra o mesmo
// histórico, aplicava limiares fixos e publicava as melhores. Esse desenho não
// tem como distinguir vantagem real de ruído: testar muitas hipóteses contra um
// único conjunto de dados e ficar com as vencedoras é o procedimento que
// FABRICA falsos positivos. Com 5.940 testes a 5%, cerca de 297 combinações
// "aprovam" mesmo que nenhuma tenha vantagem alguma.
//
// A CORREÇÃO tem duas partes independentes, e as duas precisam passar:
//
//  1. SIGNIFICÂNCIA COM CORREÇÃO PARA MÚLTIPLOS TESTES. Cada combinação vira um
//     teste binomial unilateral contra a probabilidade que a própria odd embutia
//     (1/odd). Os p-valores de TODAS as combinações testadas entram numa
//     correção de FDR (Benjamini–Yekutieli, conservadora sob dependência
//     arbitrária). O limiar deixa de ser fixo e passa a depender de quantos
//     testes foram feitos — testar mais passa a custar mais caro, que é
//     exatamente o incentivo correto.
//
//  2. VALIDAÇÃO FORA DA AMOSTRA. O histórico é cortado por data em janela de
//     DESCOBERTA (a mais antiga) e janela de VALIDAÇÃO (a mais recente). A
//     mineração só enxerga a de descoberta. Quem sobrevive é reexecutado na de
//     validação, que nunca foi vista, e só é publicado se continuar de pé lá.
//
// A ordem temporal importa: descobrir no passado e validar no futuro é o único
// arranjo que imita como a estratégia seria usada de verdade. O inverso seria
// look-ahead.

// window é uma janela temporal meio-aberta [From, To). To nil = sem limite.
type window struct {
	From time.Time
	To   *time.Time
}

// split divide o histórico de uma liga em janela de descoberta e janela de
// validação, cortando por DATA.
//
// O corte é feito pelo quantil da CONTAGEM de partidas, não pelo calendário:
// ligas têm pausas e temporadas de tamanhos diferentes, e cortar "na metade do
// tempo" produziria amostras desbalanceadas. Cortar no jogo de índice
// trainFraction·n mantém a proporção pedida.
//
// Todas as partidas da data de corte vão para a VALIDAÇÃO. Sem essa regra, uma
// rodada disputada no mesmo dia ficaria dividida entre as duas janelas e a
// mineração teria visto parte do que depois é usado para validar.
//
// Devolve ok = false quando não há partidas suficientes para um corte com
// significado — o chamador deve tratar isso como "esta liga não é validável", e
// nunca como "publique sem validar".
func splitHistory(matches []domain.Match, trainFraction float64) (train, holdout window, ok bool) {
	if len(matches) < 2 || trainFraction <= 0 || trainFraction >= 1 {
		return window{}, window{}, false
	}

	dates := make([]time.Time, 0, len(matches))
	for _, m := range matches {
		dates = append(dates, m.MatchDate)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	idx := int(float64(len(dates)) * trainFraction)
	if idx <= 0 || idx >= len(dates) {
		return window{}, window{}, false
	}
	cut := dates[idx]

	// Recua até o primeiro jogo da data de corte, para que a data inteira caia
	// na validação.
	for idx > 0 && dates[idx-1].Equal(cut) {
		idx--
	}
	if idx == 0 {
		// Todas as partidas são do mesmo dia (ou o corte caiu no primeiro dia):
		// não há como separar passado de futuro.
		return window{}, window{}, false
	}
	cut = dates[idx]

	first := dates[0]
	return window{From: first, To: &cut},
		window{From: cut, To: nil},
		true
}

// breakEvenProbability devolve a probabilidade de acerto que as odds do próprio
// backtest já embutiam — a média de 1/odd sobre as entradas (Catálogo 02/04).
//
// É a hipótese nula do teste de significância: se a casa precifica corretamente,
// é essa a frequência de acerto esperada no longo prazo. Acertar mais do que
// isso, de forma que o acaso não explique, é a definição operacional de
// "vantagem" aqui.
//
// Devolve ok = false se alguma entrada não tiver odd utilizável — sem odd não há
// hipótese nula, e sem hipótese nula não existe p-valor.
func breakEvenProbability(r *usecase.BacktestResult) (float64, bool) {
	if r == nil || len(r.Entries) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, e := range r.Entries {
		if e.Odd <= 1 {
			return 0, false
		}
		sum += 1 / e.Odd
	}
	return sum / float64(len(r.Entries)), true
}

// pValue devolve o p-valor unilateral do backtest: a probabilidade de observar
// pelo menos os acertos observados, se a estratégia não tivesse vantagem
// nenhuma sobre a odd oferecida.
//
// p pequeno = difícil de explicar por acaso. Devolve ok = false quando o teste
// não é aplicável (sem entradas, sem odd válida).
func pValue(r *usecase.BacktestResult) (float64, bool) {
	p0, ok := breakEvenProbability(r)
	if !ok {
		return 0, false
	}
	p, err := formulas.BinomialAtLeast(r.MatchCount, r.Hits, p0)
	if err != nil {
		return 0, false
	}
	return p, true
}

// holdoutVerdict é o resultado da checagem fora da amostra.
type holdoutVerdict struct {
	Passed  bool
	Reason  rejection // preenchido só quando Passed == false
	Games   int
	HitRate float64
	ROI     float64
	PValue  float64
}

// checkHoldout aplica os critérios da janela de validação a um candidato que já
// passou por tudo na janela de descoberta.
//
// Os critérios NÃO são os mesmos da descoberta, e isso é deliberado: a janela de
// validação é menor por construção, então exigir os mesmos 100 jogos do doc 08
// reprovaria tudo por tamanho de amostra e a validação viraria teatro. O que se
// exige aqui é CONSISTÊNCIA:
//
//   - amostra mínima própria — abaixo dela o resultado não é conclusivo, e
//     "inconclusivo" reprova (não publicar é o padrão seguro);
//   - lucro positivo — uma estratégia que perde dinheiro no período que não foi
//     usado para descobri-la não é uma estratégia;
//   - significância própria — a mesma pergunta do teste binomial, agora num
//     conjunto que a mineração nunca viu.
//
// Não há correção de múltiplos testes aqui: nesta altura o conjunto de
// candidatos já é pequeno e foi FIXADO antes de olhar a validação, que é
// justamente a condição que a correção existe para restaurar. Ainda assim o
// número de candidatos testados é registrado em LeagueResult.
func checkHoldout(r *usecase.BacktestResult, minGames int, alpha float64) holdoutVerdict {
	v := holdoutVerdict{}
	if r == nil {
		v.Reason = rejectHoldoutSample
		return v
	}
	v.Games = r.MatchCount
	v.HitRate = r.HitRate
	v.ROI = r.ROI

	if r.MatchCount < minGames {
		v.Reason = rejectHoldoutSample
		return v
	}
	if r.Profit <= 0 {
		v.Reason = rejectHoldoutProfit
		return v
	}
	p, ok := pValue(r)
	if !ok {
		// Sem odd utilizável na validação não há como testar. Reprova.
		v.Reason = rejectHoldoutSignificance
		return v
	}
	v.PValue = p
	if p > alpha {
		v.Reason = rejectHoldoutSignificance
		return v
	}

	v.Passed = true
	return v
}
