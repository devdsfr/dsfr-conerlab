package postgres

import (
	"context"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/usagelog"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UsageRepo persiste e consulta o histórico de chamadas às APIs externas (OpenAI,
// API-Football, SportMonks). Implementa usagelog.Recorder.
type UsageRepo struct {
	db *pgxpool.Pool
}

func NewUsageRepo(db *pgxpool.Pool) *UsageRepo {
	return &UsageRepo{db: db}
}

func (r *UsageRepo) Record(ctx context.Context, e usagelog.Entry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO api_usage_log
			(provider, endpoint, success, status_code, tokens_prompt, tokens_completion, tokens_total, error_message, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		string(e.Provider), e.Endpoint, e.Success, e.StatusCode,
		e.TokensPrompt, e.TokensCompletion, e.TokensTotal, e.ErrorMessage, e.DurationMs)
	return err
}

// Stats agrega o histórico de um provedor: totais de chamadas, tokens consumidos
// (quando aplicável, ex: OpenAI), últimos horários de sucesso/erro e a série diária dos
// últimos 7 dias (para o gráfico de consumo do painel "Integrações").
func (r *UsageRepo) Stats(ctx context.Context, provider usagelog.Provider) (usagelog.ProviderStats, error) {
	stats := usagelog.ProviderStats{Provider: provider}

	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE success)::int,
			COUNT(*) FILTER (WHERE NOT success)::int,
			COALESCE(SUM(tokens_total), 0)::int,
			MAX(created_at),
			MAX(created_at) FILTER (WHERE success),
			MAX(created_at) FILTER (WHERE NOT success)
		FROM api_usage_log
		WHERE provider = $1`, string(provider)).
		Scan(&stats.TotalCalls, &stats.SuccessCalls, &stats.ErrorCalls, &stats.TokensTotal,
			&stats.LastCallAt, &stats.LastSuccessAt, &stats.LastErrorAt)
	if err != nil {
		return stats, err
	}

	if stats.LastErrorAt != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(error_message, '')
			FROM api_usage_log
			WHERE provider = $1 AND NOT success
			ORDER BY created_at DESC LIMIT 1`, string(provider)).Scan(&stats.LastErrorMessage); err != nil {
			return stats, err
		}
	}

	rows, err := r.db.Query(ctx, `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS day, COUNT(*)::int
		FROM api_usage_log
		WHERE provider = $1 AND created_at >= now() - interval '7 days'
		GROUP BY day
		ORDER BY day`, string(provider))
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var dc usagelog.DailyCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return stats, err
		}
		stats.DailyCalls = append(stats.DailyCalls, dc)
	}
	return stats, rows.Err()
}

// Recent retorna os registros mais recentes, opcionalmente filtrados por provedor.
func (r *UsageRepo) Recent(ctx context.Context, provider *usagelog.Provider, limit int) ([]usagelog.Entry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT provider, endpoint, success, status_code, tokens_prompt, tokens_completion, tokens_total,
		       COALESCE(error_message, ''), duration_ms, created_at
		FROM api_usage_log`
	args := []any{}
	if provider != nil {
		query += ` WHERE provider = $1`
		args = append(args, string(*provider))
	}
	query += ` ORDER BY created_at DESC LIMIT ` + strconv.Itoa(limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []usagelog.Entry
	for rows.Next() {
		var e usagelog.Entry
		var provStr string
		if err := rows.Scan(&provStr, &e.Endpoint, &e.Success, &e.StatusCode,
			&e.TokensPrompt, &e.TokensCompletion, &e.TokensTotal, &e.ErrorMessage, &e.DurationMs, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Provider = usagelog.Provider(provStr)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
