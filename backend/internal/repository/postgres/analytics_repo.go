package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// AnalyticsRepo persiste a camada ANALYTICS (Remodelagem F2/F3 — migration 011).
// Regra do doc 15: só workers escrevem aqui; handlers HTTP apenas leem.
type AnalyticsRepo struct {
	db *pgxpool.Pool
}

func NewAnalyticsRepo(db *pgxpool.Pool) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

// ListLeagueSeasons lista os pares liga/temporada que têm partidas FINALIZADO em
// ligas reais (external_id preenchido — mesmo guardrail do catálogo de ligas).
func (r *AnalyticsRepo) ListLeagueSeasons(ctx context.Context) ([]repository.LeagueSeason, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT m.league_id, m.season_id
		FROM matches m
		JOIN leagues l ON l.id = m.league_id
		WHERE m.status = 'FINALIZADO' AND l.external_id IS NOT NULL
		ORDER BY m.league_id, m.season_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.LeagueSeason
	for rows.Next() {
		var ls repository.LeagueSeason
		if err := rows.Scan(&ls.LeagueID, &ls.SeasonID); err != nil {
			return nil, err
		}
		out = append(out, ls)
	}
	return out, rows.Err()
}

// UpsertTeamMetrics grava (ou atualiza) o agregado pré-calculado de uma
// equipe/temporada/métrica. Idempotente — reprocessar nunca duplica.
func (r *AnalyticsRepo) UpsertTeamMetrics(ctx context.Context, m *domain.TeamMetrics) error {
	freqs, err := json.Marshal(m.Frequencies)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO team_metrics (
			team_id, season_id, league_id, metric, sample_size,
			avg_total, avg_for, avg_against, avg_home, avg_away,
			last5_avg, last10_avg, last20_avg,
			variance, std_dev, consistency, trend, frequencies,
			algorithm_version, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19, now())
		ON CONFLICT (team_id, season_id, metric) DO UPDATE SET
			league_id = EXCLUDED.league_id,
			sample_size = EXCLUDED.sample_size,
			avg_total = EXCLUDED.avg_total,
			avg_for = EXCLUDED.avg_for,
			avg_against = EXCLUDED.avg_against,
			avg_home = EXCLUDED.avg_home,
			avg_away = EXCLUDED.avg_away,
			last5_avg = EXCLUDED.last5_avg,
			last10_avg = EXCLUDED.last10_avg,
			last20_avg = EXCLUDED.last20_avg,
			variance = EXCLUDED.variance,
			std_dev = EXCLUDED.std_dev,
			consistency = EXCLUDED.consistency,
			trend = EXCLUDED.trend,
			frequencies = EXCLUDED.frequencies,
			algorithm_version = EXCLUDED.algorithm_version,
			updated_at = now()`,
		m.TeamID, m.SeasonID, m.LeagueID, m.Metric, m.SampleSize,
		m.AvgTotal, m.AvgFor, m.AvgAgainst, m.AvgHome, m.AvgAway,
		m.Last5Avg, m.Last10Avg, m.Last20Avg,
		m.Variance, m.StdDev, m.Consistency, m.Trend, freqs,
		m.AlgorithmVersion)
	return err
}

// StartWorkerRun abre o registro de observabilidade de uma execução (doc 15:
// cada worker registra tempo, quantidade processada e erros).
func (r *AnalyticsRepo) StartWorkerRun(ctx context.Context, worker string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO worker_runs (worker, status) VALUES ($1, 'running') RETURNING id`,
		worker).Scan(&id)
	return id, err
}

// FinishWorkerRun fecha o registro com status/estatísticas finais.
func (r *AnalyticsRepo) FinishWorkerRun(ctx context.Context, id int64, status string, processed, errCount int, started time.Time, details map[string]any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		raw = []byte("{}")
	}
	_, err = r.db.Exec(ctx, `
		UPDATE worker_runs
		SET status=$2, finished_at=now(), duration_ms=$3, processed=$4, errors=$5, details=$6
		WHERE id=$1`,
		id, status, time.Since(started).Milliseconds(), processed, errCount, raw)
	return err
}
