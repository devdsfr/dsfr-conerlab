package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertRuleRepo struct {
	db *pgxpool.Pool
}

func NewAlertRuleRepo(db *pgxpool.Pool) *AlertRuleRepo {
	return &AlertRuleRepo{db: db}
}

func (r *AlertRuleRepo) Create(ctx context.Context, rule *domain.AlertRule) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO alert_rules (user_id, name, definition, active)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id, created_at`, rule.UserID, rule.Name, rule.Definition, rule.Active).
		Scan(&rule.ID, &rule.CreatedAt)
}

func (r *AlertRuleRepo) List(ctx context.Context, userID int64) ([]domain.AlertRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, definition::text, active, created_at
		FROM alert_rules WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.AlertRule
	for rows.Next() {
		var a domain.AlertRule
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Definition, &a.Active, &a.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, a)
	}
	return rules, rows.Err()
}

func (r *AlertRuleRepo) GetByID(ctx context.Context, id int64) (*domain.AlertRule, error) {
	var a domain.AlertRule
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, definition::text, active, created_at
		FROM alert_rules WHERE id=$1`, id).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Definition, &a.Active, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AlertRuleRepo) Delete(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}
