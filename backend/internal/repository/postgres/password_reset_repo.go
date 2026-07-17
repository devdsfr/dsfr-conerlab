package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PasswordResetRepo struct {
	db *pgxpool.Pool
}

func NewPasswordResetRepo(db *pgxpool.Pool) *PasswordResetRepo {
	return &PasswordResetRepo{db: db}
}

func (r *PasswordResetRepo) Create(ctx context.Context, t *domain.PasswordResetToken) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO password_reset_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`, t.UserID, t.Token, t.ExpiresAt).
		Scan(&t.ID, &t.CreatedAt)
}

func (r *PasswordResetRepo) GetValidByToken(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token=$1 AND used_at IS NULL AND expires_at > now()`, token).
		Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PasswordResetRepo) MarkUsed(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE password_reset_tokens SET used_at = now() WHERE id=$1`, id)
	return err
}

func (r *PasswordResetRepo) InvalidateAllForUser(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE password_reset_tokens SET used_at = now() WHERE user_id=$1 AND used_at IS NULL`, userID)
	return err
}
