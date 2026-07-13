package domain

import "time"

// AlertRule representa uma regra de alerta inteligente criada pelo usuário (Módulo 7).
// Definition guarda o JSON serializado do critério (ex: métrica, operador, limite).
type AlertRule struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	Name       string    `json:"name" db:"name"`
	Definition string    `json:"definition" db:"definition"`
	Active     bool      `json:"active" db:"active"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// StrategyHistoryEntry registra cada execução de backtest/filtro para permitir
// comparação de desempenho ao longo do tempo (regra geral do MVP + seção
// "Histórico da Estratégia" do módulo de inteligência estatística).
type StrategyHistoryEntry struct {
	ID            int64     `json:"id" db:"id"`
	UserID        int64     `json:"user_id" db:"user_id"`
	SavedFilterID *int64    `json:"saved_filter_id" db:"saved_filter_id"`
	Definition    string    `json:"definition" db:"definition"`
	LeagueID      *int64    `json:"league_id" db:"league_id"`
	SeasonIDs     []int64   `json:"season_ids" db:"season_ids"`
	Result        string    `json:"result" db:"result"` // JSON serializado do BacktestResult
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
