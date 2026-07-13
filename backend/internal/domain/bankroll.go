package domain

import "time"

// BankrollPhase representa uma fase configurável de evolução de banca (Módulo de
// Gestão Evolutiva de Banca). Ex: Fase 1 = R$150, Fase 2 = R$300... A sequência é o
// identificador lógico e estável da fase (usado por BankrollState), independente do
// ID de banco de dados.
type BankrollPhase struct {
	ID       int64   `json:"id" db:"id"`
	UserID   int64   `json:"user_id" db:"user_id"`
	Sequence int     `json:"sequence" db:"sequence"`
	Name     string  `json:"name" db:"name"`
	Amount   float64 `json:"amount" db:"amount"`
}

// BankrollCriteria define os critérios mínimos, configuráveis pelo usuário, que devem
// ser cumpridos integralmente para que a banca possa evoluir de fase. Nenhum critério
// isolado (nem o tempo) é suficiente sozinho.
type BankrollCriteria struct {
	UserID                 int64   `json:"user_id" db:"user_id"`
	MinDays                int     `json:"min_days" db:"min_days"`
	MinBets                int     `json:"min_bets" db:"min_bets"`
	MinWinRate             float64 `json:"min_win_rate" db:"min_win_rate"`
	MinROI                 float64 `json:"min_roi" db:"min_roi"`
	MinYield               float64 `json:"min_yield" db:"min_yield"`
	RequirePositiveProfit  bool    `json:"require_positive_profit" db:"require_positive_profit"`
	MinCompletedCycles     int     `json:"min_completed_cycles" db:"min_completed_cycles"`
	// CycleWinStreak define quantas vitórias consecutivas (sem sobreposição) formam um
	// "ciclo completo" — usado para contar automaticamente MinCompletedCycles a partir
	// das apostas reais já registradas, sem exigir cadastro manual de ciclos.
	CycleWinStreak int `json:"cycle_win_streak" db:"cycle_win_streak"`
}

// BankrollState é o estado atual do usuário no módulo: em qual fase está e desde
// quando, mais os contadores históricos de promoções/rebaixamentos.
type BankrollState struct {
	UserID                int64     `json:"user_id" db:"user_id"`
	CurrentPhaseSequence  int       `json:"current_phase_sequence" db:"current_phase_sequence"`
	PhaseStartedAt        time.Time `json:"phase_started_at" db:"phase_started_at"`
	HighestPhaseSequence  int       `json:"highest_phase_sequence" db:"highest_phase_sequence"`
	Promotions            int       `json:"promotions" db:"promotions"`
	Demotions             int       `json:"demotions" db:"demotions"`
}

// BankrollHistoryEntry registra toda mudança de banca (promoção ou rebaixamento).
// Nunca é apagado — é o registro de auditoria da evolução da estratégia.
type BankrollHistoryEntry struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	FromAmount float64   `json:"from_amount" db:"from_amount"`
	ToAmount   float64   `json:"to_amount" db:"to_amount"`
	Direction  string    `json:"direction" db:"direction"` // "promotion" | "demotion"
	Reason     string    `json:"reason" db:"reason"`
	Notes      string    `json:"notes" db:"notes"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
