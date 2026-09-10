package apifootball

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Testes da falha silenciosa que parou o pipeline de 24/08 a 09/09.
//
// A API-Football devolve HTTP 200 mesmo quando RECUSA a requisição, colocando o
// motivo num campo "errors" dentro do corpo e deixando "response" vazio. O
// cliente checava só o status HTTP, então uma recusa virava "sucesso com zero
// resultados": o worker gravava nada, o log de uso registrava 200/success=true e
// a tela de diagnóstico ficava verde. Ninguém investiga o que parece funcionar.

// newTestClient aponta o cliente para um servidor local e remove o espaçamento
// entre chamadas (6,5s por requisição inviabilizaria os testes).
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := New("chave-de-teste", nil)
	c.httpClient = srv.Client()
	c.baseURLOverride = srv.URL
	// lastCall no futuro faria wait() dormir; zerar mantém o teste instantâneo.
	c.lastCall = time.Time{}
	return c, srv.Close
}

func TestAPIErrorMessage(t *testing.T) {
	casos := []struct {
		nome  string
		corpo string
		want  string
	}{
		// Sucesso: a API manda array vazio.
		{"array vazio", `{"errors":[],"response":[1,2]}`, ""},
		{"objeto vazio", `{"errors":{},"response":[]}`, ""},
		{"sem o campo", `{"response":[]}`, ""},
		{"nulo", `{"errors":null}`, ""},

		// Recusa: objeto com o motivo. É este o caso que passava por sucesso.
		{
			"plano nao cobre a temporada",
			`{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}`,
			"plan: Free plans do not have access to this season.",
		},
		{
			"chave invalida",
			`{"errors":{"token":"Error/Missing application key."},"response":[]}`,
			"token: Error/Missing application key.",
		},
		{
			"limite de requisicoes",
			`{"errors":{"rateLimit":"Too many requests."},"response":[]}`,
			"rateLimit: Too many requests.",
		},

		// Corpo malformado não pode virar silêncio.
		{"formato inesperado", `{"errors":"algo estranho"}`, `"algo estranho"`},
	}

	for _, c := range casos {
		if got := apiErrorMessage([]byte(c.corpo)); got != c.want {
			t.Errorf("%s: apiErrorMessage = %q, esperado %q", c.nome, got, c.want)
		}
	}
}

// Vários motivos vêm juntos: a mensagem tem que ser estável entre execuções,
// senão o log e o próprio teste ficam intermitentes.
func TestAPIErrorMessageOrdemEstavel(t *testing.T) {
	corpo := []byte(`{"errors":{"token":"chave ruim","plan":"sem acesso","bug":"x"}}`)
	primeiro := apiErrorMessage(corpo)
	for i := 0; i < 20; i++ {
		if got := apiErrorMessage(corpo); got != primeiro {
			t.Fatalf("mensagem instável: %q vs %q", got, primeiro)
		}
	}
	if !strings.HasPrefix(primeiro, "bug:") {
		t.Errorf("esperava ordem alfabética, veio %q", primeiro)
	}
}

// O TESTE QUE IMPORTA: 200 com erro no corpo tem que virar erro de verdade.
func TestRecusaComStatus200ViraErro(t *testing.T) {
	c, fechar := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}`))
	})
	defer fechar()

	_, err := c.doGet(context.Background(), "/fixtures", map[string]string{"league": "71"}, "fixtures")
	if err == nil {
		t.Fatal("recusa com status 200 passou por sucesso — é exatamente o defeito que parou o pipeline")
	}
	if !strings.Contains(err.Error(), "Free plans do not have access") {
		t.Errorf("o erro precisa carregar a mensagem da API para ser diagnosticável, veio: %v", err)
	}
}

// Consequência prática no worker de descoberta: antes, a recusa virava "0
// partidas, nenhum erro" e o ciclo seguia como se estivesse tudo bem.
func TestSyncFixturesNaoDevolveVazioSilenciosoEmRecusa(t *testing.T) {
	c, fechar := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}`))
	})
	defer fechar()

	fixtures, err := c.SyncFixtures(context.Background(), "71", 2026)
	if err == nil {
		t.Fatalf("SyncFixtures devolveu %d partidas sem erro; a API tinha recusado a requisição", len(fixtures))
	}
}

// Consequência no worker de atualização: virava "partida não encontrada",
// mascarando uma recusa de plano como problema da partida.
func TestSyncFixtureStatisticsDistingueRecusaDePartidaInexistente(t *testing.T) {
	c, fechar := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}`))
	})
	defer fechar()

	_, err := c.SyncFixtureStatistics(context.Background(), "12345")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), "não encontrada") {
		t.Errorf("recusa da API está sendo reportada como partida inexistente: %v", err)
	}
	if !strings.Contains(err.Error(), "Free plans") {
		t.Errorf("erro não carrega o motivo real: %v", err)
	}
}

// Resposta legítima tem que continuar passando — a correção não pode transformar
// "nenhum jogo neste período" em erro.
func TestRespostaValidaContinuaPassando(t *testing.T) {
	c, fechar := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[],"results":0,"response":[]}`))
	})
	defer fechar()

	fixtures, err := c.SyncFixtures(context.Background(), "71", 2026)
	if err != nil {
		t.Fatalf("resposta válida com zero partidas não é erro: %v", err)
	}
	if len(fixtures) != 0 {
		t.Errorf("esperava 0 partidas, veio %d", len(fixtures))
	}
}

func TestStatusHTTPDeErroContinuaSendoErro(t *testing.T) {
	c, fechar := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{}`))
	})
	defer fechar()

	if _, err := c.doGet(context.Background(), "/fixtures", nil, "fixtures"); err == nil {
		t.Fatal("status 403 deveria continuar sendo erro")
	}
}

func TestIsRateLimitMessage(t *testing.T) {
	sim := []string{
		"rateLimit: Too many requests.",
		"requests: You have reached the requests per day limit.",
		"rateLimit: too many requests per minute",
	}
	nao := []string{
		"plan: Free plans do not have access to this season.",
		"token: Error/Missing application key.",
		"",
	}
	for _, m := range sim {
		if !isRateLimitMessage(m) {
			t.Errorf("%q deveria ser tratado como limite de requisições", m)
		}
	}
	for _, m := range nao {
		if isRateLimitMessage(m) {
			t.Errorf("%q NÃO é limite de requisições — repetir com backoff não resolveria", m)
		}
	}
}
