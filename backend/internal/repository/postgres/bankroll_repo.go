package postgres

import (
	"context"
	"errors"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BankrollRepo implementa repository.BankrollRepository (Módulo de Gestão Evolutiva
// de Banca).
type BankrollRepo struct {
	db *pgxpool.Pool
}

func NewBankrollRepo(db *pgxpool.Pool) *BankrollRepo {
	return &BankrollRepo{db: db}
}

func (r *BankrollRepo) ListPhases(ctx context.Context, userID int64) ([]domain.BankrollPhase, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, sequence, name, amount
		FROM bankroll_phases WHERE user_id=$1 ORDER BY sequence`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phases []domain.BankrollPhase
	for rows.Next() {
		var p domain.BankrollPhase
		if err := rows.Scan(&p.ID, &p.UserID, &p.Sequence, &p.Name, &p.Amount); err != nil {
			return nil, err
		}
		phases = append(phases, p)
	}
	return phases, rows.Err()
}

func (r *BankrollRepo) ReplacePhases(ctx context.Context, userID int64, phases []domain.BankrollPhase) ([]domain.BankrollPhase, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM bankroll_phases WHERE user_id=$1`, userID); err != nil {
		return nil, err
	}
	for _, p := range phases {
		if _, err := tx.Exec(ctx, `
			INSERT INTO bankroll_phases (user_id, sequence, name, amount)
			VALUES ($1, $2, $3, $4)`, userID, p.Sequence, p.Name, p.Amount); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListPhases(ctx, userID)
}

func defaultCriteria(userID int64) domain.BankrollCriteria {
	return domain.BankrollCriteria{
		UserID: userID, MinDays: 90, MinBets: 100, MinWinRate: 80, MinROI: 10, MinYield: 5,
		RequirePositiveProfit: true, MinCompletedCycles: 20, CycleWinStreak: 3,
	}
}

func (r *BankrollRepo) GetCriteria(ctx context.Context, userID int64) (domain.BankrollCriteria, error) {
	var c domain.BankrollCriteria
	err := r.db.QueryRow(ctx, `
		SELECT user_id, min_days, min_bets, min_win_rate, min_roi, min_yield,
		       require_positive_profit, min_completed_cycles, cycle_win_streak
		FROM bankroll_criteria WHERE user_id=$1`, userID).
		Scan(&c.UserID, &c.MinDays, &c.MinBets, &c.MinWinRate, &c.MinROI, &c.MinYield,
			&c.RequirePositiveProfit, &c.MinCompletedCycles, &c.CycleWinStreak)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultCriteria(userID), nil
	}
	if err != nil {
		return domain.BankrollCriteria{}, err
	}
	return c, nil
}

func (r *BankrollRepo) SaveCriteria(ctx context.Context, c domain.BankrollCriteria) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO bankroll_criteria
			(user_id, min_days, min_bets, min_win_rate, min_roi, min_yield,
			 require_positive_profit, min_completed_cycles, cycle_win_streak, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		ON CONFLICT (user_id) DO UPDATE SET
			min_days=$2, min_bets=$3, min_win_rate=$4, min_roi=$5, min_yield=$6,
			require_positive_profit=$7, min_completed_cycles=$8, cycle_win_streak=$9, updated_at=now()`,
		c.UserID, c.MinDays, c.MinBets, c.MinWinRate, c.MinROI, c.MinYield,
		c.RequirePositiveProfit, c.MinCompletedCycles, c.CycleWinStreak)
	return err
}

func (r *BankrollRepo) GetState(ctx context.Context, userID int64) (*domain.BankrollState, error) {
	var s domain.BankrollState
	err := r.db.QueryRow(ctx, `
		SELECT user_id, current_phase_sequence, phase_started_at, highest_phase_sequence, promotions, demotions
		FROM bankroll_state WHERE user_id=$1`, userID).
		Scan(&s.UserID, &s.CurrentPhaseSequence, &s.PhaseStartedAt, &s.HighestPhaseSequence, &s.Promotions, &s.Demotions)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *BankrollRepo) InitState(ctx context.Context, userID int64) (*domain.BankrollState, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO bankroll_state (user_id, current_phase_sequence, phase_started_at, highest_phase_sequence)
		VALUES ($1, 1, now(), 1)
		ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return nil, err
	}
	return r.GetState(ctx, userID)
}

func (r *BankrollRepo) SetPhase(ctx context.Context, userID int64, newSequence int) (*domain.BankrollState, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE bankroll_state SET
			current_phase_sequence = $2,
			phase_started_at = now(),
			highest_phase_sequence = GREATEST(highest_phase_sequence, $2),
			promotions = promotions + CASE WHEN $2 > current_phase_sequence THEN 1 ELSE 0 END,
			demotions  = demotions  + CASE WHEN $2 < current_phase_sequence THEN 1 ELSE 0 END
		WHERE user_id=$1`, userID, newSequence)
	if err != nil {
		return nil, err
	}
	return r.GetState(ctx, userID)
}

func (r *BankrollRepo) AddHistory(ctx context.Context, e *domain.BankrollHistoryEntry) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO bankroll_history (user_id, from_amount, to_amount, direction, reason, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		e.UserID, e.FromAmount, e.ToAmount, e.Direction, e.Reason, e.Notes).
		Scan(&e.ID, &e.CreatedAt)
}

func (r *BankrollRepo) ListHistory(ctx context.Context, userID int64) ([]domain.BankrollHistoryEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, from_amount, to_amount, direction, reason, notes, created_at
		FROM bankroll_history WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.BankrollHistoryEntry
	for rows.Next() {
		var e domain.BankrollHistoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.FromAmount, &e.ToAmount, &e.Direction, &e.Reason, &e.Notes, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
