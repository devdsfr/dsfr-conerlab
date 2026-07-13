// Package llm fornece um cliente mínimo para a OpenAI Chat Completions API, usado
// exclusivamente para gerar explicações em linguagem analítica a partir de dados já
// calculados pelo backend (ver internal/usecase/intelligence.ExplainUsecase). O
// cliente nunca decide o conteúdo por conta própria — apenas formata em texto os
// números que o backend já calculou e valida a resposta contra uma lista de termos
// proibidos antes de devolvê-la.
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
)

const defaultAPIURL = "https://api.openai.com/v1/chat/completions"
const defaultModel = "gpt-4o-mini"

var ErrNotConfigured = errors.New("OPENAI_API_KEY não configurada")

type OpenAIClient struct {
	apiKey     string
	model      string
	apiURL     string
	httpClient *http.Client
}

func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:     apiKey,
		model:      defaultModel,
		apiURL:     defaultAPIURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
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

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
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
	if c.apiKey == "" {
		return "", ErrNotConfigured
	}

	reqBody := chatRequest{
		Model:     c.model,
		MaxTokens: 600,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha ao chamar a API da OpenAI: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("resposta inesperada da API da OpenAI: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("erro da API da OpenAI: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API da OpenAI retornou status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("API da OpenAI não retornou conteúdo")
	}

	return parsed.Choices[0].Message.Content, nil
}
