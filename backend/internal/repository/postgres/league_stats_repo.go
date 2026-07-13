package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeagueStatsRepo struct {
	db *pgxpool.Pool
}

func NewLeagueStatsRepo(db *pgxpool.Pool) *LeagueStatsRepo {
	return &LeagueStatsRepo{db: db}
}

// TeamAggregates calcula médias e desvio padrão de escanteios por equipe, considerando
// no máximo `limit` jogos mais recentes de cada equipe dentro do campeonato/temporadas
// informados (limit=0 → usa todos os jogos do período). Usado pelos rankings e pela
// análise de adversário do módulo de Inteligência Estatística.
func (r *LeagueStatsRepo) TeamAggregates(ctx context.Context, leagueID int64, seasonIDs []int64, limit int) ([]repository.TeamAggregate, error) {
	var seasonFilter []int64
	if len(seasonIDs) > 0 {
		seasonFilter = seasonIDs
	}

	query := `
		WITH team_games AS (
			SELECT m.home_team_id AS team_id, m.home_corners AS corners_for, m.away_corners AS corners_against, m.match_date
			FROM matches m
			WHERE m.league_id = $1 AND ($2::bigint[] IS NULL OR m.season_id = ANY($2))
			UNION ALL
			SELECT m.away_team_id, m.away_corners, m.home_corners, m.match_date
			FROM matches m
			WHERE m.league_id = $1 AND ($2::bigint[] IS NULL OR m.season_id = ANY($2))
		),
		ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY team_id ORDER BY match_date DESC) AS rn
			FROM team_games
		),
		filtered AS (
			SELECT * FROM ranked WHERE ($3::int = 0 OR rn <= $3)
		)
		SELECT team_id,
		       COUNT(*) AS sample_size,
		       AVG(corners_for) AS avg_for,
		       AVG(corners_against) AS avg_against,
		       AVG(corners_for + corners_against) AS avg_total,
		       COALESCE(STDDEV_POP(corners_for + corners_against), 0) AS stddev_total
		FROM filtered
		GROUP BY team_id`

	rows, err := r.db.Query(ctx, query, leagueID, seasonFilter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type raw struct {
		teamID     int64
		sample     int
		avgFor     float64
		avgAgainst float64
		avgTotal   float64
		stdDev     float64
	}
	var raws []raw
	teamIDs := make([]int64, 0)
	for rows.Next() {
		var rr raw
		if err := rows.Scan(&rr.teamID, &rr.sample, &rr.avgFor, &rr.avgAgainst, &rr.avgTotal, &rr.stdDev); err != nil {
			return nil, err
		}
		raws = append(raws, rr)
		teamIDs = append(teamIDs, rr.teamID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(raws) == 0 {
		return nil, nil
	}

	teamRows, err := r.db.Query(ctx, `SELECT id, name, short_name, country, tier, created_at FROM teams WHERE id = ANY($1)`, teamIDs)
	if err != nil {
		return nil, err
	}
	defer teamRows.Close()
	teamsByID := map[int64]domain.Team{}
	for teamRows.Next() {
		var t domain.Team
		if err := teamRows.Scan(&t.ID, &t.Name, &t.ShortName, &t.Country, &t.Tier, &t.CreatedAt); err != nil {
			return nil, err
		}
		teamsByID[t.ID] = t
	}

	result := make([]repository.TeamAggregate, 0, len(raws))
	for _, rr := range raws {
		cv := 0.0
		if rr.avgTotal != 0 {
			cv = rr.stdDev / rr.avgTotal
		}
		consistency := 1 - cv
		if consistency < 0 {
			consistency = 0
		}
		if consistency > 1 {
			consistency = 1
		}
		result = append(result, repository.TeamAggregate{
			Team:           teamsByID[rr.teamID],
			SampleSize:     rr.sample,
			AvgFor:         round2(rr.avgFor),
			AvgAgainst:     round2(rr.avgAgainst),
			AvgTotal:       round2(rr.avgTotal),
			StdDevTotal:    round2(rr.stdDev),
			ConsistencyIdx: round4(consistency),
		})
	}
	return result, nil
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
