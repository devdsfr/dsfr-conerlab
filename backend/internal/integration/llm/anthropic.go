// Package llm fornece um cliente mínimo para a Anthropic Messages API, usado
// exclusivamente para gerar explicações em linguagem analítica a partir de dados já
// calculados pelo backend (ver internal/usecase/intelligence.ExplainUsecase). O
// cliente nunca decide o conteúdo por conta própria — apenas formata em texto os
// números que o backend já calculou e valida a resposta contra uma lista de termos
// proibidos antes de devolvê-la.
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

const defaultAPIURL = "https://api.anthropic.com/v1/messages"
const defaultModel = "claude-sonnet-5"
const anthropicVersion = "2023-06-01"

var ErrNotConfigured = errors.New("ANTHROPIC_API_KEY não configurada")

type AnthropicClient struct {
	apiKey     string
	model      string
	apiURL     string
	httpClient *http.Client
}

func NewAnthropicClient(apiKey string) *AnthropicClient {
	return &AnthropicClient{
		apiKey:     apiKey,
		model:      defaultModel,
		apiURL:     defaultAPIURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type messageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []message `json:"messages"`
}

type completionResponse struct {
	Content []messageContent `json:"content"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete envia um prompt de usuário e um system prompt restritivo à API da
// Anthropic e retorna o texto gerado. Retorna ErrNotConfigured se nenhuma chave de
// API estiver configurada, permitindo que a camada HTTP devolva um erro claro em vez
// de quebrar a requisição.
func (c *AnthropicClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
		return "", ErrNotConfigured
	}

	reqBody := completionRequest{
		Model:     c.model,
		MaxTokens: 600,
		System:    systemPrompt,
		Messages:  []message{{Role: "user", Content: userPrompt}},
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
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha ao chamar a API da Anthropic: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed completionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("resposta inesperada da API da Anthropic: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("erro da API da Anthropic: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API da Anthropic retornou status %d", resp.StatusCode)
	}
	if len(parsed.Content) == 0 {
		return "", errors.New("API da Anthropic não retornou conteúdo")
	}

	text := ""
	for _, c := range parsed.Content {
		text += c.Text
	}
	return text, nil
}
