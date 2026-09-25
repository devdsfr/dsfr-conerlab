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
	"strconv"
	"strings"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/progress"
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

	// report acompanha o andamento para a barra de progresso. Nunca é nil
	// (progress.Noop por padrão), então os laços não precisam checar.
	report progress.Reporter
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
		report: progress.Noop{},
	}
}

// PhaseScan é a fase reportada durante a varredura (ver internal/progress).
const PhaseScan = "varredura"

// LeagueResult resume a descoberta de uma liga (observabilidade do doc 15).
type LeagueResult struct {
	LeagueID     int64  `json:"league_id"`
	LeagueName   string `json:"league_name"`
	Seasons      int    `json:"seasons"`
	Combinations int    `json:"combinations"`
	Approved     int    `json:"approved"`

	// AUD-003 — rastro da validação. Sem estes números o ciclo não é auditável:
	// "publiquei 3" não significa nada sem "de quantos testes" e "validadas
	// contra qual período".
	TrainUntil   string  `json:"train_until,omitempty"`  // janela de descoberta: [início, esta data)
	HoldoutFrom  string  `json:"holdout_from,omitempty"` // janela de validação: [esta data, fim]
	Tested       int     `json:"tested"`                 // combinações com p-valor calculável
	FDRThreshold float64 `json:"fdr_threshold"`          // limiar efetivo após correção
	Significant  int     `json:"significant"`            // sobreviveram à correção
	Validated    int     `json:"validated"`              // sobreviveram também fora da amostra

	Published   int            `json:"published"`
	Deactivated int            `json:"deactivated"`
	Errors      int            `json:"errors"`
	Rejections  map[string]int `json:"rejections"`
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

// WithProgress liga o acompanhamento de andamento a este motor, para a barra de
// progresso do botão "Procurar agora". O ciclo do cron roda sem acompanhar.
func (e *Engine) WithProgress(r progress.Reporter) *Engine {
	if r == nil {
		r = progress.Noop{}
	}
	clone := *e
	clone.report = r
	return &clone
}

// RunAll roda a descoberta em todas as ligas cadastradas. Uma liga que falha não
// interrompe as demais (mesma resiliência dos outros workers do pipeline).
func (e *Engine) RunAll(ctx context.Context) (Result, error) {
	var out Result

	leagues, err := e.leagues.List(ctx)
	if err != nil {
		return out, fmt.Errorf("listar ligas: %w", err)
	}

	// A barra anda de campeonato em campeonato: é o único total conhecido antes de
	// começar (a quantidade de combinações só existe depois de carregar os times de
	// cada liga). O texto abaixo da barra mostra o progresso fino, dentro da liga.
	e.report.Phase(PhaseScan, "Minerando combinações", len(leagues))

	for _, l := range leagues {
		e.report.Step(l.Name)
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

	// AUD-003: p-valor na janela de descoberta e resultado da validação fora da
	// amostra. Ambos acompanham o candidato até a publicação para virar texto na
	// descrição da estratégia — o usuário precisa poder ver em que período o
	// padrão foi encontrado e em que período ele se sustentou.
	pValue  float64
	holdout holdoutVerdict
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

	// AUD-003: corte temporal do histórico ANTES de qualquer backtest. A
	// mineração só enxerga a janela de descoberta; a de validação fica
	// intocada até o candidato já estar escolhido.
	allMatches, err := matches.AllMatches(ctx, leagueID, seasonIDs)
	if err != nil {
		return res, fmt.Errorf("carregar partidas da liga %d: %w", leagueID, err)
	}
	train, holdout, splittable := splitHistory(allMatches, e.opts.Criteria.TrainFraction)
	if !splittable {
		// Sem janela de validação não se publica nada. Não é erro do ciclo — é
		// uma liga cujo histórico ainda não sustenta descoberta validável.
		res.Rejections[string(rejectNotSplittable)]++
		res.Combinations = 0
		return res, nil
	}
	res.TrainUntil = train.To.Format("2006-01-02")
	res.HoldoutFrom = holdout.From.Format("2006-01-02")

	combos := generateCombos(teams, e.opts.IncludeTeams)
	res.Combinations = len(combos)

	approved := e.mine(ctx, filters, leagueID, seasonIDs, combos, train, &res)
	approved = e.validate(ctx, filters, leagueID, seasonIDs, approved, holdout, &res)
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

// mine executa o backtest de cada combinação NA JANELA DE DESCOBERTA e aplica os
// critérios do doc 08 mais a correção para testes múltiplos (AUD-003). Erros de
// uma combinação isolada (ex.: definição inválida) são contados e o ciclo segue
// — uma combinação ruim não pode invalidar a varredura inteira.
//
// Duas passagens são necessárias e a ordem não é negociável: o limiar de
// significância depende de QUANTOS testes foram feitos, então nenhum candidato
// pode ser aprovado antes de a varredura inteira terminar. Aprovar na primeira
// passagem seria aplicar um limiar fixo — o defeito que esta correção remove.
func (e *Engine) mine(
	ctx context.Context,
	filters *usecase.FilterUsecase,
	leagueID int64,
	seasonIDs []int64,
	combos []combo,
	train window,
	res *LeagueResult,
) []candidate {
	crit := e.opts.Criteria
	var passed []candidate

	// Reporta a cada 25 combinações: dá movimento visível no texto sem travar o
	// laço pegando o mutex do tracker milhares de vezes.
	const detalheACada = 25

	for i, c := range combos {
		if ctx.Err() != nil {
			// Shutdown no meio da varredura: NÃO devolve o que já foi minerado.
			// A correção de múltiplos testes só é válida sobre a varredura
			// COMPLETA — corrigir sobre um prefixo do espaço de busca usaria um
			// m menor que o real e afrouxaria o limiar. Ciclo interrompido não
			// publica nada.
			return nil
		}

		if i%detalheACada == 0 {
			e.report.Detail(res.LeagueName + " — " +
				strconv.Itoa(i) + " de " + strconv.Itoa(len(combos)) + " combinações testadas")
		}

		criteria := usecase.FilterCriteria{
			TeamID:           c.teamID,
			LastNGames:       c.window,
			HomeAway:         c.homeAway,
			CornersThreshold: c.line,
			OpponentTier:     c.tier,
			MaxOdds:          c.maxOdds,
			Metric:           c.effectiveMetric(),

			// AUD-001: a descoberta só pode minerar sobre odd de mercado. Odd
			// sintética é derivada da média do próprio lote histórico — filtrar
			// por "odd <= X" sobre ela seleciona lotes de média alta e devolve
			// acerto alto por construção, não por vantagem. Sem odd real, a
			// combinação simplesmente não é testável e não vira estratégia.
			RequireRealOdds: true,

			// AUD-003: só a janela de descoberta. A de validação não existe para
			// este backtest.
			DateFrom: &train.From,
			DateTo:   train.To,
		}

		// maxAgeDays = 0: o recorte temporal aqui é o do split, absoluto. O cap
		// de 90 dias é uma regra do plano gratuito na navegação, não da descoberta.
		result, err := filters.RunBacktest(ctx, leagueID, seasonIDs, criteria, 0)
		if err != nil {
			res.Errors++
			continue
		}

		if reason := crit.validate(result); reason != "" {
			res.Rejections[string(reason)]++
			continue
		}

		// Último filtro do doc 08 (faixa "Descartar" = score < 40). O score é o
		// mesmo que a estratégia receberá ao ser persistida.
		dsfr := strategyengine.PreviewScores(result).DSFRScore
		if dsfr < crit.MinDSFR {
			res.Rejections[string(rejectScore)]++
			continue
		}

		// AUD-003: p-valor unilateral contra a probabilidade que a odd embutia.
		// Sem odd utilizável não há hipótese nula — e sem hipótese nula não se
		// publica. "Não testável" reprova; nunca passa direto.
		p, ok := pValue(result)
		if !ok {
			res.Rejections[string(rejectNotTestable)]++
			continue
		}

		passed = append(passed, candidate{combo: c, result: result, dsfr: dsfr, pValue: p})
	}

	return e.applyFDR(passed, res)
}

// applyFDR corrige o limiar de significância pelo número de testes efetivamente
// realizados e devolve só os candidatos que sobrevivem.
//
// O limiar NÃO é uma constante: quanto mais combinações o ciclo testar, mais
// exigente ele fica. É isso que torna o espaço de busca um custo em vez de uma
// vantagem — hoje ampliar a grade aumenta a chance de achar sorte, e a correção
// é o que cobra por isso.
func (e *Engine) applyFDR(passed []candidate, res *LeagueResult) []candidate {
	res.Tested = len(passed)
	if len(passed) == 0 {
		return nil
	}

	pvalues := make([]float64, len(passed))
	for i, c := range passed {
		pvalues[i] = c.pValue
	}

	threshold, err := formulas.FDRThreshold(pvalues, e.opts.Criteria.FDRq, e.opts.Criteria.FDRMethod)
	if err != nil {
		// Critério inválido não pode virar "publique tudo". Reprova o lote.
		res.Errors++
		res.Rejections[string(rejectMultipleTesting)] += len(passed)
		return nil
	}
	res.FDRThreshold = threshold

	// threshold == 0 significa que nenhum p-valor sobreviveu ao procedimento.
	// A comparação abaixo já cuida disso (nenhum p-valor real é <= 0), mas o
	// caso é explicitado porque tratá-lo como "sem limiar" publicaria tudo.
	var significant []candidate
	for _, c := range passed {
		if threshold > 0 && c.pValue <= threshold {
			significant = append(significant, c)
			continue
		}
		res.Rejections[string(rejectMultipleTesting)]++
	}
	res.Significant = len(significant)
	return significant
}

// validate reexecuta cada candidato na JANELA DE VALIDAÇÃO — dados que a
// mineração nunca leu — e devolve só os que continuam de pé.
//
// É aqui que o ciclo deixa de premiar sorte. Um padrão que existe só porque foi
// procurado entre milhares de combinações não tem motivo para reaparecer num
// período que não participou da busca.
func (e *Engine) validate(
	ctx context.Context,
	filters *usecase.FilterUsecase,
	leagueID int64,
	seasonIDs []int64,
	candidates []candidate,
	holdout window,
	res *LeagueResult,
) []candidate {
	if len(candidates) == 0 {
		return nil
	}
	crit := e.opts.Criteria
	var validated []candidate

	for _, c := range candidates {
		if ctx.Err() != nil {
			return validated
		}

		criteria := usecase.FilterCriteria{
			TeamID:           c.combo.teamID,
			LastNGames:       c.combo.window,
			HomeAway:         c.combo.homeAway,
			CornersThreshold: c.combo.line,
			OpponentTier:     c.combo.tier,
			MaxOdds:          c.combo.maxOdds,
			Metric:           c.combo.effectiveMetric(),
			RequireRealOdds:  true,

			DateFrom: &holdout.From,
			DateTo:   holdout.To,
		}

		out, err := filters.RunBacktest(ctx, leagueID, seasonIDs, criteria, 0)
		if err != nil {
			res.Errors++
			continue
		}

		verdict := checkHoldout(out, crit.HoldoutMinGames, crit.HoldoutAlpha)
		if !verdict.Passed {
			res.Rejections[string(verdict.Reason)]++
			continue
		}

		c.holdout = verdict
		validated = append(validated, c)
	}

	res.Validated = len(validated)
	return validated
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

	// O que foi observado muda conforme o mercado: escanteios é limiar sobre um
	// total, resultado é desfecho da partida. Descrever os dois com a mesma frase
	// produziria texto errado ("o total de escanteios ficou acima de 0.5").
	var observado string
	if c.combo.isResult() {
		observado = fmt.Sprintf(
			"Restrito ao mercado de %s. Em %d ocorrências analisadas, o desfecho se confirmou em %.1f%% delas.",
			strings.ToLower(resultLabel(c.combo.metric)), r.MatchCount, r.HitRate)
	} else {
		observado = fmt.Sprintf(
			"Restrito a jogos com odd registrada até %.2f. Em %d ocorrências analisadas, "+
				"o total de escanteios ficou acima de %d.5 em %.1f%% delas.",
			c.combo.maxOdds, r.MatchCount, c.combo.line, r.HitRate)
	}

	return fmt.Sprintf(
		"Padrão identificado automaticamente pela mineração do histórico do %s, %s (%s). "+
			"%s "+
			"No mesmo período o retorno histórico foi de %.2f%% (yield %.2f%%), com lucro acumulado de %.2f unidades "+
			"e drawdown máximo de %.1f%% do capital movimentado. "+
			"Classificação DSFR: %s (score %.1f). "+
			"%s"+
			"Números apurados sobre dados históricos armazenados — não constituem recomendação de aposta "+
			"nem previsão de resultados futuros.",
		leagueName, scope, filters,
		observado,
		// A descrição só é gerada para candidata aprovada, e aprovação exige
		// série financeira completa (ver checkCriteria) — então os ponteiros
		// nunca são nil aqui.
		derefF(r.ROI), derefF(r.Yield), derefF(r.Profit), drawdownPctOrZero(r),
		Classify(c.dsfr), c.dsfr,
		describeValidation(c),
	)
}

// describeValidation transforma o rastro estatístico do candidato em texto.
//
// Existe porque um número de acerto sem o contexto de "quantas hipóteses foram
// testadas para chegar nele" e "isso se repetiu num período que a busca não
// viu?" comunica mais confiança do que a evidência sustenta. O usuário precisa
// ver as duas coisas junto com o resultado, não escondidas num log de ciclo.
func describeValidation(c candidate) string {
	if !c.holdout.Passed {
		return ""
	}
	return fmt.Sprintf(
		"Verificação fora da amostra: o padrão foi procurado apenas no trecho mais antigo do "+
			"histórico e depois reexecutado no trecho mais recente, que não participou da busca. "+
			"Nesse período reservado foram %d ocorrências, %.1f%% de acerto e retorno de %.2f%%. "+
			"A chance de um resultado assim aparecer por acaso, se a estratégia não tivesse "+
			"vantagem sobre a odd oferecida, é de %.2f%% na busca e %.2f%% na verificação. ",
		c.holdout.Games, c.holdout.HitRate, c.holdout.ROI,
		c.pValue*100, c.holdout.PValue*100,
	)
}

// derefF e drawdownPctOrZero existem só para a formatação do texto de
// candidatas APROVADAS, onde a série financeira já foi exigida por
// checkCriteria. Não são atalho para tratar ausência como zero.
func derefF(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func drawdownPctOrZero(r *usecase.BacktestResult) float64 {
	pct, _ := drawdownPct(r)
	return pct
}
