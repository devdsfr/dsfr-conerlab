// Package apifootball implementa sportsdata.Provider usando a API-Football
// (api-sports.io / v3.football.api-sports.io). Requer uma chave de assinatura direta
// da API-Sports (header "x-apisports-key"). Para uso via RapidAPI, ajuste os headers
// em newRequest. Cada chamada real é registrada via internal/usagelog, para alimentar
// o painel de diagnóstico "Integrações".
package apifootball

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata"
	"github.com/devdsfr/cornerlab/internal/usagelog"
)

const baseURL = "https://v3.football.api-sports.io"

// minRequestInterval espaça as chamadas à API-Football. O plano gratuito limita
// requisições POR MINUTO (além da cota diária) e responde 429 quando o worker
// dispara em rajada — foi exatamente o que aconteceu em produção: um ciclo
// verificou 50 partidas em 13 segundos (~4 req/s) e a API cortou com
// "status 429 para /fixtures". 6,5s entre chamadas mantém o ritmo em ~9 por
// minuto, abaixo do teto de 10/min do plano gratuito.
//
// Se um dia o plano for atualizado, basta reduzir esta constante — nenhum outro
// ponto do código precisa mudar.
const minRequestInterval = 6500 * time.Millisecond

// maxRetriesOn429 define quantas vezes uma chamada é repetida quando a API
// responde 429. O intervalo dobra a cada tentativa (backoff exponencial),
// começando em minRequestInterval.
const maxRetriesOn429 = 3

var ErrNotConfigured = errors.New("API_FOOTBALL_KEY não configurada")

type Client struct {
	apiKey     string
	httpClient *http.Client
	recorder   usagelog.Recorder

	// throttle serializa as chamadas e garante o espaçamento mínimo entre elas.
	// Um mutex (em vez de um ticker global) mantém o cliente utilizável tanto
	// pelo worker quanto por um handler HTTP sem vazar goroutine.
	throttle sync.Mutex
	lastCall time.Time
}

// New cria o cliente. recorder pode ser nil (nenhum uso é registrado) ou um
// usagelog.Recorder (ex: internal/repository/postgres.UsageRepo).
func New(apiKey string, recorder usagelog.Recorder) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{Timeout: 20 * time.Second}, recorder: recorder}
}

// wait bloqueia até que o intervalo mínimo desde a última chamada tenha passado.
// Respeita o cancelamento do contexto: se o ciclo do worker for interrompido, a
// espera é abortada em vez de segurar o processo.
func (c *Client) wait(ctx context.Context) error {
	c.throttle.Lock()
	defer c.throttle.Unlock()

	if elapsed := time.Since(c.lastCall); !c.lastCall.IsZero() && elapsed < minRequestInterval {
		select {
		case <-time.After(minRequestInterval - elapsed):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.lastCall = time.Now()
	return nil
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

// doGet executa uma requisição GET autenticada e registra o resultado (sucesso/erro,
// status HTTP, duração) via internal/usagelog. endpointLabel identifica a operação no
// histórico de uso (ex: "leagues", "fixtures", "fixtures.statistics", "status").
func (c *Client) doGet(ctx context.Context, path string, query map[string]string, endpointLabel string) ([]byte, error) {
	start := time.Now()
	if c.apiKey == "" {
		c.record(endpointLabel, false, nil, ErrNotConfigured.Error(), time.Since(start))
		return nil, ErrNotConfigured
	}

	backoff := minRequestInterval
	var lastErr error

	// Tentativa 0 é a chamada normal; as seguintes só acontecem em caso de 429.
	for attempt := 0; attempt <= maxRetriesOn429; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				c.record(endpointLabel, false, nil, ctx.Err().Error(), time.Since(start))
				return nil, ctx.Err()
			}
			backoff *= 2
		}

		// Espaçamento mínimo entre chamadas — ver minRequestInterval.
		if err := c.wait(ctx); err != nil {
			c.record(endpointLabel, false, nil, err.Error(), time.Since(start))
			return nil, err
		}

		req, err := c.newRequest(ctx, path, query)
		if err != nil {
			c.record(endpointLabel, false, nil, err.Error(), time.Since(start))
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			wrapped := fmt.Errorf("falha ao chamar a API-Football (%s): %w", endpointLabel, err)
			c.record(endpointLabel, false, nil, wrapped.Error(), time.Since(start))
			return nil, wrapped
		}

		body, readErr := io.ReadAll(resp.Body)
		status := resp.StatusCode
		resp.Body.Close()

		if readErr != nil {
			c.record(endpointLabel, false, &status, readErr.Error(), time.Since(start))
			return nil, readErr
		}

		// 429 = cota/ritmo estourado. Vale repetir depois de esperar; qualquer
		// outro status de erro é definitivo (chave inválida, endpoint errado...).
		if status == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("API-Football retornou status 429 para %s (limite de requisições)", path)
			continue
		}

		if status != http.StatusOK {
			errMsg := fmt.Sprintf("API-Football retornou status %d para %s", status, path)
			c.record(endpointLabel, false, &status, errMsg, time.Since(start))
			return nil, errors.New(errMsg)
		}

		c.record(endpointLabel, true, &status, "", time.Since(start))
		return body, nil
	}

	// Esgotou as tentativas sempre recebendo 429.
	status := http.StatusTooManyRequests
	c.record(endpointLabel, false, &status, lastErr.Error(), time.Since(start))
	return nil, lastErr
}

func (c *Client) record(endpoint string, success bool, statusCode *int, errMsg string, dur time.Duration) {
	usagelog.RecordAsync(c.recorder, usagelog.Entry{
		Provider:     usagelog.ProviderAPIFootball,
		Endpoint:     endpoint,
		Success:      success,
		StatusCode:   statusCode,
		ErrorMessage: errMsg,
		DurationMs:   int(dur.Milliseconds()),
	})
}

// TestConnection chama o endpoint /status da API-Football, que devolve informações da
// assinatura (plano, cota diária) sem consumir a cota de requisições — ideal para o
// botão "Testar agora" do painel de diagnóstico.
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doGet(ctx, "/status", nil, "status")
	return err
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
	body, err := c.doGet(ctx, "/leagues", map[string]string{"name": leagueName, "country": country}, "leagues")
	if err != nil {
		return 0, err
	}
	var parsed leaguesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
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

	body, err := c.doGet(ctx, "/fixtures", map[string]string{
		"league": strconv.Itoa(leagueID),
		"season": strconv.Itoa(season),
	}, "fixtures")
	if err != nil {
		return nil, err
	}

	var parsed fixturesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
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
	body, err := c.doGet(ctx, "/fixtures/statistics", map[string]string{"fixture": fixtureExternalID}, "fixtures.statistics")
	if err != nil {
		return 0, 0, false, err
	}

	var parsed statisticsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
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
