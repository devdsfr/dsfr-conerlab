package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncRepo concentra operações de upsert idempotentes usadas pela sincronização com
// provedores de dados externos (cmd/sync). Upserts são feitos por external_id, para
// que rodar a sincronização várias vezes não duplique campeonatos/equipes/partidas.
type SyncRepo struct {
	db *pgxpool.Pool
}

func NewSyncRepo(db *pgxpool.Pool) *SyncRepo {
	return &SyncRepo{db: db}
}

func (r *SyncRepo) UpsertLeague(ctx context.Context, externalID, name, country, tier string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO leagues (external_id, name, country, tier)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, externalID, name, country, tier).Scan(&id)
	return id, err
}

func (r *SyncRepo) UpsertSeason(ctx context.Context, leagueID int64, year int, label string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO seasons (league_id, year, label)
		VALUES ($1, $2, $3)
		ON CONFLICT (league_id, year) DO UPDATE SET label = EXCLUDED.label
		RETURNING id`, leagueID, year, label).Scan(&id)
	return id, err
}

func (r *SyncRepo) UpsertTeam(ctx context.Context, externalID, name, shortName, country string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO teams (external_id, name, short_name, country, tier)
		VALUES ($1, $2, $3, $4, 'G12')
		ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, externalID, name, shortName, country).Scan(&id)
	return id, err
}

func (r *SyncRepo) LinkTeamToLeague(ctx context.Context, leagueID, teamID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO league_teams (league_id, team_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, leagueID, teamID)
	return err
}

func (r *SyncRepo) UpsertMatch(ctx context.Context, externalID string, leagueID, seasonID int64, round int, matchDate any,
	homeTeamID, awayTeamID int64, homeCorners, awayCorners, homeGoals, awayGoals int, cornerOdds map[string]float64) error {

	oddsJSON, err := json.Marshal(cornerOdds)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO matches (external_id, league_id, season_id, round, match_date, home_team_id, away_team_id,
			home_corners, away_corners, home_goals, away_goals, corner_odds)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
		ON CONFLICT (external_id) DO UPDATE SET
			home_corners = EXCLUDED.home_corners,
			away_corners = EXCLUDED.away_corners,
			home_goals = EXCLUDED.home_goals,
			away_goals = EXCLUDED.away_goals,
			corner_odds = EXCLUDED.corner_odds`,
		externalID, leagueID, seasonID, round, matchDate, homeTeamID, awayTeamID,
		homeCorners, awayCorners, homeGoals, awayGoals, string(oddsJSON))
	return err
}
