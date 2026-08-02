package domain

import "time"

// Entidades da camada ANALYTICS (Remodelagem Fase 2 — migration 011, seguindo
// Remodelagem/16-modelo-de-dados.md). Tudo aqui é CALCULADO por workers a partir
// de RAW+NORMALIZED e é reprocessável; algorithm_version referencia a versão do
// Formula Catalog (internal/formulas.Version) usada no cálculo.

// TeamMetrics é o agregado pré-calculado por equipe/temporada/métrica que
// substitui gradualmente o cálculo on-the-fly do Dashboard.
type TeamMetrics struct {
	TeamID           int64              `json:"team_id"`
	SeasonID         int64              `json:"season_id"`
	LeagueID         int64              `json:"league_id"`
	Metric           string             `json:"metric"` // corners|goals|offsides|shots|shots_on_target
	SampleSize       int                `json:"sample_size"`
	AvgTotal         *float64           `json:"avg_total,omitempty"`
	AvgFor           *float64           `json:"avg_for,omitempty"`
	AvgAgainst       *float64           `json:"avg_against,omitempty"`
	AvgHome          *float64           `json:"avg_home,omitempty"`
	AvgAway          *float64           `json:"avg_away,omitempty"`
	Last5Avg         *float64           `json:"last5_avg,omitempty"`
	Last10Avg        *float64           `json:"last10_avg,omitempty"`
	Last20Avg        *float64           `json:"last20_avg,omitempty"`
	Variance         *float64           `json:"variance,omitempty"`
	StdDev           *float64           `json:"std_dev,omitempty"`
	Consistency      *float64           `json:"consistency,omitempty"` // 0..100 (Catálogo 22)
	Trend            *float64           `json:"trend,omitempty"`       // -1..1 (Catálogo 29)
	Frequencies      map[string]float64 `json:"frequencies"`           // limiar -> % acima
	AlgorithmVersion string             `json:"algorithm_version"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// Strategy é uma estratégia salva — criada pelo usuário ou descoberta
// automaticamente pelo Discovery Engine (origin = "discovery").
type Strategy struct {
	ID          int64     `json:"id"`
	OwnerID     *int64    `json:"owner_id,omitempty"` // nil = estratégia do sistema
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Definition  string    `json:"definition"` // JSON dos filtros (liga, métrica, limiar, mando, janela...)
	Origin      string    `json:"origin"`     // user|discovery
	Visibility  string    `json:"visibility"` // private|public
	Active      bool      `json:"active"`
	Favorite    bool      `json:"favorite"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Backtest é uma execução de backtest persistida de uma estratégia.
type Backtest struct {
	ID               int64      `json:"id"`
	StrategyID       int64      `json:"strategy_id"`
	Games            int        `json:"games"`
	Wins             int        `json:"wins"`
	Losses           int        `json:"losses"`
	Voids            int        `json:"voids"`
	ROI              *float64   `json:"roi,omitempty"`
	Yield            *float64   `json:"yield,omitempty"`
	EV               *float64   `json:"ev,omitempty"`
	Drawdown         *float64   `json:"drawdown,omitempty"`
	Profit           *float64   `json:"profit,omitempty"`
	Confidence       *float64   `json:"confidence,omitempty"` // 0..100 (Catálogo 23)
	PeriodStart      *time.Time `json:"period_start,omitempty"`
	PeriodEnd        *time.Time `json:"period_end,omitempty"`
	AlgorithmVersion string     `json:"algorithm_version"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Simulation é uma simulação financeira Monte Carlo persistida (Catálogo 19).
type Simulation struct {
	ID                  int64     `json:"id"`
	StrategyID          int64     `json:"strategy_id"`
	Stake               float64   `json:"stake"`
	Bankroll            float64   `json:"bankroll"`
	WinRate             float64   `json:"win_rate"` // fração 0..1
	Odd                 float64   `json:"odd"`
	Runs                int       `json:"runs"`
	ExpectedProfit      *float64  `json:"expected_profit,omitempty"`
	ExpectedCapital     *float64  `json:"expected_capital,omitempty"`
	Drawdown            *float64  `json:"drawdown,omitempty"`             // fração média
	ProbabilityPositive *float64  `json:"probability_positive,omitempty"` // fração
	RuinProbability     *float64  `json:"ruin_probability,omitempty"`     // fração
	AlgorithmVersion    string    `json:"algorithm_version"`
	CreatedAt           time.Time `json:"created_at"`
}

// StrategyHealth é a saúde recalculada da estratégia (Catálogo 25):
// 0..100, 50 = estável.
type StrategyHealth struct {
	StrategyID       int64     `json:"strategy_id"`
	HealthScore      float64   `json:"health_score"`
	Trend            *float64  `json:"trend,omitempty"`
	Variation        string    `json:"variation"` // JSON dos deltas (roi, ev, drawdown, consistency)
	AlgorithmVersion string    `json:"algorithm_version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// StrategyScores agrupa os scores proprietários (Catálogo 24 e 26–32).
type StrategyScores struct {
	StrategyID       int64     `json:"strategy_id"`
	DSFRScore        float64   `json:"dsfr_score"`
	Components       string    `json:"components"` // JSON dos componentes normalizados
	Confidence       *float64  `json:"confidence,omitempty"`
	Robustness       *float64  `json:"robustness,omitempty"`
	Volatility       *float64  `json:"volatility,omitempty"`
	Risk             *float64  `json:"risk,omitempty"`
	Ranking          *float64  `json:"ranking,omitempty"`
	LifecycleStage   string    `json:"lifecycle_stage"` // nascimento|crescimento|maturidade|declinio|obsoleta
	AlgorithmVersion string    `json:"algorithm_version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Opportunity é uma mudança relevante detectada pelo Opportunity Engine, sempre
// com explicação legível (princípio da remodelagem: explicação acima de números).
type Opportunity struct {
	ID               int64      `json:"id"`
	TeamID           *int64     `json:"team_id,omitempty"`
	StrategyID       *int64     `json:"strategy_id,omitempty"`
	Priority         int        `json:"priority"`
	OpportunityScore float64    `json:"opportunity_score"`
	Reason           string     `json:"reason"`
	Status           string     `json:"status"` // open|seen|expired
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// Insight é uma explicação gerada pelo Insight Engine sobre uma mudança.
type Insight struct {
	ID          int64     `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	StrategyID  *int64    `json:"strategy_id,omitempty"`
	TeamID      *int64    `json:"team_id,omitempty"`
	Status      string    `json:"status"` // new|read|archived
	CreatedAt   time.Time `json:"created_at"`
}

// WorkerRun registra uma execução de worker do pipeline analytics (Fase 3).
type WorkerRun struct {
	ID         int64      `json:"id"`
	Worker     string     `json:"worker"`
	Status     string     `json:"status"` // running|ok|error
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	DurationMs *int64     `json:"duration_ms,omitempty"`
	Processed  int        `json:"processed"`
	Errors     int        `json:"errors"`
	Details    string     `json:"details"` // JSON livre
}
