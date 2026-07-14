package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StatSyncRepo concentra as operações de persistência do Módulo de Sincronização de
// Dados (Workers de Descoberta e Atualização — ver internal/usecase/statsync).
// Reaproveita a tabela `matches` já existente (ver comentário no topo de
// migrations/004_statistics_sync.sql sobre a decisão de não duplicar a fonte da
// verdade em uma tabela `fixtures` paralela).
//
// Importante: nenhuma query aqui toca a coluna corner_odds — ela é calculada por um
// processo separado (ver usecase.SyntheticCornerOdds / UpdateCornerOdds usado no seed
// manual) e sobrescrevê-la aqui apagaria odds já calculadas.
type StatSyncRepo struct {
	db *pgxpool.Pool
}

func NewStatSyncRepo(db *pgxpool.Pool) *StatSyncRepo {
	return &StatSyncRepo{db: db}
}

// SyncTarget é um campeonato já presente no CornerLab com origem em um provedor real
// (external_id preenchido) e a temporada mais recente já sincronizada para ele. O
// Worker de Descoberta usa isso para saber "o que observar" sem precisar de
// configuração manual: assim que uma nova temporada é semeada (seguindo a regra
// "temporada atual ou a mais recente passada"), o worker passa a acompanhá-la
// automaticamente, sem mudança de código.
type SyncTarget struct {
	LeagueID         int64
	LeagueExternalID string
	LeagueName       string
	Country          string
	SeasonID         int64
	SeasonYear       int
}

func (r *StatSyncRepo) ListSyncTargets(ctx context.Context) ([]SyncTarget, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id, l.external_id, l.name, l.country, s.id, s.year
		FROM leagues l
		JOIN seasons s ON s.league_id = l.id
		JOIN (
			SELECT league_id, MAX(year) AS max_year FROM seasons GROUP BY league_id
		) latest ON latest.league_id = s.league_id AND latest.max_year = s.year
		WHERE l.external_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := []SyncTarget{}
	for rows.Next() {
		var t SyncTarget
		if err := rows.Scan(&t.LeagueID, &t.LeagueExternalID, &t.LeagueName, &t.Country, &t.SeasonID, &t.SeasonYear); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// UpsertTeam e LinkTeamToLeague reaproveitam exatamente a mesma convenção idempotente
// (upsert por external_id) já usada por SyncRepo — mantidas aqui como métodos próprios
// para não acoplar este repositório ao de cmd/sync.
func (r *StatSyncRepo) UpsertTeam(ctx context.Context, externalID, name, shortName, country string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO teams (external_id, name, short_name, country, tier)
		VALUES ($1, $2, $3, $4, 'G12')
		ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, externalID, name, shortName, country).Scan(&id)
	return id, err
}

func (r *StatSyncRepo) LinkTeamToLeague(ctx context.Context, leagueID, teamID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO league_teams (league_id, team_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, leagueID, teamID)
	return err
}

// UpsertScheduledFixture grava (ou atualiza) uma partida descoberta pelo Worker de
// Descoberta. Nunca regride uma partida já FINALIZADA de volta para AGENDADO (guarda
// contra uma redescoberta tardia sobrescrever um resultado já sincronizado), e nunca
// toca em goals/corners/estatísticas — essas colunas só são preenchidas pelo Worker de
// Atualização (FinalizeFixture), que tem os dados completos.
func (r *StatSyncRepo) UpsertScheduledFixture(ctx context.Context, externalID string, leagueID, seasonID int64,
	round int, matchDate time.Time, homeTeamID, awayTeamID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO matches (external_id, league_id, season_id, round, match_date, home_team_id, away_team_id, status, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'AGENDADO', now())
		ON CONFLICT (external_id) DO UPDATE SET
			match_date = EXCLUDED.match_date,
			round      = EXCLUDED.round,
			status     = CASE WHEN matches.status = 'FINALIZADO' THEN matches.status ELSE EXCLUDED.status END,
			updated_at = now()`,
		externalID, leagueID, seasonID, round, matchDate, homeTeamID, awayTeamID)
	return err
}

// DueFixture é uma partida cujo status ainda é AGENDADO mas a data já passou — o
// Worker de Atualização busca o resultado completo dela no provedor.
type DueFixture struct {
	ExternalID string
	MatchDate  time.Time
}

func (r *StatSyncRepo) ListDueForUpdate(ctx context.Context, buffer time.Duration, limit int) ([]DueFixture, error) {
	rows, err := r.db.Query(ctx, `
		SELECT external_id, match_date FROM matches
		WHERE status = 'AGENDADO' AND match_date < (now() - ($1 * interval '1 second')) AND external_id IS NOT NULL
		ORDER BY match_date ASC
		LIMIT $2`, buffer.Seconds(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	due := []DueFixture{}
	for rows.Next() {
		var d DueFixture
		if err := rows.Scan(&d.ExternalID, &d.MatchDate); err != nil {
			return nil, err
		}
		due = append(due, d)
	}
	return due, rows.Err()
}

// FixtureStatsUpdate é o conjunto de campos que o Worker de Atualização grava ao
// finalizar uma partida. Ponteiros nil não sobrescrevem o valor já gravado (COALESCE),
// para o caso do provedor devolver só parte das estatísticas.
type FixtureStatsUpdate struct {
	HomeGoals, AwayGoals                 int
	HomeCorners, AwayCorners             *int
	HomePossession, AwayPossession       *int
	HomeShots, AwayShots                 *int
	HomeShotsOnTarget, AwayShotsOnTarget *int
	HomeYellowCards, AwayYellowCards     *int
	HomeRedCards, AwayRedCards           *int
	Referee, Venue                       string
}

func (r *StatSyncRepo) FinalizeFixture(ctx context.Context, externalID string, u FixtureStatsUpdate) error {
	_, err := r.db.Exec(ctx, `
		UPDATE matches SET
			status               = 'FINALIZADO',
			home_goals           = $2,
			away_goals           = $3,
			home_corners         = COALESCE($4, home_corners),
			away_corners         = COALESCE($5, away_corners),
			home_possession      = $6,
			away_possession      = $7,
			home_shots           = $8,
			away_shots           = $9,
			home_shots_on_target = $10,
			away_shots_on_target = $11,
			home_yellow_cards    = $12,
			away_yellow_cards    = $13,
			home_red_cards       = $14,
			away_red_cards       = $15,
			referee              = NULLIF($16, ''),
			venue                = NULLIF($17, ''),
			stats_synced_at      = now(),
			updated_at           = now()
		WHERE external_id = $1`,
		externalID, u.HomeGoals, u.AwayGoals, u.HomeCorners, u.AwayCorners,
		u.HomePossession, u.AwayPossession, u.HomeShots, u.AwayShots,
		u.HomeShotsOnTarget, u.AwayShotsOnTarget, u.HomeYellowCards, u.AwayYellowCards,
		u.HomeRedCards, u.AwayRedCards, u.Referee, u.Venue)
	return err
}

// ErrNoRows é reexportado para os usecases distinguirem "não achou o time" (segue em
// frente) de erro real de banco, sem importar pgx diretamente.
var ErrNoRows = pgx.ErrNoRows
