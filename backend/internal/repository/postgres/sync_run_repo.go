package postgres

import (
	"context"
	"errors"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncRunRepo implementa repository.SyncRunRepository.
type SyncRunRepo struct {
	db *pgxpool.Pool
}

func NewSyncRunRepo(db *pgxpool.Pool) *SyncRunRepo {
	return &SyncRunRepo{db: db}
}

func (r *SyncRunRepo) AddRun(ctx context.Context, e *domain.SyncRun) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO sync_runs
			(triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
		e.TriggeredBy, e.Targets, e.FixturesFound, e.FixturesUpserted, e.MatchesChecked, e.MatchesFinalized, e.Errors, e.DurationMs).
		Scan(&e.ID, &e.CreatedAt)
}

func (r *SyncRunRepo) LastRun(ctx context.Context) (*domain.SyncRun, error) {
	var e domain.SyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms, created_at
		FROM sync_runs ORDER BY created_at DESC LIMIT 1`).
		Scan(&e.ID, &e.TriggeredBy, &e.Targets, &e.FixturesFound, &e.FixturesUpserted, &e.MatchesChecked, &e.MatchesFinalized, &e.Errors, &e.DurationMs, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
