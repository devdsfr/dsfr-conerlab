package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeagueRepo struct {
	db *pgxpool.Pool
}

func NewLeagueRepo(db *pgxpool.Pool) *LeagueRepo {
	return &LeagueRepo{db: db}
}

func (r *LeagueRepo) List(ctx context.Context) ([]domain.League, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, country, tier, created_at FROM leagues ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leagues []domain.League
	for rows.Next() {
		var l domain.League
		if err := rows.Scan(&l.ID, &l.Name, &l.Country, &l.Tier, &l.CreatedAt); err != nil {
			return nil, err
		}
		leagues = append(leagues, l)
	}
	return leagues, rows.Err()
}

func (r *LeagueRepo) GetByID(ctx context.Context, id int64) (*domain.League, error) {
	var l domain.League
	err := r.db.QueryRow(ctx, `SELECT id, name, country, tier, created_at FROM leagues WHERE id=$1`, id).
		Scan(&l.ID, &l.Name, &l.Country, &l.Tier, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LeagueRepo) ListSeasons(ctx context.Context, leagueID int64) ([]domain.Season, error) {
	rows, err := r.db.Query(ctx, `SELECT id, league_id, year, label FROM seasons WHERE league_id=$1 ORDER BY year DESC`, leagueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []domain.Season
	for rows.Next() {
		var s domain.Season
		if err := rows.Scan(&s.ID, &s.LeagueID, &s.Year, &s.Label); err != nil {
			return nil, err
		}
		seasons = append(seasons, s)
	}
	return seasons, rows.Err()
}
