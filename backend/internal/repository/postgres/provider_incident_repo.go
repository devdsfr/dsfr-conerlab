package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderIncidentRepo grava e consulta a saúde dos provedores de dados (ver
// migrations/004_statistics_sync.sql e internal/usecase/statsync/healthcheck.go).
// Enquanto existir um incidente aberto (resolved_at IS NULL) para um provedor, os
// Workers de Descoberta/Atualização pulam o ciclo daquele provedor — sem apagar dados,
// sem derrubar a API, só evitando bater numa fonte que já se mostrou instável no
// último check horário.
type ProviderIncidentRepo struct {
	db *pgxpool.Pool
}

func NewProviderIncidentRepo(db *pgxpool.Pool) *ProviderIncidentRepo {
	return &ProviderIncidentRepo{db: db}
}

func (r *ProviderIncidentRepo) Open(ctx context.Context, provider, checkType, message string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO provider_incidents (provider, check_type, message)
		VALUES ($1, $2, $3)`, provider, checkType, message)
	return err
}

// ResolveOpen marca todos os incidentes abertos de um provedor como resolvidos.
// Chamado assim que um health check volta a passar, para o provedor sair do estado de
// "sincronização suspensa" automaticamente.
func (r *ProviderIncidentRepo) ResolveOpen(ctx context.Context, provider string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE provider_incidents SET resolved_at = now()
		WHERE provider = $1 AND resolved_at IS NULL`, provider)
	return err
}

func (r *ProviderIncidentRepo) IsSuspended(ctx context.Context, provider string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM provider_incidents WHERE provider = $1 AND resolved_at IS NULL)`,
		provider).Scan(&exists)
	return exists, err
}

// ProviderIncident é usado tanto pelo alerta ao administrador (log estruturado) quanto,
// na fase 2 deste módulo, pelo endpoint do Dashboard Administrativo.
type ProviderIncident struct {
	ID         int64
	Provider   string
	CheckType  string
	Message    string
	ResolvedAt *string
	CreatedAt  string
}

// RecentIncidents lista os últimos incidentes registrados (resolvidos ou não) de um
// provedor — usado para o painel de diagnóstico e para depuração manual.
func (r *ProviderIncidentRepo) Recent(ctx context.Context, provider string, limit int) ([]ProviderIncident, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, provider, check_type, message,
			resolved_at::text, created_at::text
		FROM provider_incidents
		WHERE provider = $1
		ORDER BY created_at DESC
		LIMIT $2`, provider, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	incidents := []ProviderIncident{}
	for rows.Next() {
		var inc ProviderIncident
		var resolvedAt *string
		if err := rows.Scan(&inc.ID, &inc.Provider, &inc.CheckType, &inc.Message, &resolvedAt, &inc.CreatedAt); err != nil {
			return nil, err
		}
		inc.ResolvedAt = resolvedAt
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}
