package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// O Discovery Engine executa centenas de backtests sobre EXATAMENTE o mesmo
// conjunto de partidas — o que muda entre uma combinação e outra são só os
// critérios aplicados em memória. Como usecase.FilterUsecase recarrega as
// partidas do Postgres a cada RunBacktest, rodar 180 combinações de uma liga
// significaria 180 varreduras idênticas da tabela `matches`.
//
// A solução aqui é decorar os repositórios com um memoizador de vida curta
// (dura só um ciclo de descoberta) em vez de mudar a assinatura do
// FilterUsecase: o motor de backtest continua sendo exatamente o mesmo código
// usado pelo Simulador de Filtros, garantindo que uma estratégia descoberta
// produza os mesmos números quando o usuário a reexecutar na tela.
//
// Escopo de uso: um cachedMatchRepo é criado por ciclo e descartado no fim.
// Não é um cache de aplicação e nunca é compartilhado entre requisições HTTP.

// cachedMatchRepo memoiza AllMatches por (liga, temporadas).
type cachedMatchRepo struct {
	repository.MatchRepository

	mu    sync.Mutex
	cache map[string][]domain.Match
}

func newCachedMatchRepo(inner repository.MatchRepository) *cachedMatchRepo {
	return &cachedMatchRepo{MatchRepository: inner, cache: map[string][]domain.Match{}}
}

// AllMatches devolve a MESMA fatia em todas as chamadas com a mesma chave. O
// motor de backtest só lê as partidas (nunca as ordena nem as altera in-place),
// então compartilhar o array é seguro e evita copiar milhares de structs por
// combinação avaliada.
func (r *cachedMatchRepo) AllMatches(ctx context.Context, leagueID int64, seasonIDs []int64) ([]domain.Match, error) {
	key := matchesKey(leagueID, seasonIDs)

	r.mu.Lock()
	cached, hit := r.cache[key]
	r.mu.Unlock()
	if hit {
		return cached, nil
	}

	matches, err := r.MatchRepository.AllMatches(ctx, leagueID, seasonIDs)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[key] = matches
	r.mu.Unlock()
	return matches, nil
}

// cachedTeamRepo memoiza List por (liga, temporadas) — o FilterUsecase monta um
// índice de equipes a cada backtest para resolver nomes e o tier do adversário.
type cachedTeamRepo struct {
	repository.TeamRepository

	mu    sync.Mutex
	cache map[string][]domain.Team
}

func newCachedTeamRepo(inner repository.TeamRepository) *cachedTeamRepo {
	return &cachedTeamRepo{TeamRepository: inner, cache: map[string][]domain.Team{}}
}

func (r *cachedTeamRepo) List(ctx context.Context, leagueID *int64, seasonIDs ...int64) ([]domain.Team, error) {
	var id int64 = -1
	if leagueID != nil {
		id = *leagueID
	}
	key := matchesKey(id, seasonIDs)

	r.mu.Lock()
	cached, hit := r.cache[key]
	r.mu.Unlock()
	if hit {
		return cached, nil
	}

	teams, err := r.TeamRepository.List(ctx, leagueID, seasonIDs...)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[key] = teams
	r.mu.Unlock()
	return teams, nil
}

// matchesKey é determinística: a ordem das temporadas importa para o repositório,
// então é preservada aqui em vez de ordenada.
func matchesKey(leagueID int64, seasonIDs []int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", leagueID)
	for _, s := range seasonIDs {
		fmt.Fprintf(&b, ":%d", s)
	}
	return b.String()
}
