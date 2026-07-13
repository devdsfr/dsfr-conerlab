package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StrategyHistoryRepo struct {
	db *pgxpool.Pool
}

func NewStrategyHistoryRepo(db *pgxpool.Pool) *StrategyHistoryRepo {
	return &StrategyHistoryRepo{db: db}
}

func (r *StrategyHistoryRepo) Create(ctx context.Context, e *domain.StrategyHistoryEntry) error {
	// season_ids é bigint[] NOT NULL no schema; um slice nil em Go é enviado como
	// NULL pelo driver, o que viola a constraint. Normaliza para slice vazio.
	seasonIDs := e.SeasonIDs
	if seasonIDs == nil {
		seasonIDs = []int64{}
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO filter_backtests (user_id, saved_filter_id, definition, league_id, season_ids, result)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6::jsonb)
		RETURNING id, created_at`,
		e.UserID, e.SavedFilterID, e.Definition, e.LeagueID, seasonIDs, e.Result).
		Scan(&e.ID, &e.CreatedAt)
}

func (r *StrategyHistoryRepo) List(ctx context.Context, userID int64) ([]domain.StrategyHistoryEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, saved_filter_id, definition::text, league_id, season_ids, result::text, created_at
		FROM filter_backtests WHERE user_id=$1 ORDER BY created_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.StrategyHistoryEntry
	for rows.Next() {
		var e domain.StrategyHistoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.SavedFilterID, &e.Definition, &e.LeagueID, &e.SeasonIDs, &e.Result, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *StrategyHistoryRepo) TopPerforming(ctx context.Context, limit int) ([]domain.StrategyHistoryEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, saved_filter_id, definition::text, league_id, season_ids, result::text, created_at
		FROM filter_backtests
		WHERE (result->>'roi') IS NOT NULL
		ORDER BY (result->>'roi')::float DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.StrategyHistoryEntry
	for rows.Next() {
		var e domain.StrategyHistoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.SavedFilterID, &e.Definition, &e.LeagueID, &e.SeasonIDs, &e.Result, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
