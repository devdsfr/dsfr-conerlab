package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devdsfr/cornerlab/internal/domain"
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

	// Força do adversário pela classificação da liga: "" = qualquer adversário.
	opponentTierOptions = []string{"", "G6", "G12", "Z4"}

	// Teto de odd. TODOS os valores são > 0 DE PROPÓSITO: com MaxOdds = 0 o motor
	// aceita jogos sem odd histórica e assume odd 1.0, o que produziria um ROI
	// artificialmente negativo e sem significado. Exigir um teto garante que só
	// entrem no backtest jogos com odd real registrada — sem isso não há como
	// validar ROI/EV, que são critérios obrigatórios do doc 08.
	maxOddsOptions = []float64{1.70, 2.20, 3.50}
)

// combo é uma combinação candidata do espaço de busca.
type combo struct {
	teamID   *int64
	teamName string
	line     int
	homeAway string
	window   int
	tier     string
	maxOdds  float64
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
				for _, tier := range opponentTierOptions {
					for _, odds := range maxOddsOptions {
						out = append(out, combo{
							line: line, homeAway: ha, window: window,
							tier: tier, maxOdds: odds,
						})
					}
				}
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
		OpponentTier:     c.tier,
		MaxOdds:          c.maxOdds,
		Metric:           "corners",
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
	parts := []string{fmt.Sprintf("Escanteios %d.5+", c.line)}

	if c.teamName != "" {
		parts = append(parts, c.teamName)
	}
	parts = append(parts, homeAwayLabel(c.homeAway))
	if c.window > 0 {
		parts = append(parts, fmt.Sprintf("últimos %d", c.window))
	}
	if c.tier != "" {
		parts = append(parts, "vs "+c.tier)
	}
	parts = append(parts, fmt.Sprintf("odd ≤ %.2f", c.maxOdds))

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
