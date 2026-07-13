package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FilterRepo struct {
	db *pgxpool.Pool
}

func NewFilterRepo(db *pgxpool.Pool) *FilterRepo {
	return &FilterRepo{db: db}
}

func (r *FilterRepo) Create(ctx context.Context, f *domain.SavedFilter) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO saved_filters (user_id, name, description, definition)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, created_at, updated_at`,
		f.UserID, f.Name, f.Description, f.Definition).
		Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
}

func (r *FilterRepo) List(ctx context.Context, userID int64) ([]domain.SavedFilter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, description, definition::text, created_at, updated_at
		FROM saved_filters WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []domain.SavedFilter
	for rows.Next() {
		var f domain.SavedFilter
		if err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.Description, &f.Definition, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

func (r *FilterRepo) GetByID(ctx context.Context, id int64) (*domain.SavedFilter, error) {
	var f domain.SavedFilter
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, description, definition::text, created_at, updated_at
		FROM saved_filters WHERE id=$1`, id).
		Scan(&f.ID, &f.UserID, &f.Name, &f.Description, &f.Definition, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FilterRepo) Update(ctx context.Context, f *domain.SavedFilter) error {
	return r.db.QueryRow(ctx, `
		UPDATE saved_filters SET name=$1, description=$2, definition=$3::jsonb, updated_at=now()
		WHERE id=$4 AND user_id=$5
		RETURNING updated_at`, f.Name, f.Description, f.Definition, f.ID, f.UserID).
		Scan(&f.UpdatedAt)
}

func (r *FilterRepo) Delete(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM saved_filters WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}
