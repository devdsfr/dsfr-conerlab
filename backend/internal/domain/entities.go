package domain

import (
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/pkg/devaccess"
)

// League representa um campeonato (ex: Brasileirão Série A)
type League struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Country   string    `json:"country" db:"country"`
	Tier      string    `json:"tier" db:"tier"` // ex: G6, G4 etc para classificação de força
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Season representa uma temporada de um campeonato (ex: 2024)
type Season struct {
	ID       int64  `json:"id" db:"id"`
	LeagueID int64  `json:"league_id" db:"league_id"`
	Year     int    `json:"year" db:"year"`
	Label    string `json:"label" db:"label"` // ex: "2024" ou "2024/2025"
}

// Team representa uma equipe
type Team struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	ShortName string    `json:"short_name" db:"short_name"`
	Country   string    `json:"country" db:"country"`
	Tier      string    `json:"tier" db:"tier"` // força do adversário (ex: G6)
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Match representa uma partida com dados de escanteios (mercado único do MVP)
type Match struct {
	ID          int64     `json:"id" db:"id"`
	LeagueID    int64     `json:"league_id" db:"league_id"`
	SeasonID    int64     `json:"season_id" db:"season_id"`
	Round       int       `json:"round" db:"round"`
	MatchDate   time.Time `json:"match_date" db:"match_date"`
	HomeTeamID  int64     `json:"home_team_id" db:"home_team_id"`
	AwayTeamID  int64     `json:"away_team_id" db:"away_team_id"`
	HomeCorners int       `json:"home_corners" db:"home_corners"`
	AwayCorners int       `json:"away_corners" db:"away_corners"`
	HomeGoals   int       `json:"home_goals" db:"home_goals"`
	AwayGoals   int       `json:"away_goals" db:"away_goals"`

	// Estatísticas complementares vindas do mesmo /fixtures/statistics da
	// API-Football — nullable porque nem toda partida (ligas menores, sobretudo) tem
	// 100% dos campos publicados pelo provedor. Prioridade Alta (posse, chutes, chutes
	// no alvo, cartões) já vinha sendo persistida desde a migration 004 mas nunca
	// exposta; Prioridade Média (chutes de área, bloqueados, faltas, impedimentos)
	// adicionada na migration 010.
	HomePossession      *int `json:"home_possession,omitempty" db:"home_possession"`
	AwayPossession      *int `json:"away_possession,omitempty" db:"away_possession"`
	HomeShots           *int `json:"home_shots,omitempty" db:"home_shots"`
	AwayShots           *int `json:"away_shots,omitempty" db:"away_shots"`
	HomeShotsOnTarget   *int `json:"home_shots_on_target,omitempty" db:"home_shots_on_target"`
	AwayShotsOnTarget   *int `json:"away_shots_on_target,omitempty" db:"away_shots_on_target"`
	HomeYellowCards     *int `json:"home_yellow_cards,omitempty" db:"home_yellow_cards"`
	AwayYellowCards     *int `json:"away_yellow_cards,omitempty" db:"away_yellow_cards"`
	HomeRedCards        *int `json:"home_red_cards,omitempty" db:"home_red_cards"`
	AwayRedCards        *int `json:"away_red_cards,omitempty" db:"away_red_cards"`
	HomeShotsInsidebox  *int `json:"home_shots_insidebox,omitempty" db:"home_shots_insidebox"`
	AwayShotsInsidebox  *int `json:"away_shots_insidebox,omitempty" db:"away_shots_insidebox"`
	HomeShotsOutsidebox *int `json:"home_shots_outsidebox,omitempty" db:"home_shots_outsidebox"`
	AwayShotsOutsidebox *int `json:"away_shots_outsidebox,omitempty" db:"away_shots_outsidebox"`
	HomeBlockedShots    *int `json:"home_blocked_shots,omitempty" db:"home_blocked_shots"`
	AwayBlockedShots    *int `json:"away_blocked_shots,omitempty" db:"away_blocked_shots"`
	HomeFouls           *int `json:"home_fouls,omitempty" db:"home_fouls"`
	AwayFouls           *int `json:"away_fouls,omitempty" db:"away_fouls"`
	HomeOffsides        *int `json:"home_offsides,omitempty" db:"home_offsides"`
	AwayOffsides        *int `json:"away_offsides,omitempty" db:"away_offsides"`

	// CornerOdds mapeia "linha" (ex: "4.5", "5.5" ... "10.5") -> odd histórica registrada
	// para o mercado "mais de X escanteios". Usado pelo motor de filtros/backtesting e
	// pelo simulador financeiro para calcular ROI/yield/lucro de forma reproduzível.
	CornerOdds map[string]float64 `json:"corner_odds" db:"corner_odds"`
	CreatedAt  time.Time          `json:"created_at" db:"created_at"`
}

// OddForThreshold retorna a odd histórica para "mais de N escanteios" (equivalente à
// linha N+0.5), e um bool indicando se a odd está disponível.
func (m Match) OddForThreshold(threshold int) (float64, bool) {
	if m.CornerOdds == nil {
		return 0, false
	}
	key := fmt.Sprintf("%d.5", threshold)
	v, ok := m.CornerOdds[key]
	return v, ok
}

// TotalCorners retorna o total de escanteios da partida.
func (m Match) TotalCorners() int {
	return m.HomeCorners + m.AwayCorners
}

// TeamMatchView é uma visão "por equipe" de uma partida — usada para estatísticas
// de uma equipe específica (a favor/sofridos, mandante/visitante).
type TeamMatchView struct {
	MatchID        int64     `json:"match_id"`
	MatchDate      time.Time `json:"match_date"`
	Opponent       Team      `json:"opponent"`
	IsHome         bool      `json:"is_home"`
	CornersFor     int       `json:"corners_for"`
	CornersAgainst int       `json:"corners_against"`
	TotalCorners   int       `json:"total_corners"`
	OpponentTier   string    `json:"opponent_tier"`

	// Estatísticas complementares (ver comentário em Match), já reorientadas pela
	// perspectiva da equipe consultada (For = a própria equipe, Against = o
	// adversário) — mesmo padrão de CornersFor/CornersAgainst. Nil quando o provedor
	// não publicou aquele campo para a partida.
	PossessionFor          *int `json:"possession_for,omitempty"`
	PossessionAgainst      *int `json:"possession_against,omitempty"`
	ShotsFor               *int `json:"shots_for,omitempty"`
	ShotsAgainst           *int `json:"shots_against,omitempty"`
	ShotsOnTargetFor       *int `json:"shots_on_target_for,omitempty"`
	ShotsOnTargetAgainst   *int `json:"shots_on_target_against,omitempty"`
	ShotsInsideboxFor      *int `json:"shots_insidebox_for,omitempty"`
	ShotsInsideboxAgainst  *int `json:"shots_insidebox_against,omitempty"`
	ShotsOutsideboxFor     *int `json:"shots_outsidebox_for,omitempty"`
	ShotsOutsideboxAgainst *int `json:"shots_outsidebox_against,omitempty"`
	BlockedShotsFor        *int `json:"blocked_shots_for,omitempty"`
	BlockedShotsAgainst    *int `json:"blocked_shots_against,omitempty"`
	FoulsFor               *int `json:"fouls_for,omitempty"`
	FoulsAgainst           *int `json:"fouls_against,omitempty"`
	OffsidesFor            *int `json:"offsides_for,omitempty"`
	OffsidesAgainst        *int `json:"offsides_against,omitempty"`
	YellowCardsFor         *int `json:"yellow_cards_for,omitempty"`
	YellowCardsAgainst     *int `json:"yellow_cards_against,omitempty"`
	RedCardsFor            *int `json:"red_cards_for,omitempty"`
	RedCardsAgainst        *int `json:"red_cards_against,omitempty"`
}

// User representa um usuário da plataforma
type User struct {
	ID           int64     `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`

	// Campos da Assinatura Premium (ver migration 005_billing.sql). Plan/
	// SubscriptionStatus são a fonte de verdade para o gate de acesso
	// (middleware.RequirePremium); os IDs do Stripe conectam este usuário ao
	// Customer/Subscription correspondente; TrialEndsAt/CurrentPeriodEnd vêm dos
	// webhooks do Stripe e são usados só para exibição.
	Plan                 string     `json:"plan" db:"plan"`
	StripeCustomerID     *string    `json:"-" db:"stripe_customer_id"`
	StripeSubscriptionID *string    `json:"-" db:"stripe_subscription_id"`
	SubscriptionStatus   string     `json:"subscription_status" db:"subscription_status"`
	TrialEndsAt          *time.Time `json:"trial_ends_at,omitempty" db:"trial_ends_at"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty" db:"current_period_end"`
}

// IsPremium indica se o usuário tem acesso aos recursos pagos agora (assinatura
// ativa ou dentro do período de trial, OU o e-mail está na liberação manual de
// desenvolvimento — ver pkg/devaccess/DEV_PREMIUM_EMAILS). Esta é a única lógica
// de decisão de acesso — usada tanto pelo middleware.RequirePremium quanto por
// qualquer outro ponto do backend que precise checar o plano do usuário.
func (u User) IsPremium() bool {
	if devaccess.IsPremium(u.Email) {
		return true
	}
	return u.SubscriptionStatus == "active" || u.SubscriptionStatus == "trialing"
}

// PasswordResetToken representa uma solicitação de "esqueci minha senha" pendente.
// Token de uso único, com expiração curta — ver migration 006_password_reset.sql e
// AuthUsecase.ForgotPassword/ResetPassword.
type PasswordResetToken struct {
	ID        int64      `json:"id" db:"id"`
	UserID    int64      `json:"user_id" db:"user_id"`
	Token     string     `json:"-" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// SavedFilter representa um filtro personalizado salvo pelo usuário (Módulo 3)
type SavedFilter struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Definition  string    `json:"definition" db:"definition"` // JSON serializado do FilterCriteria
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// BetStatus representa o status de uma aposta registrada
type BetStatus string

const (
	BetStatusPending BetStatus = "pending"
	BetStatusWon     BetStatus = "won"
	BetStatusLost    BetStatus = "lost"
	BetStatusVoid    BetStatus = "void"
)

// Bet representa uma aposta registrada manualmente pelo usuário (Módulo 5)
type Bet struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	MatchLabel string    `json:"match_label" db:"match_label"`
	LeagueID   *int64    `json:"league_id" db:"league_id"`
	Market     string    `json:"market" db:"market"`
	Odd        float64   `json:"odd" db:"odd"`
	Stake      float64   `json:"stake" db:"stake"`
	Status     BetStatus `json:"status" db:"status"`
	ProfitLoss float64   `json:"profit_loss" db:"profit_loss"`
	EventDate  time.Time `json:"event_date" db:"event_date"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
