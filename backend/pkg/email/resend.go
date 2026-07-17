// Package email fornece um cliente mínimo para a Resend API (https://resend.com),
// usado exclusivamente para enviar o e-mail de redefinição de senha (ver
// internal/usecase.AuthUsecase.ForgotPassword). Segue o mesmo padrão dos outros
// clientes hand-rolled do backend (ver internal/integration/llm.OpenAIClient): sem
// SDK externo, apenas net/http, e um método Configured() para o restante do backend
// decidir se deve tentar enviar e-mails ou responder com erro claro.
package email

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

const defaultAPIURL = "https://api.resend.com/emails"

// ErrNotConfigured é retornado quando RESEND_API_KEY não está definida — o chamador
// (AuthUsecase) traduz isso em uma resposta HTTP clara em vez de falhar silenciosamente.
var ErrNotConfigured = errors.New("envio de e-mail não configurado (RESEND_API_KEY ausente)")

// Sender é a interface que o restante do backend depende — permite trocar o
// provedor de e-mail (ou usar um fake nos testes) sem tocar em AuthUsecase.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
	Configured() bool
}

type ResendClient struct {
	apiKey     string
	from       string
	apiURL     string
	httpClient *http.Client
}

// NewResendClient cria o cliente. from deve estar no formato "Nome <email@dominio>"
// aceito pela Resend — por padrão usa o remetente de testes da própria Resend
// (onboarding@resend.dev), que funciona sem verificar domínio mas só entrega para o
// e-mail da conta Resend enquanto nenhum domínio próprio for verificado.
func NewResendClient(apiKey, from string) *ResendClient {
	return &ResendClient{
		apiKey:     apiKey,
		from:       from,
		apiURL:     defaultAPIURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *ResendClient) Configured() bool {
	return c.apiKey != ""
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type sendResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Name    string `json:"name"`
}

func (c *ResendClient) Send(ctx context.Context, to, subject, htmlBody string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(sendRequest{
		From:    c.from,
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao chamar a API da Resend: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var parsed sendResponse
		_ = json.Unmarshal(body, &parsed)
		if parsed.Message != "" {
			return fmt.Errorf("Resend retornou erro (%d): %s", resp.StatusCode, parsed.Message)
		}
		return fmt.Errorf("Resend retornou status %d", resp.StatusCode)
	}

	return nil
}
