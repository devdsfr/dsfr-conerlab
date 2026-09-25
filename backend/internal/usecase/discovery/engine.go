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

	// Funnel é o funil auditável do ciclo (REV-P4, B3). Cada combinação gerada
	// termina em exatamente UM estágio, e as identidades de Funnel.Check fecham.
	Funnel Funnel `json:"funnel"`
}

// Funnel conta, estágio a estágio, onde cada combinação gerada parou.
//
// Só contém o que o motor mede. As identidades (ver Check) são:
//
//	Generated         = BacktestErrors + NoRealOdds + NoMetric + InsufficientSample
//	                    + NotTestable + TestedStatistically
//	TestedStatistically = RejectedFDR + FDRSurvivors
//	FDRSurvivors      = RejectedSecondary + HoldoutInput
//	HoldoutInput      = HoldoutErrors + RejectedHoldout + HoldoutValidated + HoldoutInterrupted
//	HoldoutValidated  = CappedPerLeague + PublishErrors + Published
//
// TestedStatistically é o m entregue à correção de múltiplas comparações.
type Funnel struct {
	Generated int `json:"generated"`

	// Antes do teste — só critérios ESTRUTURAIS, que não dependem do resultado.
	BacktestErrors     int `json:"backtest_errors"`
	NoRealOdds         int `json:"rejected_no_real_odds"`
	NoMetric           int `json:"rejected_no_metric"`
	InsufficientSample int `json:"rejected_insufficient_sample"`
	NotTestable        int `json:"rejected_not_testable"`

	// Teste estatístico sobre o conjunto COMPLETO de hipóteses elegíveis.
	TestedStatistically int `json:"tested_statistically"`
	RejectedFDR         int `json:"rejected_fdr"`
	FDRSurvivors        int `json:"fdr_survivors"`

	// Depois do FDR — critérios do doc 08 que dependem do resultado.
	RejectedSecondary int `json:"rejected_secondary"`

	// Fora da amostra.
	HoldoutInput       int `json:"holdout_input"`
	HoldoutErrors      int `json:"holdout_errors"`
	RejectedHoldout    int `json:"rejected_holdout"`
	HoldoutInterrupted int `json:"holdout_interrupted"`
	HoldoutValidated   int `json:"holdout_validated"`

	// Publicação.
	CappedPerLeague int `json:"capped_per_league"`
	PublishErrors   int `json:"publish_errors"`
	Published       int `json:"published"`

	// Interrupted = o ciclo foi cortado (shutdown/timeout) durante a varredura.
	// Nesse caso nada é publicado e as identidades não se aplicam.
	Interrupted bool `json:"interrupted,omitempty"`
}

// Check verifica as identidades do funil. Devolve "" se todas fecham, ou a
// primeira que não fecha. Usado por testes e antes de gravar o ciclo.
func (f Funnel) Check() string {
	if f.Interrupted {
		return ""
	}
	pre := f.BacktestErrors + f.NoRealOdds + f.NoMetric + f.InsufficientSample +
		f.NotTestable + f.TestedStatistically
	switch {
	case f.Generated != pre:
		return fmt.Sprintf("geradas %d ≠ estruturais+testadas %d", f.Generated, pre)
	case f.TestedStatistically != f.RejectedFDR+f.FDRSurvivors:
		return fmt.Sprintf("testadas %d ≠ FDR rejeitadas %d + sobreviventes %d",
			f.TestedStatistically, f.RejectedFDR, f.FDRSurvivors)
	case f.FDRSurvivors != f.RejectedSecondary+f.HoldoutInput:
		return fmt.Sprintf("sobreviventes %d ≠ secundários %d + holdout %d",
			f.FDRSurvivors, f.RejectedSecondary, f.HoldoutInput)
	case f.HoldoutInput != f.HoldoutErrors+f.RejectedHoldout+f.HoldoutValidated+f.HoldoutInterrupted:
		return fmt.Sprintf("holdout %d ≠ erros %d + reprovadas %d + validadas %d + interrompidas %d",
			f.HoldoutInput, f.HoldoutErrors, f.RejectedHoldout, f.HoldoutValidated, f.HoldoutInterrupted)
	case f.HoldoutValidated != f.CappedPerLeague+f.PublishErrors+f.Published:
		return fmt.Sprintf("validadas %d ≠ teto %d + erros %d + publicadas %d",
			f.HoldoutValidated, f.CappedPerLeague, f.PublishErrors, f.Published)
	}
	return ""
}

// Result resume um ciclo completo (todas as ligas).
type Result struct {
	Leagues      int            `json:"leagues"`
	Combinations int            `json:"combinations"`
	Published    int            `json:"published"`
	Deactivated  int            `json:"deactivated"`
	Errors       int            `json:"errors"`
	ByLeague     []LeagueResult `json:"by_league"`

	// REV-P4 (B3): funil e motivos AGREGADOS. A tela lia `rejections` no topo
	// do resultado, campo que não existia aqui — a lista de motivos de descarte
	// nunca foi renderizada em nenhuma varredura.
	Rejections map[string]int `json:"rejections"`
	Funnel     Funnel         `json:"funnel"`
}

// FromLeague monta o Result de um ciclo de uma única liga (disparo manual com
// liga escolhida), com o funil e os motivos no topo.
func FromLeague(lr LeagueResult) Result {
	r := Result{Leagues: 1, Rejections: map[string]int{}}
	r.absorb(lr)
	return r
}

func (r *Result) absorb(lr LeagueResult) {
	if r.Rejections == nil {
		r.Rejections = map[string]int{}
	}
	r.Combinations += lr.Combinations
	r.Published += lr.Published
	r.Deactivated += lr.Deactivated
	r.Errors += lr.Errors
	r.ByLeague = append(r.ByLeague, lr)
	for k, v := range lr.Rejections {
		r.Rejections[k] += v
	}
	r.Funnel.add(lr.Funnel)
}

// add soma o funil de uma liga ao agregado. As identidades de Check continuam
// valendo na soma, porque valem em cada parcela.
func (f *Funnel) add(o Funnel) {
	f.Generated += o.Generated
	f.BacktestErrors += o.BacktestErrors
	f.NoRealOdds += o.NoRealOdds
	f.NoMetric += o.NoMetric
	f.InsufficientSample += o.InsufficientSample
	f.NotTestable += o.NotTestable
	f.TestedStatistically += o.TestedStatistically
	f.RejectedFDR += o.RejectedFDR
	f.FDRSurvivors += o.FDRSurvivors
	f.RejectedSecondary += o.RejectedSecondary
	f.HoldoutInput += o.HoldoutInput
	f.HoldoutErrors += o.HoldoutErrors
	f.RejectedHoldout += o.RejectedHoldout
	f.HoldoutInterrupted += o.HoldoutInterrupted
	f.HoldoutValidated += o.HoldoutValidated
	f.CappedPerLeague += o.CappedPerLeague
	f.PublishErrors += o.PublishErrors
	f.Published += o.Published
	f.Interrupted = f.Interrupted || o.Interrupted
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

	out.Rejections = map[string]int{}
	for _, l := range leagues {
		e.report.Step(l.Name)
		lr, err := e.RunLeague(ctx, l.ID, nil)
		if err != nil {
			out.Errors++
			// Um ciclo interrompido também entra no agregado, marcado, para o
			// registro dizer que houve interrupção em vez de sumir com a liga.
			if lr.Funnel.Interrupted {
				out.Funnel.Interrupted = true
			}
			continue
		}
		out.Leagues++
		out.absorb(lr)
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
	res.Funnel.Generated = len(combos)

	approved := e.mine(ctx, filters, leagueID, seasonIDs, combos, train, &res)
	if res.Funnel.Interrupted {
		// Varredura incompleta: o m do FDR seria menor que o real. Não publica e,
		// principalmente, não desativa as descobertas vigentes com base num ciclo
		// que não terminou.
		return res, ctx.Err()
	}
	approved = e.validate(ctx, filters, leagueID, seasonIDs, approved, holdout, &res)
	if res.Funnel.HoldoutInterrupted > 0 {
		// Mesmo motivo: publicar só parte dos validados e desativar o resto
		// retiraria do ar descobertas que nem chegaram a ser checadas.
		res.Funnel.Interrupted = true
		return res, ctx.Err()
	}
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

// mine executa o backtest de cada combinação NA JANELA DE DESCOBERTA, aplica a
// correção para testes múltiplos (AUD-003) e só DEPOIS os critérios do doc 08.
//
// REV-P4 (B1) — a ordem é o conteúdo da correção:
//
//  1. ELEGIBILIDADE ESTRUTURAL — só o que não depende do resultado: erro de
//     backtest, falta de odd real, métrica não publicada, amostra abaixo do
//     mínimo, p-valor não calculável.
//  2. p-valor de TODAS as hipóteses elegíveis; m = quantas são.
//  3. BH/BY sobre esse conjunto completo.
//  4. SÓ ENTÃO os critérios do doc 08 que dependem do resultado (win rate, ROI,
//     yield, lucro, drawdown, DSFR), aplicados aos sobreviventes.
//
// Antes os passos 4 vinham antes do 2: o FDR recebia só as combinações que já
// "pareciam boas". Medido sobre ruído puro com os critérios de produção (200
// seeds × 700 partidas): m médio de 0,75 contra ~70 elegíveis, e falsos
// "significativos" em 8,5 % das simulações contra ~1 % no procedimento correto.
// O holdout segurava a publicação; a camada de FDR não cumpria o que prometia.
//
// Erros de uma combinação isolada são contados e o ciclo segue. Um ciclo
// interrompido NÃO publica nada: a correção só é válida sobre a varredura
// completa — corrigir sobre um prefixo usaria um m menor que o real.
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
	f := &res.Funnel
	var hipoteses []candidate

	// Reporta a cada 25 combinações: dá movimento visível no texto sem travar o
	// laço pegando o mutex do tracker milhares de vezes.
	const detalheACada = 25

	for i, c := range combos {
		if ctx.Err() != nil {
			f.Interrupted = true
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
			MaxOdds:          c.maxOdds,
			Metric:           c.effectiveMetric(),

			// AUD-001: a descoberta só pode minerar sobre odd de mercado. Odd
			// sintética é derivada da média do próprio lote histórico — filtrar
			// por "odd <= X" sobre ela seleciona lotes de média alta e devolve
			// acerto alto por construção, não por vantagem. Sem odd real, a
			// combinação simplesmente não é testável e não vira estratégia.
			// Não há odd fixa aqui: odd digitada é cenário, não mercado.
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
			f.BacktestErrors++
			continue
		}

		// 1. Elegibilidade estrutural.
		if reason := crit.structuralSample(result); reason != "" {
			res.Rejections[string(reason)]++
			switch reason {
			case rejectNoRealOdds:
				f.NoRealOdds++
			case rejectNoMetric:
				f.NoMetric++
			default:
				f.InsufficientSample++
			}
			continue
		}
		// AUD-003: p-valor unilateral contra a probabilidade que a odd embutia.
		// Sem odd utilizável não há hipótese nula — e sem hipótese nula não se
		// publica. "Não testável" reprova; nunca passa direto.
		p, ok := pValue(result)
		if !ok {
			res.Rejections[string(rejectNotTestable)]++
			f.NotTestable++
			continue
		}

		hipoteses = append(hipoteses, candidate{combo: c, result: result, pValue: p})
	}

	// 2–3. Correção sobre o conjunto completo.
	sobreviventes := e.applyFDR(hipoteses, res)

	// 4. Critérios do doc 08 que dependem do resultado — só agora.
	var aprovadas []candidate
	for _, c := range sobreviventes {
		if reason := crit.secondary(c.result); reason != "" {
			res.Rejections[string(reason)]++
			f.RejectedSecondary++
			continue
		}
		// Último filtro do doc 08 (faixa "Descartar" = score < 40). O score é o
		// mesmo que a estratégia receberá ao ser persistida.
		c.dsfr = strategyengine.PreviewScores(c.result).DSFRScore
		if c.dsfr < crit.MinDSFR {
			res.Rejections[string(rejectScore)]++
			f.RejectedSecondary++
			continue
		}
		aprovadas = append(aprovadas, c)
	}
	return aprovadas
}

// applyFDR corrige o limiar de significância pelo número de hipóteses
// ELEGÍVEIS — todas as que passaram só nos critérios estruturais — e devolve as
// que sobrevivem.
//
// O limiar NÃO é uma constante: quanto mais hipóteses o ciclo testar, mais
// exigente ele fica. É isso que torna o espaço de busca um custo em vez de uma
// vantagem.
func (e *Engine) applyFDR(hipoteses []candidate, res *LeagueResult) []candidate {
	f := &res.Funnel
	res.Tested = len(hipoteses)
	f.TestedStatistically = len(hipoteses)
	if len(hipoteses) == 0 {
		return nil
	}

	pvalues := make([]float64, len(hipoteses))
	for i, c := range hipoteses {
		pvalues[i] = c.pValue
	}

	threshold, err := formulas.FDRThreshold(pvalues, e.opts.Criteria.FDRq, e.opts.Criteria.FDRMethod)
	if err != nil {
		// Critério inválido não pode virar "publique tudo". Reprova o lote.
		res.Errors++
		res.Rejections[string(rejectMultipleTesting)] += len(hipoteses)
		f.RejectedFDR += len(hipoteses)
		return nil
	}
	res.FDRThreshold = threshold

	// threshold == 0 significa que nenhum p-valor sobreviveu ao procedimento.
	// Tratá-lo como "sem limiar" publicaria tudo.
	var significant []candidate
	for _, c := range hipoteses {
		if threshold > 0 && c.pValue <= threshold {
			significant = append(significant, c)
			continue
		}
		res.Rejections[string(rejectMultipleTesting)]++
		f.RejectedFDR++
	}
	res.Significant = len(significant)
	f.FDRSurvivors = len(significant)
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
	f := &res.Funnel
	f.HoldoutInput = len(candidates)
	if len(candidates) == 0 {
		return nil
	}
	crit := e.opts.Criteria
	var validated []candidate

	for i, c := range candidates {
		if ctx.Err() != nil {
			// As que não chegaram a ser checadas ficam registradas como
			// interrompidas, não como reprovadas.
			f.HoldoutInterrupted = len(candidates) - i
			f.HoldoutValidated = len(validated)
			res.Validated = len(validated)
			return validated
		}

		criteria := usecase.FilterCriteria{
			TeamID:           c.combo.teamID,
			LastNGames:       c.combo.window,
			HomeAway:         c.combo.homeAway,
			CornersThreshold: c.combo.line,
			MaxOdds:          c.combo.maxOdds,
			Metric:           c.combo.effectiveMetric(),
			RequireRealOdds:  true,

			DateFrom: &holdout.From,
			DateTo:   holdout.To,
		}

		out, err := filters.RunBacktest(ctx, leagueID, seasonIDs, criteria, 0)
		if err != nil {
			res.Errors++
			f.HoldoutErrors++
			continue
		}

		verdict := checkHoldout(out, crit.HoldoutMinGames, crit.HoldoutAlpha)
		if !verdict.Passed {
			res.Rejections[string(verdict.Reason)]++
			f.RejectedHoldout++
			continue
		}

		c.holdout = verdict
		validated = append(validated, c)
	}

	res.Validated = len(validated)
	f.HoldoutValidated = len(validated)
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
		res.Funnel.CappedPerLeague = len(approved) - e.opts.Criteria.MaxPerLeague
		approved = approved[:e.opts.Criteria.MaxPerLeague]
	}

	published := make([]int64, 0, len(approved))
	for _, c := range approved {
		definition, err := c.combo.definition(leagueID, seasonIDs)
		if err != nil {
			res.Errors++
			res.Funnel.PublishErrors++
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
			res.Funnel.PublishErrors++
			continue
		}
		if _, err := e.persister.PersistResult(ctx, s.ID, c.result); err != nil {
			res.Errors++
			res.Funnel.PublishErrors++
			continue
		}
		published = append(published, s.ID)
	}
	res.Funnel.Published = len(published)
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
