package postgres

import (
	"context"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

const userBillingColumns = `id, name, email, password_hash, created_at, plan, stripe_customer_id, stripe_subscription_id, subscription_status, trial_ends_at, current_period_end`

func scanUser(row interface{ Scan(...any) error }, u *domain.User) error {
	return row.Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt,
		&u.Plan, &u.StripeCustomerID, &u.StripeSubscriptionID, &u.SubscriptionStatus,
		&u.TrialEndsAt, &u.CurrentPeriodEnd,
	)
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
		RETURNING id, created_at, plan, subscription_status`, u.Name, u.Email, u.PasswordHash).
		Scan(&u.ID, &u.CreatedAt, &u.Plan, &u.SubscriptionStatus)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	row := r.db.QueryRow(ctx, `SELECT `+userBillingColumns+` FROM users WHERE email=$1`, email)
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	row := r.db.QueryRow(ctx, `SELECT `+userBillingColumns+` FROM users WHERE id=$1`, id)
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByStripeCustomerID(ctx context.Context, customerID string) (*domain.User, error) {
	var u domain.User
	row := r.db.QueryRow(ctx, `SELECT `+userBillingColumns+` FROM users WHERE stripe_customer_id=$1`, customerID)
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) SetStripeCustomerID(ctx context.Context, userID int64, customerID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET stripe_customer_id=$1 WHERE id=$2`, customerID, userID)
	return err
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, passwordHash, userID)
	return err
}

func (r *UserRepo) UpdateSubscriptionByCustomerID(ctx context.Context, customerID, subscriptionID, status, plan string, trialEndsAt, currentPeriodEnd *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			stripe_subscription_id=$1,
			subscription_status=$2,
			plan=$3,
			trial_ends_at=$4,
			current_period_end=$5
		WHERE stripe_customer_id=$6`,
		subscriptionID, status, plan, trialEndsAt, currentPeriodEnd, customerID)
	return err
}
