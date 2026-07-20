// Package usagelog define o vocabulário compartilhado para registrar e consultar o
// consumo das integrações externas (OpenAI, API-Football, SportMonks). Os clientes de
// integração (internal/integration/...) gravam um Entry a cada chamada real feita à
// API externa; o painel "Integrações" do frontend lê esse histórico agregado via
// internal/usecase/diagnostics.
package usagelog

import (
	"context"
	"log/slog"
	"time"
)

// Provider identifica a integração externa que originou o registro de uso.
type Provider string

const (
	ProviderOpenAI      Provider = "openai"
	ProviderAPIFootball Provider = "api_football"
	ProviderSportMonks  Provider = "sportmonks"
)

// Entry é um registro de uma chamada feita a uma API externa.
type Entry struct {
	Provider         Provider
	Endpoint         string
	Success          bool
	StatusCode       *int
	TokensPrompt     *int
	TokensCompletion *int
	TokensTotal      *int
	ErrorMessage     string
	DurationMs       int
	CreatedAt        time.Time
}

// DailyCount é a contagem de chamadas em um dia (usado para o gráfico de consumo).
// Precisa das tags json em minúsculo — sem elas, cada item serializava como
// {"Date":..., "Count":...} (maiúsculo), enquanto o frontend espera {"date":...,
// "count":...}, quebrando o painel Integrações assim que havia alguma chamada
// registrada (chartLabels: dailyCalls.map(d => d.date.slice(5)) lia d.date
// undefined).
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// ProviderStats agrega o histórico de uso de um provedor específico.
type ProviderStats struct {
	Provider         Provider
	TotalCalls       int
	SuccessCalls     int
	ErrorCalls       int
	TokensTotal      int
	LastCallAt       *time.Time
	LastSuccessAt    *time.Time
	LastErrorAt      *time.Time
	LastErrorMessage string
	DailyCalls       []DailyCount
}

// Recorder persiste registros de uso. Implementado por
// internal/repository/postgres.UsageRepo.
type Recorder interface {
	Record(ctx context.Context, e Entry) error
}

// RecordAsync grava o registro de uso em uma goroutine separada, com timeout curto,
// para nunca adicionar latência perceptível (nem risco de erro) à chamada real feita à
// API externa. Se recorder for nil, não faz nada.
func RecordAsync(recorder Recorder, e Entry) {
	if recorder == nil {
		return
	}
	e.CreatedAt = time.Now()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic ao registrar uso de API externa", "provider", e.Provider, "error", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := recorder.Record(ctx, e); err != nil {
			slog.Error("falha ao gravar log de uso de API externa", "provider", e.Provider, "error", err)
		}
	}()
}
