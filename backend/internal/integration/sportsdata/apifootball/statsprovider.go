// Este arquivo implementa statsprovider.StatisticsProvider no mesmo *Client já usado
// para sportsdata.Provider (ver client.go). É o mesmo cliente HTTP, a mesma chave de
// API e o mesmo registro de uso — só um contrato adicional, mais rico, usado pelo
// Módulo de Sincronização (cmd/worker) em vez do fluxo manual de importação em lote
// (cmd/sync).
package apifootball

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
)

// mapStatus converte o código curto de status da API-Football (fixture.status.short)
// para o vocabulário interno do CornerLab. Qualquer coisa que não seja claramente
// "encerrada" fica como AGENDADO — o Worker de Atualização tenta de novo no próximo
// ciclo até a partida realmente terminar (cobre também adiamentos/cancelamentos, que
// não têm tratamento especial nesta fase do módulo).
func mapStatus(shortStatus string) statsprovider.FixtureStatus {
	switch shortStatus {
	case "FT", "AET", "PEN":
		return statsprovider.FixtureFinished
	default:
		return statsprovider.FixtureScheduled
	}
}

type competitionsResponse struct {
	Response []struct {
		League struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"league"`
		Country struct {
			Name string `json:"name"`
		} `json:"country"`
	} `json:"response"`
}

func (c *Client) SyncCompetitions(ctx context.Context, name, country string) ([]statsprovider.Competition, error) {
	body, err := c.doGet(ctx, "/leagues", map[string]string{"name": name, "country": country}, "leagues")
	if err != nil {
		return nil, err
	}
	var parsed competitionsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]statsprovider.Competition, 0, len(parsed.Response))
	for _, r := range parsed.Response {
		out = append(out, statsprovider.Competition{
			ExternalID: strconv.Itoa(r.League.ID),
			Name:       r.League.Name,
			Country:    r.Country.Name,
		})
	}
	return out, nil
}

type teamsResponse struct {
	Response []struct {
		Team struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Code    string `json:"code"`
			Country string `json:"country"`
		} `json:"team"`
	} `json:"response"`
}

func (c *Client) SyncTeams(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.TeamInfo, error) {
	body, err := c.doGet(ctx, "/teams", map[string]string{
		"league": competitionExternalID,
		"season": strconv.Itoa(season),
	}, "teams")
	if err != nil {
		return nil, err
	}
	var parsed teamsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]statsprovider.TeamInfo, 0, len(parsed.Response))
	for _, r := range parsed.Response {
		shortName := r.Team.Code
		if shortName == "" {
			shortName = shortName20(r.Team.Name)
		}
		out = append(out, statsprovider.TeamInfo{
			ExternalID: strconv.Itoa(r.Team.ID),
			Name:       r.Team.Name,
			ShortName:  shortName,
			Country:    r.Team.Country,
		})
	}
	return out, nil
}

func shortName20(name string) string {
	if len(name) <= 20 {
		return name
	}
	return name[:20]
}

type fixturesSyncResponse struct {
	Response []struct {
		Fixture struct {
			ID     int       `json:"id"`
			Date   time.Time `json:"date"`
			Status struct {
				Short string `json:"short"`
			} `json:"status"`
		} `json:"fixture"`
		League struct {
			ID     int    `json:"id"`
			Season int    `json:"season"`
			Round  string `json:"round"`
		} `json:"league"`
		Teams struct {
			Home struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"home"`
			Away struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"away"`
		} `json:"teams"`
		Goals struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"goals"`
	} `json:"response"`
}

// parseRound extrai o número da rodada do texto livre devolvido pela API-Football
// (ex: "Regular Season - 34" -> 34). Quando não há número reconhecível, devolve 0 —
// mesmo comportamento silencioso já aceito pelo FetchFixtures existente.
func parseRound(text string) int {
	var n int
	if _, err := fmt.Sscanf(text, "Regular Season - %d", &n); err == nil {
		return n
	}
	return 0
}

func (c *Client) SyncFixtures(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.Fixture, error) {
	body, err := c.doGet(ctx, "/fixtures", map[string]string{
		"league": competitionExternalID,
		"season": strconv.Itoa(season),
	}, "fixtures")
	if err != nil {
		return nil, err
	}
	var parsed fixturesSyncResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]statsprovider.Fixture, 0, len(parsed.Response))
	for _, f := range parsed.Response {
		homeGoals, awayGoals := 0, 0
		if f.Goals.Home != nil {
			homeGoals = *f.Goals.Home
		}
		if f.Goals.Away != nil {
			awayGoals = *f.Goals.Away
		}
		out = append(out, statsprovider.Fixture{
			ExternalID:         strconv.Itoa(f.Fixture.ID),
			CompetitionExtID:   strconv.Itoa(f.League.ID),
			SeasonYear:         f.League.Season,
			Round:              parseRound(f.League.Round),
			MatchDate:          f.Fixture.Date,
			Status:             mapStatus(f.Fixture.Status.Short),
			HomeTeamExternalID: strconv.Itoa(f.Teams.Home.ID),
			HomeTeamName:       f.Teams.Home.Name,
			AwayTeamExternalID: strconv.Itoa(f.Teams.Away.ID),
			AwayTeamName:       f.Teams.Away.Name,
			HomeGoals:          homeGoals,
			AwayGoals:          awayGoals,
		})
	}
	return out, nil
}

type fixtureStatsResponse struct {
	Response []struct {
		Team struct {
			ID int `json:"id"`
		} `json:"team"`
		Statistics []struct {
			Type  string `json:"type"`
			Value any    `json:"value"`
		} `json:"statistics"`
	} `json:"response"`
}

type fixtureLookupResponse struct {
	Response []struct {
		Fixture struct {
			Status struct {
				Short string `json:"short"`
			} `json:"status"`
			Venue struct {
				Name string `json:"name"`
			} `json:"venue"`
			Referee string `json:"referee"`
		} `json:"fixture"`
		Goals struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"goals"`
	} `json:"response"`
}

// statValue tenta extrair um inteiro de um bloco de estatísticas da API-Football pelo
// nome do campo. Valores de percentual vêm como string ("55%"), os demais como
// float64 ou string simples — a API não é consistente entre campos.
func statValue(stats []struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}, statType string) *int {
	for _, s := range stats {
		if s.Type != statType {
			continue
		}
		switch v := s.Value.(type) {
		case float64:
			n := int(v)
			return &n
		case string:
			clean := v
			for _, ch := range []string{"%"} {
				clean = trimSuffixAll(clean, ch)
			}
			if n, err := strconv.Atoi(clean); err == nil {
				return &n
			}
		}
	}
	return nil
}

func trimSuffixAll(s, suffix string) string {
	for len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		s = s[:len(s)-len(suffix)]
	}
	return s
}

// SyncFixtureStatistics busca separadamente o status/placar/local/árbitro (endpoint
// /fixtures?id=) e as estatísticas por equipe (endpoint /fixtures/statistics), porque
// a API-Football não devolve tudo em uma única chamada. O primeiro bloco do array de
// estatísticas é sempre o time mandante, conforme documentação oficial.
func (c *Client) SyncFixtureStatistics(ctx context.Context, fixtureExternalID string) (*statsprovider.FixtureStatistics, error) {
	lookupBody, err := c.doGet(ctx, "/fixtures", map[string]string{"id": fixtureExternalID}, "fixtures.lookup")
	if err != nil {
		return nil, err
	}
	var lookup fixtureLookupResponse
	if err := json.Unmarshal(lookupBody, &lookup); err != nil {
		return nil, err
	}
	if len(lookup.Response) == 0 {
		return nil, fmt.Errorf("partida %s não encontrada na API-Football", fixtureExternalID)
	}
	fx := lookup.Response[0]
	homeGoals, awayGoals := 0, 0
	if fx.Goals.Home != nil {
		homeGoals = *fx.Goals.Home
	}
	if fx.Goals.Away != nil {
		awayGoals = *fx.Goals.Away
	}

	result := &statsprovider.FixtureStatistics{
		ExternalID: fixtureExternalID,
		Status:     mapStatus(fx.Fixture.Status.Short),
		HomeGoals:  homeGoals,
		AwayGoals:  awayGoals,
		Referee:    fx.Fixture.Referee,
		Venue:      fx.Fixture.Venue.Name,
	}

	// Estatísticas detalhadas só existem para partidas já em andamento/encerradas —
	// não tem sentido chamar o endpoint para um jogo ainda AGENDADO.
	if result.Status != statsprovider.FixtureFinished {
		return result, nil
	}

	statsBody, err := c.doGet(ctx, "/fixtures/statistics", map[string]string{"fixture": fixtureExternalID}, "fixtures.statistics")
	if err != nil {
		// Placar/status já são válidos mesmo se as estatísticas detalhadas falharem —
		// devolve o que temos em vez de descartar tudo (partida fica FINALIZADO com
		// campos de estatística nil, o Worker de Atualização não tenta de novo pois o
		// status já não é mais AGENDADO).
		return result, nil
	}
	var stats fixtureStatsResponse
	if err := json.Unmarshal(statsBody, &stats); err != nil || len(stats.Response) < 2 {
		return result, nil
	}

	home, away := stats.Response[0].Statistics, stats.Response[1].Statistics
	result.HomeCorners = statValue(home, "Corner Kicks")
	result.AwayCorners = statValue(away, "Corner Kicks")
	result.HomePossessionPct = statValue(home, "Ball Possession")
	result.AwayPossessionPct = statValue(away, "Ball Possession")
	result.HomeShots = statValue(home, "Total Shots")
	result.AwayShots = statValue(away, "Total Shots")
	result.HomeShotsOnTarget = statValue(home, "Shots on Goal")
	result.AwayShotsOnTarget = statValue(away, "Shots on Goal")
	result.HomeYellowCards = statValue(home, "Yellow Cards")
	result.AwayYellowCards = statValue(away, "Yellow Cards")
	result.HomeRedCards = statValue(home, "Red Cards")
	result.AwayRedCards = statValue(away, "Red Cards")

	// Prioridade Média: chutes de dentro/fora da área, chutes bloqueados, faltas e
	// impedimentos — mesma resposta, sem chamada extra à API.
	result.HomeShotsInsidebox = statValue(home, "Shots insidebox")
	result.AwayShotsInsidebox = statValue(away, "Shots insidebox")
	result.HomeShotsOutsidebox = statValue(home, "Shots outsidebox")
	result.AwayShotsOutsidebox = statValue(away, "Shots outsidebox")
	result.HomeBlockedShots = statValue(home, "Blocked Shots")
	result.AwayBlockedShots = statValue(away, "Blocked Shots")
	result.HomeFouls = statValue(home, "Fouls")
	result.AwayFouls = statValue(away, "Fouls")
	result.HomeOffsides = statValue(home, "Offsides")
	result.AwayOffsides = statValue(away, "Offsides")

	return result, nil
}

type standingsResponse struct {
	Response []struct {
		League struct {
			Standings [][]struct {
				Rank int `json:"rank"`
				Team struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"team"`
				Points int `json:"points"`
				All    struct {
					Played int `json:"played"`
					Win    int `json:"win"`
					Draw   int `json:"draw"`
					Lose   int `json:"lose"`
				} `json:"all"`
			} `json:"standings"`
		} `json:"league"`
	} `json:"response"`
}

func (c *Client) SyncStandings(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.StandingEntry, error) {
	body, err := c.doGet(ctx, "/standings", map[string]string{
		"league": competitionExternalID,
		"season": strconv.Itoa(season),
	}, "standings")
	if err != nil {
		return nil, err
	}
	var parsed standingsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Response) == 0 || len(parsed.Response[0].League.Standings) == 0 {
		return nil, nil
	}
	table := parsed.Response[0].League.Standings[0]
	out := make([]statsprovider.StandingEntry, 0, len(table))
	for _, row := range table {
		out = append(out, statsprovider.StandingEntry{
			TeamExternalID: strconv.Itoa(row.Team.ID),
			TeamName:       row.Team.Name,
			Position:       row.Rank,
			Played:         row.All.Played,
			Won:            row.All.Win,
			Drawn:          row.All.Draw,
			Lost:           row.All.Lose,
			Points:         row.Points,
		})
	}
	return out, nil
}

type statusEndpointResponse struct {
	Response struct {
		Requests struct {
			Current  int `json:"current"`
			LimitDay int `json:"limit_day"`
		} `json:"requests"`
	} `json:"response"`
}

// HealthCheck usa o endpoint /status (não consome cota de requisições, conforme
// documentação da API-Football, o mesmo já usado por TestConnection) para checar:
// endpoint responde, sem bloqueio (403/429 — capturado como erro por doGet, que já
// distingue status HTTP de erro de rede) e o formato esperado da resposta continua o
// mesmo. Validação de campos "de conteúdo" (escanteios, posse, cartões) acontece de
// forma contínua como efeito colateral do Worker de Atualização: se SyncFixtureStatistics
// parar de encontrar esses campos, isso já é reportado ao mesmo mecanismo de
// incidentes (ver internal/usecase/statsync).
func (c *Client) HealthCheck(ctx context.Context) (statsprovider.HealthResult, error) {
	start := time.Now()
	body, err := c.doGet(ctx, "/status", nil, "status")
	elapsed := time.Since(start)
	if err != nil {
		return statsprovider.HealthResult{OK: false, ResponseTime: elapsed, Message: err.Error()}, nil
	}
	var parsed statusEndpointResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return statsprovider.HealthResult{OK: false, ResponseTime: elapsed, Message: "formato de resposta inesperado do endpoint /status: " + err.Error()}, nil
	}
	const slowThreshold = 5 * time.Second
	if elapsed > slowThreshold {
		return statsprovider.HealthResult{OK: false, ResponseTime: elapsed, Message: fmt.Sprintf("tempo de resposta acima do esperado: %s", elapsed)}, nil
	}
	return statsprovider.HealthResult{OK: true, ResponseTime: elapsed}, nil
}
