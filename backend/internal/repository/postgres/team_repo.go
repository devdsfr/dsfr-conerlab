package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamRepo struct {
	db *pgxpool.Pool
}

func NewTeamRepo(db *pgxpool.Pool) *TeamRepo {
	return &TeamRepo{db: db}
}

func (r *TeamRepo) List(ctx context.Context, leagueID *int64, seasonID *int64) ([]domain.Team, error) {
	query := `SELECT id, name, short_name, country, tier, created_at FROM teams ORDER BY name`
	args := []any{}
	switch {
	case leagueID != nil && seasonID != nil:
		// Restringe a equipes que de fato têm partida registrada nessa liga+temporada
		// — league_teams sozinho é um vínculo histórico "alguma vez jogou aqui" e
		// listava equipes de temporadas passadas (ex: rebaixadas) na temporada atual.
		query = `
			SELECT DISTINCT t.id, t.name, t.short_name, t.country, t.tier, t.created_at
			FROM teams t
			JOIN matches m ON (m.home_team_id = t.id OR m.away_team_id = t.id)
			WHERE m.league_id = $1 AND m.season_id = $2
			ORDER BY t.name`
		args = append(args, *leagueID, *seasonID)
	case leagueID != nil:
		query = `
			SELECT t.id, t.name, t.short_name, t.country, t.tier, t.created_at
			FROM teams t
			JOIN league_teams lt ON lt.team_id = t.id
			WHERE lt.league_id = $1
			ORDER BY t.name`
		args = append(args, *leagueID)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []domain.Team
	for rows.Next() {
		var t domain.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.ShortName, &t.Country, &t.Tier, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (r *TeamRepo) GetByID(ctx context.Context, id int64) (*domain.Team, error) {
	var t domain.Team
	err := r.db.QueryRow(ctx, `SELECT id, name, short_name, country, tier, created_at FROM teams WHERE id=$1`, id).
		Scan(&t.ID, &t.Name, &t.ShortName, &t.Country, &t.Tier, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TeamRepo) Search(ctx context.Context, query string) ([]domain.Team, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, short_name, country, tier, created_at
		FROM teams
		WHERE name ILIKE '%' || $1 || '%' OR short_name ILIKE '%' || $1 || '%'
		ORDER BY name LIMIT 20`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []domain.Team
	for rows.Next() {
		var t domain.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.ShortName, &t.Country, &t.Tier, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}
