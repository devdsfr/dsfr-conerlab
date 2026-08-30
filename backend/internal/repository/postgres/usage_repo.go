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
	// DailyCalls precisa começar como [] (não nil) — se o provedor não teve
	// nenhuma chamada nos últimos 7 dias o loop abaixo nunca roda, e um slice nil
	// vira `null` no JSON, quebrando o .map() do frontend (ver integrations.component.ts).
	stats := usagelog.ProviderStats{Provider: provider, DailyCalls: []usagelog.DailyCount{}}

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

// UsageLogRetentionDays define por quanto tempo o histórico de chamadas é mantido.
//
// api_usage_log é a ÚNICA tabela do sistema que cresce sem teto: uma linha por
// chamada externa, ~400 bytes cada, e nada nunca a limpa. As demais crescem com o
// calendário (uma linha por partida) e são pequenas — o banco inteiro tem 14 MB.
// Com o worker diário são ~65 registros/dia, ou ~10 MB/ano, então 90 dias mantêm o
// gráfico de 7 dias e a auditoria recente sem deixar a tabela virar um problema
// daqui a alguns anos.
const UsageLogRetentionDays = 90

// PurgeOldUsage apaga registros de uso mais antigos que UsageLogRetentionDays e
// devolve quantas linhas saíram. Seguro rodar a qualquer momento: o painel
// "Integrações" só consulta os últimos 7 dias, e o histórico recente é limitado a
// 200 registros.
func (r *UsageRepo) PurgeOldUsage(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM api_usage_log
		WHERE created_at < now() - make_interval(days => $1)`, UsageLogRetentionDays)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Storage descreve o consumo de armazenamento do banco, para o card de
// "Armazenamento" do painel Integrações.
func (r *UsageRepo) Storage(ctx context.Context) (usagelog.StorageStats, error) {
	var s usagelog.StorageStats
	s.RetentionDays = UsageLogRetentionDays

	if err := r.db.QueryRow(ctx, `
		SELECT pg_database_size(current_database()),
		       pg_size_pretty(pg_database_size(current_database()))`).
		Scan(&s.TotalBytes, &s.TotalPretty); err != nil {
		return s, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT relname,
		       pg_total_relation_size(relid),
		       pg_size_pretty(pg_total_relation_size(relid)),
		       n_live_tup
		FROM pg_stat_user_tables
		ORDER BY pg_total_relation_size(relid) DESC
		LIMIT 5`)
	if err != nil {
		return s, err
	}
	defer rows.Close()

	s.Tables = []usagelog.TableSize{}
	for rows.Next() {
		var t usagelog.TableSize
		if err := rows.Scan(&t.Name, &t.Bytes, &t.Pretty, &t.Rows); err != nil {
			return s, err
		}
		s.Tables = append(s.Tables, t)
	}
	return s, rows.Err()
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

	entries := []usagelog.Entry{} // idem: nunca nil, senão vira `null` no JSON
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
