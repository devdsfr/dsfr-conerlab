package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
)

// Espaço de busca do Discovery Engine (doc 08, seção "Variáveis"). Cada eixo
// abaixo é uma das variáveis listadas no documento que o motor de backtest atual
// já sabe aplicar. As demais variáveis do doc (dias de descanso, árbitro, clima)
// dependem de dados que ainda não são importados e ficam para uma próxima versão
// — o gerador é aditivo: acrescentar um eixo é acrescentar um slice aqui.
var (
	// Linhas de escanteios totais. O motor trata `threshold` como "total > N",
	// ou seja, a linha de mercado N.5 — e são exatamente as linhas para as quais
	// existem odds históricas armazenadas (corner_odds, 4.5 a 10.5).
	cornerLines = []int{6, 7, 8, 9, 10}

	// Mando de campo: "" = casa e fora.
	homeAwayOptions = []string{"", "home", "away"}

	// Janela de forma recente: 0 = histórico completo do período.
	windowOptions = []int{0, 10, 20}

	// AUD-004: o eixo de força do adversário FOI REMOVIDO do espaço de busca.
	//
	// Ele era `[]string{"", "G6", "G12", "Z4"}` e quadruplicava a grade. Metade
	// dessas combinações (G6 e Z4) nunca podia render nada: zero equipes tinham
	// esses valores. A de G12 filtrava por uma constante gravada no código de
	// sincronização, não por classificação. E as combinações "" repetiam a grade
	// inteira sem filtro nenhum.
	//
	// Resultado prático: 540 combinações por liga viraram 135. As 405 que saíram
	// não eram hipóteses — eram a mesma hipótese contada quatro vezes, três delas
	// vazias. Isso também alivia a correção de múltiplos testes do AUD-003, que
	// estava sendo penalizada por um espaço de busca inflado artificialmente.
	//
	// Para reintroduzir o eixo é preciso primeiro ter classificação por
	// temporada, apurada com os jogos anteriores à data de cada partida — sem
	// isso o filtro carrega look-ahead por construção.

	// Teto de odd sobre a odd de MERCADO. Todos os valores são > 0 porque o eixo
	// é uma restrição de elegibilidade: com RequireRealOdds (AUD-001) só entram
	// jogos com odd real registrada, e o teto recorta entre eles. (O comentário
	// antigo falava em "odd 1.0 assumida" — esse fallback não existe mais desde
	// o REV-P3: ausência de odd é nil, nunca 1,00.)
	maxOddsOptions = []float64{1.70, 2.20, 3.50}

	// Mercados de RESULTADO. Não têm linha nem teto de odd: o desfecho da partida
	// já é a resposta, e não há odd registrada para aplicar um teto.
	//
	// LEIA ISTO ANTES DE ESPERAR RESULTADO DESTE EIXO. Nenhuma rota do sistema
	// grava matches.result_odds hoje. Com RequireRealOdds as partidas sem odd de
	// mercado saem do backtest, e estas combinações são descartadas com o motivo
	// "sem_odd_real" (REV-P4, B3 — antes caíam em "amostra_insuficiente", e este
	// comentário dizia, errado, "sem_pvalor_calculavel"). É o comportamento
	// correto: publicar sem poder testar é o defeito que a auditoria inteira
	// existe para impedir.
	//
	// O eixo fica aqui porque o dia em que a coleta de odds 1X2 entrar, ele passa
	// a funcionar sem mudança de código. Enquanto isso, o custo é só CPU: as
	// combinações são rejeitadas na elegibilidade ESTRUTURAL, antes da correção de
	// múltiplos testes (ver mine), então não entram no m do FDR — o que é
	// legítimo, porque a exclusão não depende do resultado.
	resultMetrics = []string{usecase.MetricWin, usecase.MetricDraw, usecase.MetricWinOrDraw}
)

// combo é uma combinação candidata do espaço de busca.
type combo struct {
	teamID   *int64
	teamName string
	line     int
	homeAway string
	window   int
	maxOdds  float64

	// metric vazio = escanteios (o padrão histórico do motor). Preenchido nos
	// mercados de resultado, onde line e maxOdds não se aplicam.
	metric string
}

// isResult indica se a combinação é de mercado de resultado.
func (c combo) isResult() bool { return c.metric != "" && c.metric != "corners" }

// effectiveMetric devolve a métrica a enviar ao motor. Vazio = escanteios, que é
// o padrão histórico e o que o restante do sistema espera quando o campo não vem.
func (c combo) effectiveMetric() string {
	if c.metric == "" {
		return "corners"
	}
	return c.metric
}

// resultLabel é o nome em português do mercado de resultado, usado no nome da
// estratégia publicada.
func resultLabel(metric string) string {
	switch metric {
	case usecase.MetricDraw:
		return "Empate"
	case usecase.MetricWinOrDraw:
		return "Não perde"
	default:
		return "Vitória"
	}
}

// generateCombos monta o espaço de busca de uma liga.
//
// A varredura por equipe usa uma grade REDUZIDA (sem janela e sem tier) porque a
// amostra de uma única equipe é uma ordem de grandeza menor que a da liga: cruzar
// todos os eixos ali produziria milhares de combinações que a trava
// anti-overfitting descartaria de qualquer forma.
func generateCombos(teams []domain.Team, includeTeams bool) []combo {
	var out []combo

	for _, line := range cornerLines {
		for _, ha := range homeAwayOptions {
			for _, window := range windowOptions {
				for _, odds := range maxOddsOptions {
					out = append(out, combo{
						line: line, homeAway: ha, window: window, maxOdds: odds,
					})
				}
			}
		}
	}

	// Mercados de resultado: só mando e janela fazem sentido como eixo.
	for _, metric := range resultMetrics {
		for _, ha := range homeAwayOptions {
			for _, window := range windowOptions {
				out = append(out, combo{metric: metric, homeAway: ha, window: window})
			}
		}
	}

	if !includeTeams {
		return out
	}

	for _, t := range teams {
		teamID := t.ID
		for _, line := range cornerLines {
			for _, ha := range homeAwayOptions {
				for _, odds := range maxOddsOptions {
					out = append(out, combo{
						teamID: &teamID, teamName: t.Name,
						line: line, homeAway: ha, maxOdds: odds,
					})
				}
			}
		}
	}
	return out
}

// definition converte a combinação no JSONB persistido em strategies.definition —
// exatamente o mesmo formato do Simulador de Filtros, para que o usuário possa
// abrir uma estratégia descoberta na tela e reexecutá-la sem conversão nenhuma.
func (c combo) definition(leagueID int64, seasonIDs []int64) (string, error) {
	d := strategyengine.Definition{
		LeagueID:         leagueID,
		SeasonIDs:        seasonIDs,
		TeamID:           c.teamID,
		LastNGames:       c.window,
		HomeAway:         c.homeAway,
		CornersThreshold: c.line,
		MaxOdds:          c.maxOdds,
		Metric:           c.effectiveMetric(),
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("serializar definition: %w", err)
	}
	return string(raw), nil
}

// strategyNameMaxLen espelha strategies.name VARCHAR(120) (migration 011).
const strategyNameMaxLen = 120

// name gera o identificador determinístico da estratégia descoberta. É a chave de
// idempotência do ciclo (índice único parcial da migration 012): a MESMA
// combinação precisa produzir SEMPRE o mesmo nome, e combinações diferentes
// precisam produzir nomes diferentes — por isso todos os eixos do espaço de busca
// aparecem no texto, inclusive o teto de odd.
func (c combo) name(leagueName string) string {
	var parts []string
	if c.isResult() {
		parts = []string{resultLabel(c.metric)}
	} else {
		parts = []string{fmt.Sprintf("Escanteios %d.5+", c.line)}
	}

	if c.teamName != "" {
		parts = append(parts, c.teamName)
	}
	parts = append(parts, homeAwayLabel(c.homeAway))
	if c.window > 0 {
		parts = append(parts, fmt.Sprintf("últimos %d", c.window))
	}
	// Mercado de resultado não tem teto de odd — incluir "odd ≤ 0,00" no nome
	// tornaria o identificador confuso e, pior, igual entre combinações distintas.
	if !c.isResult() {
		parts = append(parts, fmt.Sprintf("odd ≤ %.2f", c.maxOdds))
	}

	return truncate(strings.Join(parts, " · ")+" — "+leagueName, strategyNameMaxLen)
}

func homeAwayLabel(ha string) string {
	switch ha {
	case "home":
		return "mandante"
	case "away":
		return "visitante"
	default:
		return "casa e fora"
	}
}

// truncate corta por RUNA, não por byte: nomes de equipes e ligas têm acentos, e
// cortar no meio de um caractere multibyte geraria texto inválido no banco.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
