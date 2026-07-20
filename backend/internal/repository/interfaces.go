package repository

import (
	"context"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

type LeagueRepository interface {
	List(ctx context.Context) ([]domain.League, error)
	GetByID(ctx context.Context, id int64) (*domain.League, error)
	ListSeasons(ctx context.Context, leagueID int64) ([]domain.Season, error)
}

type TeamRepository interface {
	List(ctx context.Context, leagueID *int64) ([]domain.Team, error)
	GetByID(ctx context.Context, id int64) (*domain.Team, error)
	Search(ctx context.Context, query string) ([]domain.Team, error)
}

// MatchFilter agrupa os parâmetros de consulta usados pelo Dashboard e Comparador.
type MatchFilter struct {
	TeamID   int64
	LeagueID *int64
	SeasonID *int64
	Limit    int // últimos N jogos
	HomeOnly bool
	AwayOnly bool
}

type MatchRepository interface {
	// TeamMatches retorna os jogos de uma equipe (mais recentes primeiro), já
	// convertidos para a visão "por equipe" (a favor/sofridos, casa/fora).
	TeamMatches(ctx context.Context, f MatchFilter) ([]domain.TeamMatchView, error)

	// AllMatches retorna partidas brutas (ambas as equipes) para uso no motor de
	// filtros/backtesting, com filtro opcional por temporadas.
	AllMatches(ctx context.Context, leagueID int64, seasonIDs []int64) ([]domain.Match, error)

	GetMatchTeams(ctx context.Context, matchIDs []int64) (map[int64]domain.Match, error)
}

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)

	// GetByStripeCustomerID localiza o usuário dono de um Customer do Stripe — usado
	// pelos webhooks de assinatura (customer.subscription.updated/deleted), que só
	// trazem o ID do customer, nunca o ID interno do usuário.
	GetByStripeCustomerID(ctx context.Context, customerID string) (*domain.User, error)

	// SetStripeCustomerID grava o vínculo usuário <-> Customer do Stripe. Chamado uma
	// única vez, na primeira criação de Checkout Session do usuário (ou reaproveitado
	// se o usuário já tiver um customer_id de uma sessão anterior).
	SetStripeCustomerID(ctx context.Context, userID int64, customerID string) error

	// UpdateSubscriptionByCustomerID aplica o estado mais recente da assinatura
	// (vindo de um evento de webhook do Stripe) ao usuário dono do customerID
	// informado. plan é derivado do status: 'premium' quando ativo/trialing, 'free'
	// caso contrário — mantido aqui (e não calculado toda hora no domínio) para que
	// o histórico de plano fique estável mesmo se a lógica de IsPremium mudar depois.
	UpdateSubscriptionByCustomerID(ctx context.Context, customerID, subscriptionID, status, plan string, trialEndsAt, currentPeriodEnd *time.Time) error

	// UpdatePassword grava um novo hash de senha (bcrypt) — usado tanto pelo fluxo de
	// "esqueci minha senha" (AuthUsecase.ResetPassword) quanto por uma futura troca de
	// senha autenticada, se vier a existir.
	UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
}

// PasswordResetRepository persiste as solicitações de "esqueci minha senha" (ver
// domain.PasswordResetToken e migration 006_password_reset.sql).
type PasswordResetRepository interface {
	Create(ctx context.Context, t *domain.PasswordResetToken) error

	// GetValidByToken retorna o token apenas se ele existir, não tiver expirado e
	// ainda não tiver sido usado — qualquer outra condição é tratada pelo chamador
	// como "token inválido", sem distinguir o motivo (evita vazar informação sobre
	// se o token existiu algum dia).
	GetValidByToken(ctx context.Context, token string) (*domain.PasswordResetToken, error)

	MarkUsed(ctx context.Context, id int64) error

	// InvalidateAllForUser marca como usados todos os tokens pendentes de um usuário
	// — chamado ao gerar um novo token, para que apenas o link mais recente enviado
	// por e-mail funcione (evita links antigos "esquecidos" na caixa de entrada
	// continuarem válidos indefinidamente).
	InvalidateAllForUser(ctx context.Context, userID int64) error
}

type FilterRepository interface {
	Create(ctx context.Context, f *domain.SavedFilter) error
	List(ctx context.Context, userID int64) ([]domain.SavedFilter, error)
	GetByID(ctx context.Context, id int64) (*domain.SavedFilter, error)
	Update(ctx context.Context, f *domain.SavedFilter) error
	Delete(ctx context.Context, id int64, userID int64) error
}

type BetRepository interface {
	Create(ctx context.Context, b *domain.Bet) error
	List(ctx context.Context, userID int64) ([]domain.Bet, error)
	Update(ctx context.Context, b *domain.Bet) error
	Delete(ctx context.Context, id int64, userID int64) error
}

type AlertRuleRepository interface {
	Create(ctx context.Context, r *domain.AlertRule) error
	List(ctx context.Context, userID int64) ([]domain.AlertRule, error)
	GetByID(ctx context.Context, id int64) (*domain.AlertRule, error)
	Delete(ctx context.Context, id int64, userID int64) error
}

type StrategyHistoryRepository interface {
	Create(ctx context.Context, e *domain.StrategyHistoryEntry) error
	List(ctx context.Context, userID int64) ([]domain.StrategyHistoryEntry, error)
	// TopPerforming retorna, entre todos os usuários, as execuções de filtro com
	// melhor ROI registrado — usado no Dashboard Executivo ("Top filtros").
	TopPerforming(ctx context.Context, limit int) ([]domain.StrategyHistoryEntry, error)
}

// TeamAggregate é o resultado de uma agregação por equipe usada em rankings e
// análises de adversário (Módulo de Inteligência Estatística).
type TeamAggregate struct {
	Team           domain.Team
	SampleSize     int
	AvgFor         float64
	AvgAgainst     float64
	AvgTotal       float64
	StdDevTotal    float64
	ConsistencyIdx float64
}

type LeagueStatsRepository interface {
	// TeamAggregates calcula, para cada equipe do campeonato/temporadas informados,
	// as médias de escanteios a favor/sofridos/total considerando até `limit` jogos
	// mais recentes de cada equipe (0 = todos os jogos do período).
	TeamAggregates(ctx context.Context, leagueID int64, seasonIDs []int64, limit int) ([]TeamAggregate, error)
}

// BankrollRepository persiste a configuração e o estado do Módulo de Gestão Evolutiva
// de Banca (fases, critérios de evolução, estado atual e histórico de mudanças).
type BankrollRepository interface {
	ListPhases(ctx context.Context, userID int64) ([]domain.BankrollPhase, error)
	// ReplacePhases substitui toda a sequência de fases configurada pelo usuário e
	// retorna a lista resultante (ordenada por sequência).
	ReplacePhases(ctx context.Context, userID int64, phases []domain.BankrollPhase) ([]domain.BankrollPhase, error)

	// GetCriteria nunca retorna erro por ausência de configuração — devolve valores
	// padrão sensatos na primeira vez que o usuário acessa o módulo.
	GetCriteria(ctx context.Context, userID int64) (domain.BankrollCriteria, error)
	SaveCriteria(ctx context.Context, c domain.BankrollCriteria) error

	// GetState retorna nil (sem erro) se o usuário ainda não inicializou o módulo.
	GetState(ctx context.Context, userID int64) (*domain.BankrollState, error)
	InitState(ctx context.Context, userID int64) (*domain.BankrollState, error)
	SetPhase(ctx context.Context, userID int64, newSequence int) (*domain.BankrollState, error)

	AddHistory(ctx context.Context, e *domain.BankrollHistoryEntry) error
	ListHistory(ctx context.Context, userID int64) ([]domain.BankrollHistoryEntry, error)

	// AddRound e ListRounds persistem o registro de rodadas confirmadas
	// manualmente (ver domain.BankrollRound) — a base do saldo real acumulado.
	AddRound(ctx context.Context, r *domain.BankrollRound) error
	ListRounds(ctx context.Context, userID int64) ([]domain.BankrollRound, error)
}
