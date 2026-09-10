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
	"sort"
	"strconv"
	"strings"
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

	// baseURLOverride existe só para os testes apontarem o cliente para um
	// servidor local. Vazio em produção — ver base().
	baseURLOverride string
}

func (c *Client) base() string {
	if c.baseURLOverride != "" {
		return c.baseURLOverride
	}
	return baseURL
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
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

		// STATUS 200 NÃO SIGNIFICA SUCESSO NESTA API.
		//
		// A API-Football devolve HTTP 200 mesmo quando recusa a requisição, e
		// coloca o motivo num campo "errors" DENTRO do corpo, com "response"
		// vazio. Checar só o status HTTP faz uma recusa passar por sucesso.
		//
		// Foi assim que o pipeline ficou parado de 24/08 a 09/09 sem ninguém
		// notar: `api_usage_log` registrava 50 chamadas com status 200 e
		// success=true por ciclo, a tela de diagnóstico mostrava tudo verde, e
		// mesmo assim o worker gravava 0 partidas. O que acontecia:
		//
		//   /fixtures?league=X&season=Y  -> 200 + errors -> response vazio
		//                                -> SyncFixtures devolvia 0 partidas
		//                                   E NENHUM ERRO (achadas=0, erros=0)
		//   /fixtures?id=N               -> 200 + errors -> response vazio
		//                                -> "partida não encontrada"
		//                                   (checadas=50, erros=50)
		//
		// Um pipeline que falha em silêncio é pior que um que quebra: ninguém
		// investiga o que parece estar funcionando. Agora a recusa vira erro de
		// verdade, com a mensagem que a própria API mandou.
		if apiErr := apiErrorMessage(body); apiErr != "" {
			// Limite de requisições também chega como 200 + errors nesta API.
			// É a mesma condição do 429 e merece o mesmo backoff.
			if isRateLimitMessage(apiErr) {
				lastErr = fmt.Errorf("API-Football recusou %s por limite de requisições: %s", path, apiErr)
				continue
			}
			errMsg := fmt.Sprintf("API-Football recusou %s (status 200, erro no corpo): %s", path, apiErr)
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

// apiErrorMessage extrai o campo "errors" do corpo de uma resposta da
// API-Football. Devolve "" quando não há erro.
//
// O campo é polimórfico, e é por isso que ele precisa de tratamento próprio:
// em caso de sucesso vem como ARRAY VAZIO (`"errors": []`); em caso de recusa
// vem como OBJETO com o motivo (`"errors": {"plan": "Free plans do not have
// access to this season."}`). Desserializar direto para um tipo fixo falha em
// um dos dois casos, então o campo é lido como RawMessage e interpretado aqui.
func apiErrorMessage(body []byte) string {
	var env struct {
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil || len(env.Errors) == 0 {
		return ""
	}

	switch strings.TrimSpace(string(env.Errors)) {
	case "[]", "{}", "null", `""`:
		return "" // sucesso
	}

	// Formato de recusa: objeto chave -> motivo.
	var asMap map[string]string
	if err := json.Unmarshal(env.Errors, &asMap); err == nil && len(asMap) > 0 {
		parts := make([]string, 0, len(asMap))
		for k, v := range asMap {
			parts = append(parts, k+": "+v)
		}
		sort.Strings(parts) // ordem estável: a mensagem entra em log e em teste
		return strings.Join(parts, "; ")
	}

	// Formato inesperado: devolve cru em vez de engolir. Preferir ruído a
	// silêncio é a lição do próprio incidente que esta função corrige.
	return strings.TrimSpace(string(env.Errors))
}

// isRateLimitMessage identifica, no texto devolvido pela API, as recusas que são
// de ritmo/cota — as únicas que vale a pena repetir com backoff.
func isRateLimitMessage(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "rate") ||
		strings.Contains(m, "too many requests") ||
		strings.Contains(m, "requests per")
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
