// Package apifootball implementa sportsdata.Provider usando a API-Football
// (api-sports.io / v3.football.api-sports.io). Requer uma chave de assinatura direta
// da API-Sports (header "x-apisports-key"). Para uso via RapidAPI, ajuste os headers
// em newRequest.
package apifootball

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata"
)

const baseURL = "https://v3.football.api-sports.io"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Name() string { return "api-football" }

func (c *Client) newRequest(ctx context.Context, path string, query map[string]string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("x-apisports-key", c.apiKey)
	return req, nil
}

type leaguesResponse struct {
	Response []struct {
		League struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"league"`
	} `json:"response"`
}

func (c *Client) resolveLeagueID(ctx context.Context, leagueName, country string) (int, error) {
	req, err := c.newRequest(ctx, "/leagues", map[string]string{"name": leagueName, "country": country})
	if err != nil {
		return 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var parsed leaguesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if len(parsed.Response) == 0 {
		return 0, fmt.Errorf("campeonato '%s' (%s) não encontrado na API-Football", leagueName, country)
	}
	return parsed.Response[0].League.ID, nil
}

type fixturesResponse struct {
	Response []struct {
		Fixture struct {
			ID   int       `json:"id"`
			Date time.Time `json:"date"`
		} `json:"fixture"`
		League struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Country string `json:"country"`
			Season  int    `json:"season"`
			Round   string `json:"round"`
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

func (c *Client) FetchFixtures(ctx context.Context, leagueName, country string, season int) ([]sportsdata.Fixture, error) {
	leagueID, err := c.resolveLeagueID(ctx, leagueName, country)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, "/fixtures", map[string]string{
		"league": strconv.Itoa(leagueID),
		"season": strconv.Itoa(season),
	})
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed fixturesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	fixtures := make([]sportsdata.Fixture, 0, len(parsed.Response))
	for _, f := range parsed.Response {
		homeGoals, awayGoals := 0, 0
		if f.Goals.Home != nil {
			homeGoals = *f.Goals.Home
		}
		if f.Goals.Away != nil {
			awayGoals = *f.Goals.Away
		}
		fixtures = append(fixtures, sportsdata.Fixture{
			ExternalID:         strconv.Itoa(f.Fixture.ID),
			LeagueExternalID:   strconv.Itoa(f.League.ID),
			LeagueName:         f.League.Name,
			LeagueCountry:      f.League.Country,
			SeasonYear:         f.League.Season,
			MatchDate:          f.Fixture.Date,
			HomeTeamExternalID: strconv.Itoa(f.Teams.Home.ID),
			HomeTeamName:       f.Teams.Home.Name,
			AwayTeamExternalID: strconv.Itoa(f.Teams.Away.ID),
			AwayTeamName:       f.Teams.Away.Name,
			HomeGoals:          homeGoals,
			AwayGoals:          awayGoals,
			// API-Football não retorna escanteios na listagem de fixtures — é preciso
			// uma chamada extra por partida via FetchCorners.
			HomeCorners: nil,
			AwayCorners: nil,
		})
	}
	return fixtures, nil
}

type statisticsResponse struct {
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

// FetchCorners busca a estatística "Corner Kicks" de uma partida específica. A API
// retorna um bloco de estatísticas por equipe (mandante e visitante); o primeiro
// bloco do array é sempre o time mandante, conforme documentação da API-Football.
func (c *Client) FetchCorners(ctx context.Context, fixtureExternalID string) (int, int, bool, error) {
	req, err := c.newRequest(ctx, "/fixtures/statistics", map[string]string{"fixture": fixtureExternalID})
	if err != nil {
		return 0, 0, false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, false, err
	}
	defer resp.Body.Close()

	var parsed statisticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, 0, false, err
	}
	if len(parsed.Response) < 2 {
		return 0, 0, false, nil
	}

	extractCorners := func(stats []struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}) (int, bool) {
		for _, s := range stats {
			if s.Type == "Corner Kicks" {
				switch v := s.Value.(type) {
				case float64:
					return int(v), true
				case string:
					n, err := strconv.Atoi(v)
					if err == nil {
						return n, true
					}
				}
			}
		}
		return 0, false
	}

	home, homeOK := extractCorners(parsed.Response[0].Statistics)
	away, awayOK := extractCorners(parsed.Response[1].Statistics)
	if !homeOK || !awayOK {
		return 0, 0, false, nil
	}
	return home, away, true, nil
}
