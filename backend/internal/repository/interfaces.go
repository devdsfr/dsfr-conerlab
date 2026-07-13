package repository

import (
	"context"

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
