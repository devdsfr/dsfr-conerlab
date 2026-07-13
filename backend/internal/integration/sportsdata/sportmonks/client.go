// Package sportmonks implementa sportsdata.Provider usando a SportMonks Football
// API v3. Diferente da API-Football, o SportMonks permite trazer participantes,
// placar e estatísticas em uma única chamada via "include", reduzindo o número de
// requisições necessárias para sincronizar uma temporada inteira. Cada chamada real é
// registrada via internal/usagelog, para alimentar o painel de diagnóstico
// "Integrações".
//
// Observação: os nomes de filtros/includes usados aqui seguem a convenção pública da
// v3 (https://docs.sportmonks.com/v3) no momento da escrita. Como a disponibilidade
// de includes varia por plano de assinatura, valide os nomes exatos no seu painel
// SportMonks antes de rodar uma sincronização em produção.
package sportmonks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata"
	"github.com/devdsfr/cornerlab/internal/usagelog"
)

const baseURL = "https://api.sportmonks.com/v3/football"

// cornersDeveloperName é o identificador estável (developer_name) usado pela
// SportMonks para a estatística de escanteios — resolvido dinamicamente via o
// include "statistics.type" em vez de um type_id fixo, para não depender de um
// número que pode variar entre contas/planos.
const cornersDeveloperName = "CORNERS"

var ErrNotConfigured = errors.New("SPORTMONKS_KEY não configurada")

type Client struct {
	apiToken   string
	httpClient *http.Client
	recorder   usagelog.Recorder
}

// New cria o cliente. recorder pode ser nil (nenhum uso é registrado) ou um
// usagelog.Recorder (ex: internal/repository/postgres.UsageRepo).
func New(apiToken string, recorder usagelog.Recorder) *Client {
	return &Client{apiToken: apiToken, httpClient: &http.Client{Timeout: 20 * time.Second}, recorder: recorder}
}

func (c *Client) Name() string { return "sportmonks" }

// get executa uma requisição GET autenticada e registra o resultado (sucesso/erro,
// status HTTP, duração) via internal/usagelog. endpointLabel identifica a operação no
// histórico de uso (ex: "leagues.search", "leagues.get", "fixtures").
func (c *Client) get(ctx context.Context, path string, query map[string]string, endpointLabel string) ([]byte, error) {
	start := time.Now()
	if c.apiToken == "" {
		c.record(endpointLabel, false, nil, ErrNotConfigured.Error(), time.Since(start))
		return nil, ErrNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		c.record(endpointLabel, false, nil, err.Error(), time.Since(start))
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	q.Set("api_token", c.apiToken)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		wrapped := fmt.Errorf("falha ao chamar a SportMonks (%s): %w", endpointLabel, err)
		c.record(endpointLabel, false, nil, wrapped.Error(), time.Since(start))
		return nil, wrapped
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("sportmonks retornou status %d para %s", resp.StatusCode, path)
		c.record(endpointLabel, false, &resp.StatusCode, errMsg, time.Since(start))
		return nil, errors.New(errMsg)
	}

	buf := make([]byte, 0)
	chunk := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	c.record(endpointLabel, true, &resp.StatusCode, "", time.Since(start))
	return buf, nil
}

func (c *Client) record(endpoint string, success bool, statusCode *int, errMsg string, dur time.Duration) {
	usagelog.RecordAsync(c.recorder, usagelog.Entry{
		Provider:     usagelog.ProviderSportMonks,
		Endpoint:     endpoint,
		Success:      success,
		StatusCode:   statusCode,
		ErrorMessage: errMsg,
		DurationMs:   int(dur.Milliseconds()),
	})
}

// TestConnection faz uma chamada leve (1 registro) só para validar que o token
// configurado é aceito pela SportMonks e a API responde. Usado pelo botão "Testar
// agora" do painel de diagnóstico "Integrações".
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.get(ctx, "/leagues", map[string]string{"per_page": "1"}, "test_connection")
	return err
}

type leagueSearchResponse struct {
	Data []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Country struct {
			Name string `json:"name"`
		} `json:"country"`
	} `json:"data"`
}

func (c *Client) resolveLeagueID(ctx context.Context, leagueName string) (int, error) {
	body, err := c.get(ctx, "/leagues/search/"+leagueName, nil, "leagues.search")
	if err != nil {
		return 0, err
	}
	var parsed leagueSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	if len(parsed.Data) == 0 {
		return 0, fmt.Errorf("campeonato '%s' não encontrado na SportMonks", leagueName)
	}
	return parsed.Data[0].ID, nil
}

type seasonsResponse struct {
	Data struct {
		Seasons []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"seasons"`
	} `json:"data"`
}

func (c *Client) resolveSeasonID(ctx context.Context, leagueID, year int) (int, error) {
	body, err := c.get(ctx, fmt.Sprintf("/leagues/%d", leagueID), map[string]string{"include": "seasons"}, "leagues.get")
	if err != nil {
		return 0, err
	}
	var parsed seasonsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	yearStr := strconv.Itoa(year)
	for _, s := range parsed.Data.Seasons {
		if s.Name == yearStr || s.Name == fmt.Sprintf("%d/%d", year, year+1) {
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("temporada %d não encontrada para o campeonato %d na SportMonks", year, leagueID)
}

type fixturesResponse struct {
	Data []struct {
		ID           int       `json:"id"`
		LeagueID     int       `json:"league_id"`
		StartingAt   time.Time `json:"starting_at"`
		Participants []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Meta struct {
				Location string `json:"location"` // "home" | "away"
			} `json:"meta"`
		} `json:"participants"`
		Scores []struct {
			Description string `json:"description"` // "CURRENT"
			Score       struct {
				Goals       int    `json:"goals"`
				Participant string `json:"participant"` // "home" | "away"
			} `json:"score"`
		} `json:"scores"`
		Statistics []struct {
			ParticipantID int `json:"participant_id"`
			Type          struct {
				DeveloperName string `json:"developer_name"`
			} `json:"type"`
			Data struct {
				Value float64 `json:"value"`
			} `json:"data"`
		} `json:"statistics"`
	} `json:"data"`
}

func (c *Client) FetchFixtures(ctx context.Context, leagueName, country string, season int) ([]sportsdata.Fixture, error) {
	leagueID, err := c.resolveLeagueID(ctx, leagueName)
	if err != nil {
		return nil, err
	}
	seasonID, err := c.resolveSeasonID(ctx, leagueID, season)
	if err != nil {
		return nil, err
	}

	body, err := c.get(ctx, "/fixtures", map[string]string{
		"filters": fmt.Sprintf("fixtureSeasons:%d", seasonID),
		"include": "participants;scores;statistics.type",
	}, "fixtures")
	if err != nil {
		return nil, err
	}
	var parsed fixturesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	fixtures := make([]sportsdata.Fixture, 0, len(parsed.Data))
	for _, f := range parsed.Data {
		var homeID, awayID int
		var homeName, awayName string
		for _, p := range f.Participants {
			if p.Meta.Location == "home" {
				homeID, homeName = p.ID, p.Name
			} else if p.Meta.Location == "away" {
				awayID, awayName = p.ID, p.Name
			}
		}
		homeGoals, awayGoals := 0, 0
		for _, s := range f.Scores {
			if s.Description != "CURRENT" {
				continue
			}
			if s.Score.Participant == "home" {
				homeGoals = s.Score.Goals
			} else if s.Score.Participant == "away" {
				awayGoals = s.Score.Goals
			}
		}

		var homeCorners, awayCorners *int
		for _, st := range f.Statistics {
			if st.Type.DeveloperName != cornersDeveloperName {
				continue
			}
			v := int(st.Data.Value)
			if st.ParticipantID == homeID {
				homeCorners = &v
			} else if st.ParticipantID == awayID {
				awayCorners = &v
			}
		}

		fixtures = append(fixtures, sportsdata.Fixture{
			ExternalID:         strconv.Itoa(f.ID),
			LeagueExternalID:   strconv.Itoa(leagueID),
			LeagueName:         leagueName,
			LeagueCountry:      country,
			SeasonYear:         season,
			MatchDate:          f.StartingAt,
			HomeTeamExternalID: strconv.Itoa(homeID),
			HomeTeamName:       homeName,
			AwayTeamExternalID: strconv.Itoa(awayID),
			AwayTeamName:       awayName,
			HomeGoals:          homeGoals,
			AwayGoals:          awayGoals,
			HomeCorners:        homeCorners,
			AwayCorners:        awayCorners,
		})
	}
	return fixtures, nil
}

// FetchCorners não é necessário para o SportMonks: as estatísticas já vêm incluídas
// em FetchFixtures via o include "statistics.type". Mantido apenas para satisfazer a
// interface sportsdata.Provider.
func (c *Client) FetchCorners(ctx context.Context, fixtureExternalID string) (int, int, bool, error) {
	return 0, 0, false, nil
}
