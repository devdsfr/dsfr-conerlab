// Package llm fornece um cliente mínimo para a OpenAI Chat Completions API, usado
// exclusivamente para gerar explicações em linguagem analítica a partir de dados já
// calculados pelo backend (ver internal/usecase/intelligence.ExplainUsecase). O
// cliente nunca decide o conteúdo por conta própria — apenas formata em texto os
// números que o backend já calculou e valida a resposta contra uma lista de termos
// proibidos antes de devolvê-la. Cada chamada real à API é registrada via
// internal/usagelog, para alimentar o painel de diagnóstico "Integrações".
//
// Nota: este arquivo se chamava anthropic.go porque a integração original usava a
// Anthropic Messages API. Foi trocado para a OpenAI Chat Completions API a pedido do
// usuário (a chave fornecida era uma chave OpenAI, formato sk-proj-...). O nome do
// arquivo ficou mantido para não precisar renomear/apagar arquivo já versionado.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devdsfr/cornerlab/internal/usagelog"
)

const defaultAPIURL = "https://api.openai.com/v1/chat/completions"
const defaultModel = "gpt-4o-mini"

var ErrNotConfigured = errors.New("OPENAI_API_KEY não configurada")

type OpenAIClient struct {
	apiKey     string
	model      string
	apiURL     string
	httpClient *http.Client
	recorder   usagelog.Recorder
}

// NewOpenAIClient cria o cliente. recorder pode ser nil (nenhum uso é registrado) ou
// um usagelog.Recorder (ex: internal/repository/postgres.UsageRepo) para alimentar o
// painel de diagnóstico "Integrações".
func NewOpenAIClient(apiKey string, recorder usagelog.Recorder) *OpenAIClient {
	return &OpenAIClient{
		apiKey:     apiKey,
		model:      defaultModel,
		apiURL:     defaultAPIURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		recorder:   recorder,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []chatMessage `json:"messages"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete envia um prompt de usuário e um system prompt restritivo à API de Chat
// Completions da OpenAI e retorna o texto gerado. Retorna ErrNotConfigured se
// nenhuma chave de API estiver configurada, permitindo que a camada HTTP devolva um
// erro claro em vez de quebrar a requisição.
func (c *OpenAIClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	text, _, err := c.complete(ctx, systemPrompt, userPrompt, 600, "chat.completions")
	return text, err
}

// TestConnection faz uma chamada mínima (max_tokens baixo) apenas para validar que a
// chave configurada é aceita pela OpenAI e a API responde. Usado pelo botão "Testar
// agora" do painel de diagnóstico "Integrações".
func (c *OpenAIClient) TestConnection(ctx context.Context) error {
	_, _, err := c.complete(ctx, "Responda apenas com a palavra: ok", "ping", 5, "test_connection")
	return err
}

func (c *OpenAIClient) complete(ctx context.Context, systemPrompt, userPrompt string, maxTokens int, endpoint string) (string, *chatUsage, error) {
	start := time.Now()
	if c.apiKey == "" {
		c.record(endpoint, false, nil, nil, ErrNotConfigured.Error(), time.Since(start))
		return "", nil, ErrNotConfigured
	}

	reqBody := chatRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		c.record(endpoint, false, nil, nil, err.Error(), time.Since(start))
		return "", nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(payload))
	if err != nil {
		c.record(endpoint, false, nil, nil, err.Error(), time.Since(start))
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		wrapped := fmt.Errorf("falha ao chamar a API da OpenAI: %w", err)
		c.record(endpoint, false, nil, nil, wrapped.Error(), time.Since(start))
		return "", nil, wrapped
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.record(endpoint, false, &resp.StatusCode, nil, err.Error(), time.Since(start))
		return "", nil, err
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		wrapped := fmt.Errorf("resposta inesperada da API da OpenAI: %w", err)
		c.record(endpoint, false, &resp.StatusCode, nil, wrapped.Error(), time.Since(start))
		return "", nil, wrapped
	}
	if parsed.Error != nil {
		wrapped := fmt.Errorf("erro da API da OpenAI: %s", parsed.Error.Message)
		c.record(endpoint, false, &resp.StatusCode, parsed.Usage, wrapped.Error(), time.Since(start))
		return "", nil, wrapped
	}
	if resp.StatusCode != http.StatusOK {
		wrapped := fmt.Errorf("API da OpenAI retornou status %d", resp.StatusCode)
		c.record(endpoint, false, &resp.StatusCode, parsed.Usage, wrapped.Error(), time.Since(start))
		return "", nil, wrapped
	}
	if len(parsed.Choices) == 0 {
		err := errors.New("API da OpenAI não retornou conteúdo")
		c.record(endpoint, false, &resp.StatusCode, parsed.Usage, err.Error(), time.Since(start))
		return "", nil, err
	}

	c.record(endpoint, true, &resp.StatusCode, parsed.Usage, "", time.Since(start))
	return parsed.Choices[0].Message.Content, parsed.Usage, nil
}

func (c *OpenAIClient) record(endpoint string, success bool, statusCode *int, usage *chatUsage, errMsg string, dur time.Duration) {
	entry := usagelog.Entry{
		Provider:     usagelog.ProviderOpenAI,
		Endpoint:     endpoint,
		Success:      success,
		StatusCode:   statusCode,
		ErrorMessage: errMsg,
		DurationMs:   int(dur.Milliseconds()),
	}
	if usage != nil {
		p, comp, tot := usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
		entry.TokensPrompt, entry.TokensCompletion, entry.TokensTotal = &p, &comp, &tot
	}
	usagelog.RecordAsync(c.recorder, entry)
}
