// Package discovery implementa a Fase 6 da Remodelagem: o Strategy Discovery
// Engine (Remodelagem/08-strategy-discovery-engine.md).
//
// Inversão de fluxo em relação ao Simulador de Filtros: em vez de o usuário
// montar um filtro e executá-lo, o sistema combina automaticamente as variáveis
// do espaço de busca, executa backtest em cada combinação, descarta tudo que não
// passa nos critérios mínimos e publica apenas os padrões consistentes como
// estratégias do sistema (owner_id NULL, origin='discovery', visibility='public').
//
// O engine NÃO tem motor de backtest próprio: ele reaproveita usecase.FilterUsecase,
// o mesmo usado pela tela do Simulador. Isso é deliberado — garante que o número
// mostrado no ranking de descobertas seja reproduzível pelo usuário na interface.
// A única diferença é que aqui os repositórios são memoizados por ciclo (cache.go),
// já que centenas de backtests leem exatamente as mesmas partidas.
//
// Regras do doc 08 respeitadas: nunca recomendar apostas; nunca publicar
// estratégia com amostra insuficiente; nunca considerar apenas Win Rate.
package discovery

import (
	"context"
	"fmt"
	"sort"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
)

// ResultPersister grava um resultado de backtest já calculado junto com health e
// scores. Implementado por *strategyengine.Engine (F4) — declarado como interface
// aqui para manter o Discovery testável sem banco.
type ResultPersister interface {
	PersistResult(ctx context.Context, strategyID int64, res *usecase.BacktestResult) (*strategyengine.Evaluation, error)
}

// Options configura um ciclo de descoberta.
type Options struct {
	Criteria Criteria

	// IncludeTeams liga a varredura por equipe individual além da varredura
	// geral da liga. Multiplica o custo do ciclo pelo número de equipes.
	IncludeTeams bool
}

// Engine é o motor de descoberta. Não guarda estado entre ciclos.
type Engine struct {
	matches    repository.MatchRepository
	teams      repository.TeamRepository
	leagues    repository.LeagueRepository
	strategies repository.StrategyRepository
	persister  ResultPersister
	opts       Options
}

func NewEngine(
	matches repository.MatchRepository,
	teams repository.TeamRepository,
	leagues repository.LeagueRepository,
	strategies repository.StrategyRepository,
	persister ResultPersister,
	opts Options,
) *Engine {
	opts.Criteria = opts.Criteria.withDefaults()
	return &Engine{
		matches: matches, teams: teams, leagues: leagues,
		strategies: strategies, persister: persister, opts: opts,
	}
}

// LeagueResult resume a descoberta de uma liga (observabilidade do doc 15).
type LeagueResult struct {
	LeagueID     int64          `json:"league_id"`
	LeagueName   string         `json:"league_name"`
	Seasons      int            `json:"seasons"`
	Combinations int            `json:"combinations"`
	Approved     int            `json:"approved"`
	Published    int            `json:"published"`
	Deactivated  int            `json:"deactivated"`
	Errors       int            `json:"errors"`
	Rejections   map[string]int `json:"rejections"`
}

// Result resume um ciclo completo (todas as ligas).
type Result struct {
	Leagues      int            `json:"leagues"`
	Combinations int            `json:"combinations"`
	Published    int            `json:"published"`
	Deactivated  int            `json:"deactivated"`
	Errors       int            `json:"errors"`
	ByLeague     []LeagueResult `json:"by_league"`
}

// RunAll roda a descoberta em todas as ligas cadastradas. Uma liga que falha não
// interrompe as demais (mesma resiliência dos outros workers do pipeline).
func (e *Engine) RunAll(ctx context.Context) (Result, error) {
	var out Result

	leagues, err := e.leagues.List(ctx)
	if err != nil {
		return out, fmt.Errorf("listar ligas: %w", err)
	}

	for _, l := range leagues {
		lr, err := e.RunLeague(ctx, l.ID, nil)
		if err != nil {
			out.Errors++
			continue
		}
		out.Leagues++
		out.Combinations += lr.Combinations
		out.Published += lr.Published
		out.Deactivated += lr.Deactivated
		out.Errors += lr.Errors
		out.ByLeague = append(out.ByLeague, lr)
	}
	return out, nil
}

// candidate é uma combinação que passou na validação, aguardando publicação.
type candidate struct {
	combo  combo
	result *usecase.BacktestResult
	dsfr   float64
}

// RunLeague executa o ciclo de descoberta de uma liga. seasonIDs vazio = todas as
// temporadas cadastradas da liga.
func (e *Engine) RunLeague(ctx context.Context, leagueID int64, seasonIDs []int64) (LeagueResult, error) {
	res := LeagueResult{LeagueID: leagueID, Rejections: map[string]int{}}

	league, err := e.leagues.GetByID(ctx, leagueID)
	if err != nil {
		return res, fmt.Errorf("carregar liga %d: %w", leagueID, err)
	}
	if league == nil {
		return res, fmt.Errorf("liga %d não encontrada", leagueID)
	}
	res.LeagueName = league.Name

	if len(seasonIDs) == 0 {
		seasons, err := e.leagues.ListSeasons(ctx, leagueID)
		if err != nil {
			return res, fmt.Errorf("listar temporadas da liga %d: %w", leagueID, err)
		}
		for _, s := range seasons {
			seasonIDs = append(seasonIDs, s.ID)
		}
	}
	res.Seasons = len(seasonIDs)
	if res.Seasons == 0 {
		return res, nil // liga sem temporadas: nada a minerar, não é erro
	}

	// Repositórios memoizados: as partidas da liga são lidas do Postgres UMA vez
	// e reutilizadas por todas as combinações deste ciclo (ver cache.go).
	matches := newCachedMatchRepo(e.matches)
	teamsRepo := newCachedTeamRepo(e.teams)
	filters := usecase.NewFilterUsecase(matches, teamsRepo, e.leagues)

	teams, err := teamsRepo.List(ctx, &leagueID, seasonIDs...)
	if err != nil {
		return res, fmt.Errorf("listar equipes da liga %d: %w", leagueID, err)
	}

	combos := generateCombos(teams, e.opts.IncludeTeams)
	res.Combinations = len(combos)

	approved := e.mine(ctx, filters, leagueID, seasonIDs, combos, &res)
	res.Approved = len(approved)

	published, err := e.publish(ctx, league.Name, leagueID, seasonIDs, approved, &res)
	if err != nil {
		return res, err
	}
	res.Published = len(published)

	deactivated, err := e.strategies.DeactivateDiscoveredExcept(ctx, leagueID, published)
	if err != nil {
		return res, fmt.Errorf("desativar descobertas obsoletas: %w", err)
	}
	res.Deactivated = deactivated

	return res, nil
}

// mine executa o backtest de cada combinação e aplica os critérios do doc 08.
// Erros de uma combinação isolada (ex.: definição inválida) são contados e o
// ciclo segue — uma combinação ruim não pode invalidar a varredura inteira.
func (e *Engine) mine(
	ctx context.Context,
	filters *usecase.FilterUsecase,
	leagueID int64,
	seasonIDs []int64,
	combos []combo,
	res *LeagueResult,
) []candidate {
	crit := e.opts.Criteria
	var approved []candidate

	for _, c := range combos {
		if ctx.Err() != nil {
			return approved // shutdown pedido: devolve o que já foi minerado
		}

		criteria := usecase.FilterCriteria{
			TeamID:           c.teamID,
			LastNGames:       c.window,
			HomeAway:         c.homeAway,
			CornersThreshold: c.line,
			OpponentTier:     c.tier,
			MaxOdds:          c.maxOdds,
			Metric:           "corners",
		}

		// maxAgeDays = 0: o pipeline sempre minera o histórico completo. O cap de
		// 90 dias é uma regra do plano gratuito na navegação, não da descoberta.
		result, err := filters.RunBacktest(ctx, leagueID, seasonIDs, criteria, 0)
		if err != nil {
			res.Errors++
			continue
		}

		if reason := crit.validate(result); reason != "" {
			res.Rejections[string(reason)]++
			continue
		}

		// Último filtro (doc 08: faixa "Descartar" = score < 40). O score é o mesmo
		// que a estratégia receberá ao ser persistida.
		dsfr := strategyengine.PreviewScores(result).DSFRScore
		if dsfr < crit.MinDSFR {
			res.Rejections[string(rejectScore)]++
			continue
		}

		approved = append(approved, candidate{combo: c, result: result, dsfr: dsfr})
	}
	return approved
}

// publish ordena as aprovadas por DSFR Score, aplica o teto por liga e grava cada
// uma como estratégia do sistema + backtest + health + scores. Retorna os IDs
// publicados (entrada do DeactivateDiscoveredExcept).
func (e *Engine) publish(
	ctx context.Context,
	leagueName string,
	leagueID int64,
	seasonIDs []int64,
	approved []candidate,
	res *LeagueResult,
) ([]int64, error) {
	// Ordenação estável e determinística: empate no score é desempatado pelo nome,
	// para que dois ciclos idênticos publiquem exatamente o mesmo conjunto.
	sort.SliceStable(approved, func(i, j int) bool {
		if approved[i].dsfr != approved[j].dsfr {
			return approved[i].dsfr > approved[j].dsfr
		}
		return approved[i].combo.name(leagueName) < approved[j].combo.name(leagueName)
	})
	if len(approved) > e.opts.Criteria.MaxPerLeague {
		approved = approved[:e.opts.Criteria.MaxPerLeague]
	}

	published := make([]int64, 0, len(approved))
	for _, c := range approved {
		definition, err := c.combo.definition(leagueID, seasonIDs)
		if err != nil {
			res.Errors++
			continue
		}

		s := &domain.Strategy{
			Name:        c.combo.name(leagueName),
			Description: describe(c, leagueName),
			Definition:  definition,
			Origin:      "discovery",
			Visibility:  "public",
			Active:      true,
		}
		if err := e.strategies.UpsertDiscovered(ctx, s); err != nil {
			res.Errors++
			continue
		}
		if _, err := e.persister.PersistResult(ctx, s.ID, c.result); err != nil {
			res.Errors++
			continue
		}
		published = append(published, s.ID)
	}
	return published, nil
}

// describe monta a explicação em linguagem analítica da descoberta.
//
// Princípio da remodelagem: "explicação acima de números". O texto é puramente
// descritivo do que ocorreu no histórico — nunca projeta o futuro, nunca sugere
// entrada e sempre carrega o período/amostra que fundamenta os números, conforme
// as regras do doc 08 e do disclaimer global da plataforma.
func describe(c candidate, leagueName string) string {
	r := c.result

	scope := "considerando todas as equipes"
	if c.combo.teamName != "" {
		scope = "considerando apenas o " + c.combo.teamName
	}

	filters := homeAwayLabel(c.combo.homeAway)
	if c.combo.window > 0 {
		filters += fmt.Sprintf(", janela dos últimos %d jogos de cada equipe", c.combo.window)
	}
	if c.combo.tier != "" {
		filters += fmt.Sprintf(", apenas contra adversários do grupo %s", c.combo.tier)
	}

	return fmt.Sprintf(
		"Padrão identificado automaticamente pela mineração do histórico do %s, %s (%s), "+
			"restrito a jogos com odd registrada até %.2f. "+
			"Em %d ocorrências analisadas, o total de escanteios ficou acima de %d.5 em %.1f%% delas. "+
			"No mesmo período o retorno histórico foi de %.2f%% (yield %.2f%%), com lucro acumulado de %.2f unidades "+
			"e drawdown máximo de %.1f%% do capital movimentado. "+
			"Classificação DSFR: %s (score %.1f). "+
			"Números apurados sobre dados históricos armazenados — não constituem recomendação de aposta "+
			"nem previsão de resultados futuros.",
		leagueName, scope, filters, c.combo.maxOdds,
		r.MatchCount, c.combo.line, r.HitRate,
		r.ROI, r.Yield, r.Profit, drawdownPct(r),
		Classify(c.dsfr), c.dsfr,
	)
}
