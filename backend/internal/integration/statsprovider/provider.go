// Package statsprovider define o contrato do Módulo de Sincronização de Dados
// (Statistics Provider): qualquer fonte externa de estatísticas (API-Football,
// Sofascore, SportMonks, Opta, StatsBomb, ...) implementa StatisticsProvider e é
// plugada por configuração (env STATISTICS_PROVIDER), sem que cmd/worker ou os
// usecases de sincronização precisem conhecer o formato específico de cada API.
//
// Este pacote é intencionalmente separado de internal/integration/sportsdata: aquele
// pacote resolve "buscar partidas e devolver pro chamador persistir"
// (usado por cmd/sync, um comando manual de importação em lote). Este aqui resolve
// "manter o Postgres continuamente sincronizado em background" (usado por
// cmd/worker), com um contrato mais rico (status da partida, estatísticas completas,
// classificação, e verificação de saúde do provedor). Nada impede o mesmo cliente HTTP
// (ex: apifootball.Client) de implementar as duas interfaces ao mesmo tempo — é
// exatamente o que a implementação da API-Football faz, reaproveitando a mesma
// autenticação e o mesmo registro de uso (internal/usagelog).
package statsprovider

import (
	"context"
	"errors"
	"time"
)

// ErrNotImplemented é o erro retornado por um provedor cuja integração real ainda não
// existe (ex: SofascoreProvider antes de termos um método de acesso legítimo aos dados
// deles). Workers tratam esse erro como uma falha comum de ciclo: registram, não
// derrubam a aplicação, e tentam de novo no próximo ciclo.
var ErrNotImplemented = errors.New("provedor de estatísticas não implementado")

// Competition é um campeonato normalizado.
type Competition struct {
	ExternalID string
	Name       string
	Country    string
}

// TeamInfo é uma equipe normalizada.
type TeamInfo struct {
	ExternalID string
	Name       string
	ShortName  string
	Country    string
}

// FixtureStatus é o estado do ciclo de vida de uma partida no pipeline de
// sincronização — ver coluna matches.status (migrations/004_statistics_sync.sql).
type FixtureStatus string

const (
	FixtureScheduled FixtureStatus = "AGENDADO"
	FixtureFinished  FixtureStatus = "FINALIZADO"
)

// Fixture é uma partida normalizada, com o suficiente para descoberta (Worker 1):
// ainda não traz estatísticas detalhadas, só o necessário para saber que o jogo existe
// e quando ele acontece.
type Fixture struct {
	ExternalID         string
	CompetitionExtID   string
	SeasonYear         int
	Round              int
	MatchDate          time.Time
	Status             FixtureStatus
	HomeTeamExternalID string
	HomeTeamName       string
	AwayTeamExternalID string
	AwayTeamName       string
	HomeGoals          int
	AwayGoals          int
}

// FixtureStatistics é o conjunto completo de estatísticas pós-jogo (Worker 2), no
// mesmo formato exigido pelo critério de aceite (escanteios, posse, chutes, cartões,
// árbitro, local). Ponteiros nil indicam "o provedor não retornou esse campo" —
// nunca é gravado como zero para não confundir "não sincronizado" com "zero de fato".
type FixtureStatistics struct {
	ExternalID string
	Status     FixtureStatus
	HomeGoals  int
	AwayGoals  int

	HomeCorners *int
	AwayCorners *int

	HomePossessionPct *int
	AwayPossessionPct *int
	HomeShots         *int
	AwayShots         *int
	HomeShotsOnTarget *int
	AwayShotsOnTarget *int
	HomeYellowCards   *int
	AwayYellowCards   *int
	HomeRedCards      *int
	AwayRedCards      *int

	Referee string
	Venue   string
}

// StandingEntry é uma linha da tabela de classificação de um campeonato/temporada.
// Exposta pela interface conforme o critério de aceite; a persistência (tabela de
// classificação) fica para a fase 2 deste módulo — por ora nenhum worker consome este
// método.
type StandingEntry struct {
	TeamExternalID string
	TeamName       string
	Position       int
	Played         int
	Won            int
	Drawn          int
	Lost           int
	Points         int
}

// HealthResult é o resultado de uma verificação de saúde do provedor (Worker de
// Health Check, item pedido explicitamente pelo usuário por depender de uma API
// não-oficial como o Sofascore).
type HealthResult struct {
	OK           bool
	ResponseTime time.Duration
	Message      string // detalhe do problema quando OK=false (ex: "HTTP 429", "campo 'Corner Kicks' ausente na resposta")
}

// StatisticsProvider é o contrato que qualquer fonte de dados esportivos precisa
// implementar para alimentar o Módulo de Sincronização. Trocar de provedor é uma
// mudança de configuração (STATISTICS_PROVIDER), nunca uma mudança nos workers ou nos
// usecases que os chamam.
type StatisticsProvider interface {
	Name() string

	// SyncCompetitions lista os campeonatos disponíveis no provedor que casam com o
	// nome/país informados (normalmente usado uma única vez, na configuração inicial
	// de um novo campeonato).
	SyncCompetitions(ctx context.Context, name, country string) ([]Competition, error)

	// SyncTeams lista as equipes de um campeonato/temporada.
	SyncTeams(ctx context.Context, competitionExternalID string, season int) ([]TeamInfo, error)

	// SyncFixtures lista as partidas de um campeonato/temporada (Worker de
	// Descoberta). Não inclui estatísticas detalhadas — só o suficiente para saber que
	// o jogo existe, quando acontece e seu status atual.
	SyncFixtures(ctx context.Context, competitionExternalID string, season int) ([]Fixture, error)

	// SyncFixtureStatistics busca o resultado e as estatísticas completas de uma
	// partida específica (Worker de Atualização). Chamado apenas para partidas cujo
	// status ainda é AGENDADO e cuja data já passou.
	SyncFixtureStatistics(ctx context.Context, fixtureExternalID string) (*FixtureStatistics, error)

	// SyncStandings busca a classificação atual de um campeonato/temporada.
	SyncStandings(ctx context.Context, competitionExternalID string, season int) ([]StandingEntry, error)

	// HealthCheck verifica se o provedor está saudável: endpoint responde, sem
	// bloqueio (403/429), tempo de resposta aceitável e formato de resposta esperado.
	// Nunca deve retornar erro por si só ao detectar um problema — o problema vai
	// dentro de HealthResult; err só indica falha na própria checagem (ex: contexto
	// cancelado).
	HealthCheck(ctx context.Context) (HealthResult, error)
}
