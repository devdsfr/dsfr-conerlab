package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// StrategyRepo persiste estratégias e seus artefatos calculados (backtests,
// health, scores) — camada ANALYTICS da Remodelagem (migration 011, docs 08/09/12).
type StrategyRepo struct {
	db *pgxpool.Pool
}

func NewStrategyRepo(db *pgxpool.Pool) *StrategyRepo {
	return &StrategyRepo{db: db}
}

const strategyCols = `id, owner_id, name, description, definition::text, origin, visibility, active, favorite, created_at, updated_at`

func scanStrategy(row pgx.Row) (*domain.Strategy, error) {
	var s domain.Strategy
	err := row.Scan(&s.ID, &s.OwnerID, &s.Name, &s.Description, &s.Definition,
		&s.Origin, &s.Visibility, &s.Active, &s.Favorite, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StrategyRepo) Create(ctx context.Context, s *domain.Strategy) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO strategies (owner_id, name, description, definition, origin, visibility, active, favorite)
		VALUES ($1,$2,$3,$4::jsonb,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`,
		s.OwnerID, s.Name, s.Description, s.Definition, s.Origin, s.Visibility, s.Active, s.Favorite).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// ListForUser retorna as estratégias do usuário + as públicas do sistema
// (origin=discovery, visibility=public).
func (r *StrategyRepo) ListForUser(ctx context.Context, userID int64) ([]domain.Strategy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+strategyCols+` FROM strategies
		WHERE owner_id = $1 OR (owner_id IS NULL AND visibility = 'public')
		ORDER BY favorite DESC, updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectStrategies(rows)
}

// ListActive retorna todas as estratégias ativas — unidade de trabalho do
// Strategy Worker (doc 15, Worker 04).
func (r *StrategyRepo) ListActive(ctx context.Context) ([]domain.Strategy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+strategyCols+` FROM strategies WHERE active ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectStrategies(rows)
}

func collectStrategies(rows pgx.Rows) ([]domain.Strategy, error) {
	var out []domain.Strategy
	for rows.Next() {
		s, err := scanStrategy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *StrategyRepo) GetByID(ctx context.Context, id int64) (*domain.Strategy, error) {
	s, err := scanStrategy(r.db.QueryRow(ctx, `SELECT `+strategyCols+` FROM strategies WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

// SetFlags atualiza active/favorite (dono confere no usecase/handler).
func (r *StrategyRepo) SetFlags(ctx context.Context, id int64, active, favorite bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE strategies SET active=$2, favorite=$3, updated_at=now() WHERE id=$1`,
		id, active, favorite)
	return err
}

func (r *StrategyRepo) Delete(ctx context.Context, id, ownerID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM strategies WHERE id=$1 AND owner_id=$2`, id, ownerID)
	return err
}

// InsertBacktest grava uma execução no histórico auditável da estratégia.
func (r *StrategyRepo) InsertBacktest(ctx context.Context, b *domain.Backtest) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO backtests (strategy_id, games, wins, losses, voids, roi, yield, ev,
			drawdown, profit, confidence, period_start, period_end, algorithm_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at`,
		b.StrategyID, b.Games, b.Wins, b.Losses, b.Voids, b.ROI, b.Yield, b.EV,
		b.Drawdown, b.Profit, b.Confidence, b.PeriodStart, b.PeriodEnd, b.AlgorithmVersion).
		Scan(&b.ID, &b.CreatedAt)
}

// LastBacktests retorna os últimos N backtests (mais recente primeiro) — o
// Health Engine compara o atual com o anterior para calcular os deltas.
func (r *StrategyRepo) LastBacktests(ctx context.Context, strategyID int64, limit int) ([]domain.Backtest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, strategy_id, games, wins, losses, voids, roi, yield, ev,
		       drawdown, profit, confidence, period_start, period_end, algorithm_version, created_at
		FROM backtests WHERE strategy_id=$1
		ORDER BY created_at DESC LIMIT $2`, strategyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Backtest
	for rows.Next() {
		var b domain.Backtest
		if err := rows.Scan(&b.ID, &b.StrategyID, &b.Games, &b.Wins, &b.Losses, &b.Voids,
			&b.ROI, &b.Yield, &b.EV, &b.Drawdown, &b.Profit, &b.Confidence,
			&b.PeriodStart, &b.PeriodEnd, &b.AlgorithmVersion, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpsertHealth grava a saúde recalculada (1 linha por estratégia).
func (r *StrategyRepo) UpsertHealth(ctx context.Context, h *domain.StrategyHealth) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO strategy_health (strategy_id, health_score, trend, variation, algorithm_version, updated_at)
		VALUES ($1,$2,$3,$4::jsonb,$5, now())
		ON CONFLICT (strategy_id) DO UPDATE SET
			health_score=EXCLUDED.health_score, trend=EXCLUDED.trend,
			variation=EXCLUDED.variation, algorithm_version=EXCLUDED.algorithm_version,
			updated_at=now()`,
		h.StrategyID, h.HealthScore, h.Trend, h.Variation, h.AlgorithmVersion)
	return err
}

// UpsertScores grava os scores proprietários (1 linha por estratégia).
func (r *StrategyRepo) UpsertScores(ctx context.Context, s *domain.StrategyScores) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO strategy_scores (strategy_id, dsfr_score, components, confidence,
			robustness, volatility, risk, ranking, lifecycle_stage, algorithm_version, updated_at)
		VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10, now())
		ON CONFLICT (strategy_id) DO UPDATE SET
			dsfr_score=EXCLUDED.dsfr_score, components=EXCLUDED.components,
			confidence=EXCLUDED.confidence, robustness=EXCLUDED.robustness,
			volatility=EXCLUDED.volatility, risk=EXCLUDED.risk, ranking=EXCLUDED.ranking,
			lifecycle_stage=EXCLUDED.lifecycle_stage,
			algorithm_version=EXCLUDED.algorithm_version, updated_at=now()`,
		s.StrategyID, s.DSFRScore, s.Components, s.Confidence,
		s.Robustness, s.Volatility, s.Risk, s.Ranking, s.LifecycleStage, s.AlgorithmVersion)
	return err
}

// GetHealth e GetScores retornam nil (sem erro) quando ainda não calculados.
func (r *StrategyRepo) GetHealth(ctx context.Context, strategyID int64) (*domain.StrategyHealth, error) {
	var h domain.StrategyHealth
	err := r.db.QueryRow(ctx, `
		SELECT strategy_id, health_score, trend, variation::text, algorithm_version, updated_at
		FROM strategy_health WHERE strategy_id=$1`, strategyID).
		Scan(&h.StrategyID, &h.HealthScore, &h.Trend, &h.Variation, &h.AlgorithmVersion, &h.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ---------------------------------------------------------------------------
// Strategy Discovery Engine (Remodelagem F6, doc 08)
// ---------------------------------------------------------------------------

// UpsertDiscovered grava uma estratégia descoberta pelo sistema de forma
// idempotente. O conflito é resolvido pelo índice único PARCIAL criado na
// migration 012 (name WHERE origin='discovery'), por isso o ON CONFLICT repete
// o predicado — sem ele o Postgres não consegue inferir qual índice usar.
//
// Um ciclo que reencontra um padrão já conhecido atualiza a descrição (os
// números mudaram) e o reativa, em vez de criar uma segunda linha.
func (r *StrategyRepo) UpsertDiscovered(ctx context.Context, s *domain.Strategy) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO strategies (owner_id, name, description, definition, origin, visibility, active, favorite)
		VALUES (NULL, $1, $2, $3::jsonb, 'discovery', 'public', TRUE, FALSE)
		ON CONFLICT (name) WHERE origin = 'discovery' DO UPDATE SET
			description = EXCLUDED.description,
			definition  = EXCLUDED.definition,
			active      = TRUE,
			updated_at  = now()
		RETURNING id, created_at, updated_at`,
		s.Name, s.Description, s.Definition).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// DeactivateDiscoveredExcept desativa as descobertas da liga que não sobreviveram
// ao ciclo atual. Desativar (e não apagar) preserva o histórico em `backtests`,
// que é auditável por definição (doc 16).
func (r *StrategyRepo) DeactivateDiscoveredExcept(ctx context.Context, leagueID int64, keepIDs []int64) (int, error) {
	if keepIDs == nil {
		keepIDs = []int64{}
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE strategies SET active = FALSE, updated_at = now()
		WHERE origin = 'discovery' AND active
		  AND (definition ->> 'league_id')::BIGINT = $1
		  AND NOT (id = ANY($2::BIGINT[]))`, leagueID, keepIDs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ListDiscovered monta o ranking de descobertas em uma única query: estratégia +
// último backtest (LATERAL) + health + scores. A ordenação segue o RankingScore
// (Catálogo 28); estratégias ainda sem score calculado caem para o fim da lista.
func (r *StrategyRepo) ListDiscovered(ctx context.Context, leagueID *int64, limit int) ([]repository.DiscoveredStrategy, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.owner_id, s.name, s.description, s.definition::text, s.origin,
		       s.visibility, s.active, s.favorite, s.created_at, s.updated_at,
		       b.id, b.games, b.wins, b.losses, b.voids, b.roi, b.yield, b.ev,
		       b.drawdown, b.profit, b.confidence, b.period_start, b.period_end,
		       b.algorithm_version, b.created_at,
		       h.strategy_id, h.health_score, h.trend, h.variation::text,
		       h.algorithm_version, h.updated_at,
		       sc.strategy_id, sc.dsfr_score, sc.components::text, sc.confidence,
		       sc.robustness, sc.volatility, sc.risk, sc.ranking, sc.lifecycle_stage,
		       sc.algorithm_version, sc.updated_at
		FROM strategies s
		LEFT JOIN LATERAL (
			SELECT * FROM backtests WHERE strategy_id = s.id
			ORDER BY created_at DESC LIMIT 1
		) b ON TRUE
		LEFT JOIN strategy_health h ON h.strategy_id = s.id
		LEFT JOIN strategy_scores sc ON sc.strategy_id = s.id
		WHERE s.origin = 'discovery' AND s.active
		  AND ($1::BIGINT IS NULL OR (s.definition ->> 'league_id')::BIGINT = $1)
		ORDER BY COALESCE(sc.ranking, sc.dsfr_score) DESC NULLS LAST, s.id
		LIMIT $2`, leagueID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []repository.DiscoveredStrategy{}
	for rows.Next() {
		item, err := scanDiscovered(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

// scanDiscovered lê a linha do LEFT JOIN triplo. Como todo artefato pode estar
// ausente, cada bloco é lido em variáveis anuláveis e só vira struct se a sua
// chave primária tiver vindo preenchida.
func scanDiscovered(row pgx.Row) (*repository.DiscoveredStrategy, error) {
	var s domain.Strategy

	var btID *int64
	var btGames, btWins, btLosses, btVoids *int
	var btROI, btYield, btEV, btDrawdown, btProfit, btConfidence *float64
	var btPeriodStart, btPeriodEnd, btCreatedAt *time.Time
	var btVersion *string

	var hID *int64
	var hScore, hTrend *float64
	var hVariation, hVersion *string
	var hUpdatedAt *time.Time

	var scID *int64
	var scDSFR, scConfidence, scRobustness, scVolatility, scRisk, scRanking *float64
	var scComponents, scLifecycle, scVersion *string
	var scUpdatedAt *time.Time

	if err := row.Scan(
		&s.ID, &s.OwnerID, &s.Name, &s.Description, &s.Definition, &s.Origin,
		&s.Visibility, &s.Active, &s.Favorite, &s.CreatedAt, &s.UpdatedAt,
		&btID, &btGames, &btWins, &btLosses, &btVoids, &btROI, &btYield, &btEV,
		&btDrawdown, &btProfit, &btConfidence, &btPeriodStart, &btPeriodEnd,
		&btVersion, &btCreatedAt,
		&hID, &hScore, &hTrend, &hVariation, &hVersion, &hUpdatedAt,
		&scID, &scDSFR, &scComponents, &scConfidence, &scRobustness, &scVolatility,
		&scRisk, &scRanking, &scLifecycle, &scVersion, &scUpdatedAt,
	); err != nil {
		return nil, err
	}

	item := &repository.DiscoveredStrategy{Strategy: s}

	if btID != nil {
		item.Backtest = &domain.Backtest{
			ID: *btID, StrategyID: s.ID,
			Games: derefInt(btGames), Wins: derefInt(btWins),
			Losses: derefInt(btLosses), Voids: derefInt(btVoids),
			ROI: btROI, Yield: btYield, EV: btEV, Drawdown: btDrawdown,
			Profit: btProfit, Confidence: btConfidence,
			PeriodStart: btPeriodStart, PeriodEnd: btPeriodEnd,
			AlgorithmVersion: derefStr(btVersion), CreatedAt: derefTime(btCreatedAt),
		}
	}
	if hID != nil {
		item.Health = &domain.StrategyHealth{
			StrategyID: *hID, HealthScore: derefFloat(hScore), Trend: hTrend,
			Variation: derefStr(hVariation), AlgorithmVersion: derefStr(hVersion),
			UpdatedAt: derefTime(hUpdatedAt),
		}
	}
	if scID != nil {
		item.Scores = &domain.StrategyScores{
			StrategyID: *scID, DSFRScore: derefFloat(scDSFR),
			Components: derefStr(scComponents), Confidence: scConfidence,
			Robustness: scRobustness, Volatility: scVolatility, Risk: scRisk,
			Ranking: scRanking, LifecycleStage: derefStr(scLifecycle),
			AlgorithmVersion: derefStr(scVersion), UpdatedAt: derefTime(scUpdatedAt),
		}
	}
	return item, nil
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}

func (r *StrategyRepo) GetScores(ctx context.Context, strategyID int64) (*domain.StrategyScores, error) {
	var s domain.StrategyScores
	err := r.db.QueryRow(ctx, `
		SELECT strategy_id, dsfr_score, components::text, confidence, robustness,
		       volatility, risk, ranking, lifecycle_stage, algorithm_version, updated_at
		FROM strategy_scores WHERE strategy_id=$1`, strategyID).
		Scan(&s.StrategyID, &s.DSFRScore, &s.Components, &s.Confidence, &s.Robustness,
			&s.Volatility, &s.Risk, &s.Ranking, &s.LifecycleStage, &s.AlgorithmVersion, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
