package statsync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
)

// HealthCheckUsecase é o Worker de Health Check, adicionado por pedido explícito do
// usuário por causa da dependência de um provedor sem API oficial (Sofascore) ou, na
// configuração atual, de uma API gratuita com cota e regras que podem mudar
// (API-Football). Roda de hora em hora e verifica:
//   - o endpoint do provedor ainda responde;
//   - não houve bloqueio (403/429) — StatisticsProvider.HealthCheck já captura isso;
//   - o tempo de resposta está dentro do esperado;
//   - o formato da resposta continua o esperado (parse não falha).
//
// A verificação de que "todos os campos obrigatórios continuam existindo" (escanteios,
// posse, cartões etc.) acontece de forma contínua como efeito colateral real do Worker
// de Atualização: cada chamada de SyncFixtureStatistics que não encontra os campos
// esperados já é, ela mesma, uma falha de conteúdo — não dá pra simular isso aqui sem
// gastar cota de API só para checar, então o health check horário cobre a camada de
// transporte (disponibilidade/latência/formato) e o Worker de Atualização cobre a
// camada de conteúdo, ambos alimentando o mesmo registro de incidentes.
type HealthCheckUsecase struct {
	provider  statsprovider.StatisticsProvider
	incidents *postgres.ProviderIncidentRepo
}

func NewHealthCheckUsecase(provider statsprovider.StatisticsProvider, incidents *postgres.ProviderIncidentRepo) *HealthCheckUsecase {
	return &HealthCheckUsecase{provider: provider, incidents: incidents}
}

func (u *HealthCheckUsecase) Run(ctx context.Context) (statsprovider.HealthResult, error) {
	result, err := u.provider.HealthCheck(ctx)
	if err != nil {
		// Erro na própria checagem (ex: contexto cancelado) — não é um veredito sobre
		// a saúde do provedor, só não conseguimos avaliar desta vez. Não abre
		// incidente, tenta de novo no próximo ciclo.
		return result, fmt.Errorf("erro ao executar health check: %w", err)
	}

	if !result.OK {
		if err := u.incidents.Open(ctx, u.provider.Name(), "endpoint", result.Message); err != nil {
			slog.Error("falha ao registrar incidente de saúde do provedor", "provider", u.provider.Name(), "error", err)
		}
		// "Notificação para o administrador": registrada como log estruturado de nível
		// Error (visível nos logs do serviço, ex: Render) e como incidente
		// consultável no Postgres — a fase 2 deste módulo expõe isso num endpoint do
		// Dashboard Administrativo. Os Workers de Descoberta/Atualização já checam
		// IsSuspended antes de cada ciclo e param de bater no provedor sozinhos.
		slog.Error("health check do provedor falhou — sincronização suspensa até novo check passar",
			"provider", u.provider.Name(), "response_time_ms", result.ResponseTime.Milliseconds(), "message", result.Message)
		return result, nil
	}

	if err := u.incidents.ResolveOpen(ctx, u.provider.Name()); err != nil {
		slog.Error("falha ao resolver incidentes de saúde do provedor", "provider", u.provider.Name(), "error", err)
	}
	slog.Info("health check do provedor ok", "provider", u.provider.Name(), "response_time_ms", result.ResponseTime.Milliseconds())
	return result, nil
}
