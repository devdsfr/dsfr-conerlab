// Package diagnostics fornece o usecase por trás do painel "Integrações": mostra se as
// chaves de API (OpenAI, API-Football, SportMonks) estão configuradas, agrega o
// histórico de consumo registrado via internal/usagelog, e permite disparar uma
// chamada mínima e real a cada provedor para validar a chave ("Testar agora").
package diagnostics

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/llm"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/sportmonks"
	"github.com/devdsfr/cornerlab/internal/usagelog"
)

// UsageRepo é o contrato de leitura necessário deste usecase (implementado por
// internal/repository/postgres.UsageRepo).
type UsageRepo interface {
	Stats(ctx context.Context, provider usagelog.Provider) (usagelog.ProviderStats, error)
	Recent(ctx context.Context, provider *usagelog.Provider, limit int) ([]usagelog.Entry, error)
}

// ProviderSummary é o retorno JSON-friendly de um provedor no painel "Integrações".
type ProviderSummary struct {
	Provider         string                `json:"provider"`
	DisplayName      string                `json:"display_name"`
	Configured       bool                  `json:"configured"`
	TotalCalls       int                   `json:"total_calls"`
	SuccessCalls     int                   `json:"success_calls"`
	ErrorCalls       int                   `json:"error_calls"`
	TokensTotal      int                   `json:"tokens_total"`
	LastCallAt       *time.Time            `json:"last_call_at"`
	LastSuccessAt    *time.Time            `json:"last_success_at"`
	LastErrorAt      *time.Time            `json:"last_error_at"`
	LastErrorMessage string                `json:"last_error_message"`
	DailyCalls       []usagelog.DailyCount `json:"daily_calls"`
}

// TestResult é o retorno de uma chamada "Testar agora".
type TestResult struct {
	Provider  string `json:"provider"`
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	LatencyMs int    `json:"latency_ms"`
}

// EntryView é a versão JSON-friendly de um usagelog.Entry (histórico recente).
type EntryView struct {
	Provider     string    `json:"provider"`
	Endpoint     string    `json:"endpoint"`
	Success      bool      `json:"success"`
	StatusCode   *int      `json:"status_code"`
	TokensTotal  *int      `json:"tokens_total"`
	ErrorMessage string    `json:"error_message"`
	DurationMs   int       `json:"duration_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type Usecase struct {
	usage UsageRepo

	openaiConfigured      bool
	apiFootballConfigured bool
	sportMonksConfigured  bool

	openaiClient      *llm.OpenAIClient
	apiFootballClient *apifootball.Client
	sportMonksClient  *sportmonks.Client
}

func New(
	usage UsageRepo,
	openaiClient *llm.OpenAIClient, openaiConfigured bool,
	apiFootballClient *apifootball.Client, apiFootballConfigured bool,
	sportMonksClient *sportmonks.Client, sportMonksConfigured bool,
) *Usecase {
	return &Usecase{
		usage:                 usage,
		openaiClient:          openaiClient,
		apiFootballClient:     apiFootballClient,
		sportMonksClient:      sportMonksClient,
		openaiConfigured:      openaiConfigured,
		apiFootballConfigured: apiFootballConfigured,
		sportMonksConfigured:  sportMonksConfigured,
	}
}

var displayNames = map[usagelog.Provider]string{
	usagelog.ProviderOpenAI:      "OpenAI",
	usagelog.ProviderAPIFootball: "API-Football",
	usagelog.ProviderSportMonks:  "SportMonks",
}

// Summary retorna o status + histórico agregado dos três provedores, na ordem
// OpenAI, API-Football, SportMonks.
func (u *Usecase) Summary(ctx context.Context) ([]ProviderSummary, error) {
	providers := []struct {
		id         usagelog.Provider
		configured bool
	}{
		{usagelog.ProviderOpenAI, u.openaiConfigured},
		{usagelog.ProviderAPIFootball, u.apiFootballConfigured},
		{usagelog.ProviderSportMonks, u.sportMonksConfigured},
	}

	summaries := make([]ProviderSummary, 0, len(providers))
	for _, p := range providers {
		stats, err := u.usage.Stats(ctx, p.id)
		if err != nil {
			return nil, fmt.Errorf("erro ao consultar histórico de %s: %w", p.id, err)
		}
		summaries = append(summaries, ProviderSummary{
			Provider:         string(p.id),
			DisplayName:      displayNames[p.id],
			Configured:       p.configured,
			TotalCalls:       stats.TotalCalls,
			SuccessCalls:     stats.SuccessCalls,
			ErrorCalls:       stats.ErrorCalls,
			TokensTotal:      stats.TokensTotal,
			LastCallAt:       stats.LastCallAt,
			LastSuccessAt:    stats.LastSuccessAt,
			LastErrorAt:      stats.LastErrorAt,
			LastErrorMessage: stats.LastErrorMessage,
			DailyCalls:       stats.DailyCalls,
		})
	}
	return summaries, nil
}

// Recent retorna o histórico recente de chamadas, opcionalmente filtrado por provedor.
func (u *Usecase) Recent(ctx context.Context, provider string, limit int) ([]EntryView, error) {
	var providerFilter *usagelog.Provider
	if provider != "" {
		p := usagelog.Provider(provider)
		providerFilter = &p
	}
	entries, err := u.usage.Recent(ctx, providerFilter, limit)
	if err != nil {
		return nil, err
	}
	views := make([]EntryView, 0, len(entries))
	for _, e := range entries {
		views = append(views, EntryView{
			Provider:     string(e.Provider),
			Endpoint:     e.Endpoint,
			Success:      e.Success,
			StatusCode:   e.StatusCode,
			TokensTotal:  e.TokensTotal,
			ErrorMessage: e.ErrorMessage,
			DurationMs:   e.DurationMs,
			CreatedAt:    e.CreatedAt,
		})
	}
	return views, nil
}

// TestConnection faz uma chamada mínima e real ao provedor informado ("openai",
// "api_football" ou "sportmonks") para validar que a chave configurada funciona. O
// resultado (sucesso ou erro) também fica registrado no histórico de uso, como
// qualquer outra chamada real feita à API externa.
func (u *Usecase) TestConnection(ctx context.Context, provider string) (TestResult, error) {
	start := time.Now()
	result := TestResult{Provider: provider}

	var err error
	switch usagelog.Provider(provider) {
	case usagelog.ProviderOpenAI:
		if !u.openaiConfigured {
			return TestResult{}, fmt.Errorf("OPENAI_API_KEY não configurada")
		}
		err = u.openaiClient.TestConnection(ctx)
	case usagelog.ProviderAPIFootball:
		if !u.apiFootballConfigured {
			return TestResult{}, fmt.Errorf("API_FOOTBALL_KEY não configurada")
		}
		err = u.apiFootballClient.TestConnection(ctx)
	case usagelog.ProviderSportMonks:
		if !u.sportMonksConfigured {
			return TestResult{}, fmt.Errorf("SPORTMONKS_KEY não configurada")
		}
		err = u.sportMonksClient.TestConnection(ctx)
	default:
		return TestResult{}, fmt.Errorf("provedor desconhecido: %s (use openai, api_football ou sportmonks)", provider)
	}

	result.LatencyMs = int(time.Since(start).Milliseconds())
	if err != nil {
		result.OK = false
		result.Message = err.Error()
		return result, nil
	}
	result.OK = true
	result.Message = "conexão validada com sucesso"
	return result, nil
}
