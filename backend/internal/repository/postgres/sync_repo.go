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

// UpsertTeam grava a equipe com tier VAZIO (AUD-004).
//
// Até 09/2026 esta query gravava a string literal 'G12' em toda equipe
// sincronizada. Não era classificação: era uma constante. O provedor não devolve
// esse campo, e nada no sistema calculava a posição de nenhuma equipe. Gravar um
// valor fixo fazia o filtro "contra o G12" parecer uma variável de força do
// adversário quando era, na prática, um filtro de procedência do cadastro.
//
// Vazio = não classificado, que é a verdade. Só volte a preencher com uma
// classificação por temporada apurada com os jogos anteriores à data da partida.
func (r *SyncRepo) UpsertTeam(ctx context.Context, externalID, name, shortName, country string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO teams (external_id, name, short_name, country, tier)
		VALUES ($1, $2, $3, $4, '')
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

	// odds_source = 'synthetic' explícito (AUD-001, complemento).
	//
	// As odds que chegam aqui vêm de usecase.SyntheticCornerOdds — derivadas da
	// média de escanteios do próprio lote, nunca de mercado. A migration 013
	// marcou os dados que JÁ existiam, mas esta query continuava gravando odd
	// sintética sem rótulo: linha nova caía no DEFAULT 'unknown', descrevendo
	// como "origem desconhecida" algo cuja origem é perfeitamente conhecida.
	//
	// O CASE no ON CONFLICT é a parte que mais importa. Sem ele, um ciclo de
	// cmd/sync sobrescreveria odd REAL com odd sintética mantendo o rótulo
	// 'real' — lavando dado fabricado como se fosse de mercado, que é
	// exatamente o defeito que o AUD-001 existe para impedir. Partida já marcada
	// como 'real' conserva odd e rótulo.
	_, err = r.db.Exec(ctx, `
		INSERT INTO matches (external_id, league_id, season_id, round, match_date, home_team_id, away_team_id,
			home_corners, away_corners, home_goals, away_goals, corner_odds, odds_source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,'synthetic')
		ON CONFLICT (external_id) DO UPDATE SET
			home_corners = EXCLUDED.home_corners,
			away_corners = EXCLUDED.away_corners,
			home_goals = EXCLUDED.home_goals,
			away_goals = EXCLUDED.away_goals,
			corner_odds = CASE WHEN matches.odds_source = 'real'
			                   THEN matches.corner_odds
			                   ELSE EXCLUDED.corner_odds END,
			odds_source = CASE WHEN matches.odds_source = 'real'
			                   THEN 'real'
			                   ELSE 'synthetic' END`,
		externalID, leagueID, seasonID, round, matchDate, homeTeamID, awayTeamID,
		homeCorners, awayCorners, homeGoals, awayGoals, string(oddsJSON))
	return err
}
