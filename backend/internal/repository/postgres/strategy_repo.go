package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devdsfr/cornerlab/internal/domain"
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
