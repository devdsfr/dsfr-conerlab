package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MatchRepo struct {
	db *pgxpool.Pool
}

func NewMatchRepo(db *pgxpool.Pool) *MatchRepo {
	return &MatchRepo{db: db}
}

// TeamMatches busca os últimos jogos JÁ DISPUTADOS de uma equipe (mandante ou
// visitante) e monta a visão "por equipe" já com escanteios a favor/sofridos e o
// adversário resolvido. Filtra status = 'FINALIZADO': partidas ainda 'AGENDADO'
// (descobertas pelo worker de sincronização mas ainda não jogadas) têm
// home_corners/away_corners zerados por padrão, e entrar aqui puxaria todas as
// estatísticas do dashboard (média, desvio padrão, frequências etc.) para zero —
// bug real relatado pelo usuário (Corinthians/Série A/2026 mostrando só jogos
// futuros com tudo 0).
func (r *MatchRepo) TeamMatches(ctx context.Context, f repository.MatchFilter) ([]domain.TeamMatchView, error) {
	// As colunas extras (posse, chutes, cartões etc.) seguem o mesmo padrão CASE WHEN
	// dos escanteios — reorientadas pela perspectiva da equipe consultada ($1), não
	// pela equipe mandante/visitante. Todas nullable: nem todo fixture tem 100% das
	// estatísticas publicadas pelo provedor (ver domain.TeamMatchView).
	query := `
		SELECT
			m.id, m.match_date,
			CASE WHEN m.home_team_id = $1 THEN m.away_team_id ELSE m.home_team_id END AS opponent_id,
			(m.home_team_id = $1) AS is_home,
			CASE WHEN m.home_team_id = $1 THEN m.home_corners ELSE m.away_corners END AS corners_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_corners ELSE m.home_corners END AS corners_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_possession ELSE m.away_possession END AS possession_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_possession ELSE m.home_possession END AS possession_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_shots ELSE m.away_shots END AS shots_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_shots ELSE m.home_shots END AS shots_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_shots_on_target ELSE m.away_shots_on_target END AS shots_on_target_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_shots_on_target ELSE m.home_shots_on_target END AS shots_on_target_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_shots_insidebox ELSE m.away_shots_insidebox END AS shots_insidebox_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_shots_insidebox ELSE m.home_shots_insidebox END AS shots_insidebox_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_shots_outsidebox ELSE m.away_shots_outsidebox END AS shots_outsidebox_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_shots_outsidebox ELSE m.home_shots_outsidebox END AS shots_outsidebox_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_blocked_shots ELSE m.away_blocked_shots END AS blocked_shots_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_blocked_shots ELSE m.home_blocked_shots END AS blocked_shots_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_fouls ELSE m.away_fouls END AS fouls_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_fouls ELSE m.home_fouls END AS fouls_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_offsides ELSE m.away_offsides END AS offsides_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_offsides ELSE m.home_offsides END AS offsides_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_yellow_cards ELSE m.away_yellow_cards END AS yellow_cards_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_yellow_cards ELSE m.home_yellow_cards END AS yellow_cards_against,
			CASE WHEN m.home_team_id = $1 THEN m.home_red_cards ELSE m.away_red_cards END AS red_cards_for,
			CASE WHEN m.home_team_id = $1 THEN m.away_red_cards ELSE m.home_red_cards END AS red_cards_against
		FROM matches m
		WHERE (m.home_team_id = $1 OR m.away_team_id = $1)
		  AND m.status = 'FINALIZADO'
	`
	args := []any{f.TeamID}
	argN := 2

	if f.LeagueID != nil {
		query += " AND m.league_id = $" + strconv.Itoa(argN)
		args = append(args, *f.LeagueID)
		argN++
	}
	if f.SeasonID != nil {
		query += " AND m.season_id = $" + strconv.Itoa(argN)
		args = append(args, *f.SeasonID)
		argN++
	}
	if f.HomeOnly {
		query += " AND m.home_team_id = $1"
	}
	if f.AwayOnly {
		query += " AND m.away_team_id = $1"
	}

	query += " ORDER BY m.match_date DESC"
	if f.Limit > 0 {
		query += " LIMIT $" + strconv.Itoa(argN)
		args = append(args, f.Limit)
		argN++
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawRow struct {
		matchID        int64
		matchDate      time.Time
		opponentID     int64
		isHome         bool
		cornersFor     int
		cornersAgainst int

		possessionFor, possessionAgainst           *int
		shotsFor, shotsAgainst                     *int
		shotsOnTargetFor, shotsOnTargetAgainst     *int
		shotsInsideboxFor, shotsInsideboxAgainst   *int
		shotsOutsideboxFor, shotsOutsideboxAgainst *int
		blockedShotsFor, blockedShotsAgainst       *int
		foulsFor, foulsAgainst                     *int
		offsidesFor, offsidesAgainst               *int
		yellowCardsFor, yellowCardsAgainst         *int
		redCardsFor, redCardsAgainst               *int
	}
	var raws []rawRow
	opponentIDs := map[int64]bool{}

	for rows.Next() {
		var rr rawRow
		if err := rows.Scan(
			&rr.matchID, &rr.matchDate, &rr.opponentID, &rr.isHome, &rr.cornersFor, &rr.cornersAgainst,
			&rr.possessionFor, &rr.possessionAgainst,
			&rr.shotsFor, &rr.shotsAgainst,
			&rr.shotsOnTargetFor, &rr.shotsOnTargetAgainst,
			&rr.shotsInsideboxFor, &rr.shotsInsideboxAgainst,
			&rr.shotsOutsideboxFor, &rr.shotsOutsideboxAgainst,
			&rr.blockedShotsFor, &rr.blockedShotsAgainst,
			&rr.foulsFor, &rr.foulsAgainst,
			&rr.offsidesFor, &rr.offsidesAgainst,
			&rr.yellowCardsFor, &rr.yellowCardsAgainst,
			&rr.redCardsFor, &rr.redCardsAgainst,
		); err != nil {
			return nil, err
		}
		raws = append(raws, rr)
		opponentIDs[rr.opponentID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	opponents := map[int64]domain.Team{}
	if len(opponentIDs) > 0 {
		ids := make([]int64, 0, len(opponentIDs))
		for id := range opponentIDs {
			ids = append(ids, id)
		}
		oRows, err := r.db.Query(ctx, `SELECT id, name, short_name, country, tier, created_at FROM teams WHERE id = ANY($1)`, ids)
		if err != nil {
			return nil, err
		}
		defer oRows.Close()
		for oRows.Next() {
			var t domain.Team
			if err := oRows.Scan(&t.ID, &t.Name, &t.ShortName, &t.Country, &t.Tier, &t.CreatedAt); err != nil {
				return nil, err
			}
			opponents[t.ID] = t
		}
	}

	views := make([]domain.TeamMatchView, 0, len(raws))
	for _, rr := range raws {
		opp := opponents[rr.opponentID]
		views = append(views, domain.TeamMatchView{
			MatchID:        rr.matchID,
			MatchDate:      rr.matchDate,
			Opponent:       opp,
			IsHome:         rr.isHome,
			CornersFor:     rr.cornersFor,
			CornersAgainst: rr.cornersAgainst,
			TotalCorners:   rr.cornersFor + rr.cornersAgainst,
			OpponentTier:   opp.Tier,

			PossessionFor:          rr.possessionFor,
			PossessionAgainst:      rr.possessionAgainst,
			ShotsFor:               rr.shotsFor,
			ShotsAgainst:           rr.shotsAgainst,
			ShotsOnTargetFor:       rr.shotsOnTargetFor,
			ShotsOnTargetAgainst:   rr.shotsOnTargetAgainst,
			ShotsInsideboxFor:      rr.shotsInsideboxFor,
			ShotsInsideboxAgainst:  rr.shotsInsideboxAgainst,
			ShotsOutsideboxFor:     rr.shotsOutsideboxFor,
			ShotsOutsideboxAgainst: rr.shotsOutsideboxAgainst,
			BlockedShotsFor:        rr.blockedShotsFor,
			BlockedShotsAgainst:    rr.blockedShotsAgainst,
			FoulsFor:               rr.foulsFor,
			FoulsAgainst:           rr.foulsAgainst,
			OffsidesFor:            rr.offsidesFor,
			OffsidesAgainst:        rr.offsidesAgainst,
			YellowCardsFor:         rr.yellowCardsFor,
			YellowCardsAgainst:     rr.yellowCardsAgainst,
			RedCardsFor:            rr.redCardsFor,
			RedCardsAgainst:        rr.redCardsAgainst,
		})
	}
	return views, nil
}

// AllMatches retorna as partidas JÁ DISPUTADAS (status = 'FINALIZADO') de um
// campeonato (opcionalmente restrito a um conjunto de temporadas) para uso no
// motor de filtros/backtesting. Mesma razão do filtro em TeamMatches: partidas
// 'AGENDADO' ainda não têm escanteios reais, e entrariam no backtest com 0/0
// distorcendo qualquer resultado.
func (r *MatchRepo) AllMatches(ctx context.Context, leagueID int64, seasonIDs []int64) ([]domain.Match, error) {
	query := `
		SELECT id, league_id, season_id, round, match_date, home_team_id, away_team_id,
		       home_corners, away_corners, home_goals, away_goals,
		       corner_odds::text, created_at
		FROM matches
		WHERE league_id = $1 AND status = 'FINALIZADO'
	`
	args := []any{leagueID}
	if len(seasonIDs) > 0 {
		query += " AND season_id = ANY($2)"
		args = append(args, seasonIDs)
	}
	query += " ORDER BY match_date ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []domain.Match
	for rows.Next() {
		var m domain.Match
		var oddsRaw string
		if err := rows.Scan(&m.ID, &m.LeagueID, &m.SeasonID, &m.Round, &m.MatchDate, &m.HomeTeamID, &m.AwayTeamID,
			&m.HomeCorners, &m.AwayCorners, &m.HomeGoals, &m.AwayGoals,
			&oddsRaw, &m.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(oddsRaw), &m.CornerOdds); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// ListUpcoming busca as próximas partidas AGENDADO (status ainda não FINALIZADO) das
// ligas com dado real (l.external_id IS NOT NULL — mesmo guardrail do catálogo de
// ligas, ver league_repo.go) para o calendário da página "Visão Geral". Limitado aos
// próximos 120 dias / 1000 jogos como teto de segurança — o worker de descoberta
// mapeia a temporada inteira à frente, então sem esse limite o payload cresceria sem
// necessidade real para uma tela de calendário.
func (r *MatchRepo) ListUpcoming(ctx context.Context) ([]domain.UpcomingMatch, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.match_date, m.league_id, l.name, m.round,
		       m.home_team_id, ht.name, m.away_team_id, at.name
		FROM matches m
		JOIN leagues l ON l.id = m.league_id
		JOIN teams ht ON ht.id = m.home_team_id
		JOIN teams at ON at.id = m.away_team_id
		WHERE m.status = 'AGENDADO'
		  AND l.external_id IS NOT NULL
		  AND m.match_date <= now() + interval '120 days'
		ORDER BY m.match_date ASC
		LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]domain.UpcomingMatch, 0)
	for rows.Next() {
		var m domain.UpcomingMatch
		if err := rows.Scan(&m.MatchID, &m.MatchDate, &m.LeagueID, &m.LeagueName, &m.Round,
			&m.HomeTeamID, &m.HomeTeamName, &m.AwayTeamID, &m.AwayTeamName); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *MatchRepo) GetMatchTeams(ctx context.Context, matchIDs []int64) (map[int64]domain.Match, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, league_id, season_id, round, match_date, home_team_id, away_team_id,
		       home_corners, away_corners, home_goals, away_goals,
		       corner_odds::text, created_at
		FROM matches WHERE id = ANY($1)`, matchIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[int64]domain.Match{}
	for rows.Next() {
		var m domain.Match
		var oddsRaw string
		if err := rows.Scan(&m.ID, &m.LeagueID, &m.SeasonID, &m.Round, &m.MatchDate, &m.HomeTeamID, &m.AwayTeamID,
			&m.HomeCorners, &m.AwayCorners, &m.HomeGoals, &m.AwayGoals,
			&oddsRaw, &m.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(oddsRaw), &m.CornerOdds); err != nil {
			return nil, err
		}
		result[m.ID] = m
	}
	return result, rows.Err()
}
