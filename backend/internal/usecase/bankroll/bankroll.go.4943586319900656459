// Package bankroll implementa o Módulo de Gestão Evolutiva de Banca: permite ao
// usuário configurar uma sequência de fases de banca e critérios objetivos de
// evolução, calcula automaticamente os indicadores (a partir das apostas reais já
// registradas no Módulo de Apostas) e só libera a promoção de fase quando TODOS os
// critérios configurados são atendidos — nunca por tempo decorrido isoladamente, e
// nunca de forma automática (a confirmação é sempre manual).
package bankroll

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

type Usecase struct {
	repo repository.BankrollRepository
	bets repository.BetRepository
}

func New(repo repository.BankrollRepository, bets repository.BetRepository) *Usecase {
	return &Usecase{repo: repo, bets: bets}
}

// defaultPhases é a "Estratégia Inicial" sugerida no critério de aceite do módulo —
// totalmente editável pelo usuário depois via SetPhases.
var defaultPhases = []struct {
	Name   string
	Amount float64
}{
	{"Fase 1", 150}, {"Fase 2", 300}, {"Fase 3", 500}, {"Fase 4", 750}, {"Fase 5", 1000},
	{"Fase 6", 1500}, {"Fase 7", 2000}, {"Fase 8", 3000}, {"Fase 9", 5000},
}

// Phases retorna as fases configuradas do usuário, criando a sequência padrão na
// primeira vez que o módulo é acessado.
func (u *Usecase) Phases(ctx context.Context, userID int64) ([]domain.BankrollPhase, error) {
	phases, err := u.repo.ListPhases(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(phases) > 0 {
		return phases, nil
	}
	seed := make([]domain.BankrollPhase, len(defaultPhases))
	for i, p := range defaultPhases {
		seed[i] = domain.BankrollPhase{UserID: userID, Sequence: i + 1, Name: p.Name, Amount: p.Amount}
	}
	return u.repo.ReplacePhases(ctx, userID, seed)
}

// SetPhases substitui a sequência de fases configurada pelo usuário. Exige ao menos
// uma fase, sequências positivas e sem repetição, e valores de banca positivos. Se a
// fase atual do usuário deixar de existir na nova sequência, reposiciona o estado na
// fase disponível mais próxima (nunca promove sozinho ao reconfigurar).
func (u *Usecase) SetPhases(ctx context.Context, userID int64, phases []domain.BankrollPhase) ([]domain.BankrollPhase, error) {
	if len(phases) == 0 {
		return nil, errors.New("informe ao menos uma fase")
	}
	seen := map[int]bool{}
	for _, p := range phases {
		if p.Sequence <= 0 {
			return nil, errors.New("a ordem da fase deve ser um número positivo")
		}
		if seen[p.Sequence] {
			return nil, fmt.Errorf("ordem de fase duplicada: %d", p.Sequence)
		}
		seen[p.Sequence] = true
		if p.Amount <= 0 {
			return nil, errors.New("o valor da banca de cada fase deve ser positivo")
		}
	}

	updated, err := u.repo.ReplacePhases(ctx, userID, phases)
	if err != nil {
		return nil, err
	}

	state, err := u.ensureState(ctx, userID)
	if err == nil && !hasSequence(updated, state.CurrentPhaseSequence) {
		_, _ = u.repo.SetPhase(ctx, userID, nearestSequence(updated, state.CurrentPhaseSequence))
	}
	return updated, nil
}

// Criteria retorna os critérios de evolução configurados (ou os padrões, na primeira
// vez que o módulo é acessado).
func (u *Usecase) Criteria(ctx context.Context, userID int64) (domain.BankrollCriteria, error) {
	return u.repo.GetCriteria(ctx, userID)
}

func (u *Usecase) SetCriteria(ctx context.Context, c domain.BankrollCriteria) error {
	if c.MinDays < 0 || c.MinBets < 0 || c.MinCompletedCycles < 0 {
		return errors.New("critérios mínimos não podem ser negativos")
	}
	if c.CycleWinStreak <= 0 {
		c.CycleWinStreak = 3
	}
	return u.repo.SaveCriteria(ctx, c)
}

func (u *Usecase) ensureState(ctx context.Context, userID int64) (*domain.BankrollState, error) {
	state, err := u.repo.GetState(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state != nil {
		return state, nil
	}
	return u.repo.InitState(ctx, userID)
}

func hasSequence(phases []domain.BankrollPhase, seq int) bool {
	for _, p := range phases {
		if p.Sequence == seq {
			return true
		}
	}
	return false
}

// nearestSequence encontra, entre as fases disponíveis, a maior sequência que ainda
// seja <= seq (ou a menor sequência disponível, se nenhuma for menor ou igual).
func nearestSequence(phases []domain.BankrollPhase, seq int) int {
	best := phases[0].Sequence
	for _, p := range phases {
		if p.Sequence < best {
			best = p.Sequence
		}
	}
	for _, p := range phases {
		if p.Sequence <= seq && p.Sequence > best {
			best = p.Sequence
		}
	}
	return best
}

func findPhase(phases []domain.BankrollPhase, seq int) (domain.BankrollPhase, bool) {
	for _, p := range phases {
		if p.Sequence == seq {
			return p, true
		}
	}
	return domain.BankrollPhase{}, false
}

func nextPhase(phases []domain.BankrollPhase, currentSeq int) *domain.BankrollPhase {
	var best *domain.BankrollPhase
	for i := range phases {
		if phases[i].Sequence > currentSeq && (best == nil || phases[i].Sequence < best.Sequence) {
			best = &phases[i]
		}
	}
	return best
}

func previousPhase(phases []domain.BankrollPhase, currentSeq int) *domain.BankrollPhase {
	var best *domain.BankrollPhase
	for i := range phases {
		if phases[i].Sequence < currentSeq && (best == nil || phases[i].Sequence > best.Sequence) {
			best = &phases[i]
		}
	}
	return best
}

// Metrics são os indicadores objetivos calculados a partir das apostas reais do
// usuário (Módulo de Apostas), consideradas desde o início da fase atual.
type Metrics struct {
	SampleSize         int     `json:"sample_size"`
	WinRate            float64 `json:"win_rate"`
	ROI                float64 `json:"roi"`
	Yield              float64 `json:"yield"`
	NetProfit          float64 `json:"net_profit"`
	CompletedCycles    int     `json:"completed_cycles"`
	DaysInPhase        int     `json:"days_in_phase"`
	MaxDrawdown        float64 `json:"max_drawdown"`         // em unidades de stake
	MaxDrawdownPct     float64 `json:"max_drawdown_pct"`     // % do total apostado na fase
	MonthlyConsistency float64 `json:"monthly_consistency"`  // 0-1: fração de meses com lucro positivo
}

type ChecklistItem struct {
	Label    string `json:"label"`
	Met      bool   `json:"met"`
	Current  string `json:"current"`
	Required string `json:"required"`
}

type MaturityScore struct {
	Score  float64 `json:"score"` // 0-100
	Stars  int     `json:"stars"` // 0-5
	Status string  `json:"status"`
}

type Status struct {
	CurrentPhase      domain.BankrollPhase    `json:"current_phase"`
	NextPhase         *domain.BankrollPhase   `json:"next_phase"`
	PreviousPhase     *domain.BankrollPhase   `json:"previous_phase"`
	Metrics           Metrics                 `json:"metrics"`
	Criteria          domain.BankrollCriteria `json:"criteria"`
	Checklist         []ChecklistItem         `json:"checklist"`
	ReadyToPromote    bool                    `json:"ready_to_promote"`
	BlockedReasons    []string                `json:"blocked_reasons"`
	Maturity          MaturityScore           `json:"maturity"`
	Progress          float64                 `json:"progress"` // 0-1
	State             domain.BankrollState    `json:"state"`
	DemotionSuggested bool                    `json:"demotion_suggested"`
	DemotionReason    string                  `json:"demotion_reason"`
}

// Status monta o dashboard de evolução: fase atual/próxima, métricas calculadas a
// partir das apostas reais da fase atual, checklist de critérios, Score de Maturidade
// da Estratégia (SME) e se a evolução está liberada para confirmação manual.
func (u *Usecase) Status(ctx context.Context, userID int64) (*Status, error) {
	phases, err := u.Phases(ctx, userID)
	if err != nil {
		return nil, err
	}
	state, err := u.ensureState(ctx, userID)
	if err != nil {
		return nil, err
	}
	criteria, err := u.repo.GetCriteria(ctx, userID)
	if err != nil {
		return nil, err
	}

	current, ok := findPhase(phases, state.CurrentPhaseSequence)
	if !ok {
		return nil, fmt.Errorf("fase atual (sequência %d) não encontrada na configuração de fases", state.CurrentPhaseSequence)
	}
	next := nextPhase(phases, state.CurrentPhaseSequence)
	prev := previousPhase(phases, state.CurrentPhaseSequence)

	bets, err := u.bets.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	phaseBets := betsSince(bets, state.PhaseStartedAt)
	metrics := computeMetrics(phaseBets, state.PhaseStartedAt, criteria.CycleWinStreak)

	checklist, blocked, ready := evaluateCriteria(metrics, criteria)
	if next == nil {
		ready = false
		blocked = append(blocked, "esta já é a última fase configurada")
	}
	maturity := computeMaturity(metrics)
	progress := checklistProgress(checklist)
	demotionSuggested, demotionReason := suggestDemotion(metrics)

	return &Status{
		CurrentPhase: current, NextPhase: next, PreviousPhase: prev,
		Metrics: metrics, Criteria: criteria, Checklist: checklist,
		ReadyToPromote: ready, BlockedReasons: blocked, Maturity: maturity,
		Progress: progress, State: *state,
		DemotionSuggested: demotionSuggested, DemotionReason: demotionReason,
	}, nil
}

func betsSince(bets []domain.Bet, since time.Time) []domain.Bet {
	out := make([]domain.Bet, 0, len(bets))
	for _, b := range bets {
		if (b.Status == domain.BetStatusWon || b.Status == domain.BetStatusLost) && !b.EventDate.Before(since) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EventDate.Before(out[j].EventDate) })
	return out
}

func computeMetrics(bets []domain.Bet, phaseStart time.Time, cycleWinStreak int) Metrics {
	m := Metrics{DaysInPhase: int(time.Since(phaseStart).Hours() / 24)}
	if len(bets) == 0 {
		return m
	}

	wins := 0
	totalStaked := 0.0
	netProfit := 0.0
	cumulative, peak, maxDD := 0.0, 0.0, 0.0
	monthly := map[string]float64{}

	for _, b := range bets {
		if b.Status == domain.BetStatusWon {
			wins++
		}
		totalStaked += b.Stake
		netProfit += b.ProfitLoss
		cumulative += b.ProfitLoss
		if cumulative > peak {
			peak = cumulative
		}
		if dd := peak - cumulative; dd > maxDD {
			maxDD = dd
		}
		monthly[b.EventDate.Format("2006-01")] += b.ProfitLoss
	}

	m.SampleSize = len(bets)
	m.WinRate = round2(100 * float64(wins) / float64(len(bets)))
	if totalStaked > 0 {
		m.ROI = round2(100 * netProfit / totalStaked)
		m.Yield = m.ROI
		m.MaxDrawdownPct = round2(100 * maxDD / totalStaked)
	}
	m.NetProfit = round2(netProfit)
	m.MaxDrawdown = round2(maxDD)
	m.CompletedCycles = countCompletedCycles(bets, cycleWinStreak)

	if len(monthly) > 0 {
		positive := 0
		for _, profit := range monthly {
			if profit > 0 {
				positive++
			}
		}
		m.MonthlyConsistency = round2(float64(positive) / float64(len(monthly)))
	}
	return m
}

// countCompletedCycles conta quantas sequências não sobrepostas de N vitórias
// consecutivas (cycleWinStreak) ocorreram, na ordem cronológica das apostas — usado
// como proxy objetivo de "ciclo completo" sem exigir cadastro manual de ciclos.
func countCompletedCycles(bets []domain.Bet, cycleWinStreak int) int {
	if cycleWinStreak <= 0 {
		return 0
	}
	cycles, streak := 0, 0
	for _, b := range bets {
		if b.Status == domain.BetStatusWon {
			streak++
			if streak == cycleWinStreak {
				cycles++
				streak = 0
			}
		} else {
			streak = 0
		}
	}
	return cycles
}

func evaluateCriteria(m Metrics, c domain.BankrollCriteria) ([]ChecklistItem, []string, bool) {
	items := []ChecklistItem{
		{
			Label: fmt.Sprintf("Mínimo de %d dias na fase atual", c.MinDays), Met: m.DaysInPhase >= c.MinDays,
			Current: fmt.Sprintf("%d dias", m.DaysInPhase), Required: fmt.Sprintf("%d dias", c.MinDays),
		},
		{
			Label: fmt.Sprintf("Mínimo de %d apostas", c.MinBets), Met: m.SampleSize >= c.MinBets,
			Current: fmt.Sprintf("%d apostas", m.SampleSize), Required: fmt.Sprintf("%d apostas", c.MinBets),
		},
		{
			Label: fmt.Sprintf("Win Rate mínimo de %.0f%%", c.MinWinRate), Met: m.WinRate >= c.MinWinRate,
			Current: fmt.Sprintf("%.1f%%", m.WinRate), Required: fmt.Sprintf("%.0f%%", c.MinWinRate),
		},
		{
			Label: fmt.Sprintf("ROI mínimo de %.0f%%", c.MinROI), Met: m.ROI >= c.MinROI,
			Current: fmt.Sprintf("%.1f%%", m.ROI), Required: fmt.Sprintf("%.0f%%", c.MinROI),
		},
		{
			Label: fmt.Sprintf("Yield mínimo de %.0f%%", c.MinYield), Met: m.Yield >= c.MinYield,
			Current: fmt.Sprintf("%.1f%%", m.Yield), Required: fmt.Sprintf("%.0f%%", c.MinYield),
		},
		{
			Label: fmt.Sprintf("Mínimo de %d ciclos completos", c.MinCompletedCycles), Met: m.CompletedCycles >= c.MinCompletedCycles,
			Current: fmt.Sprintf("%d ciclos", m.CompletedCycles), Required: fmt.Sprintf("%d ciclos", c.MinCompletedCycles),
		},
	}
	if c.RequirePositiveProfit {
		items = append(items, ChecklistItem{
			Label: "Lucro líquido positivo", Met: m.NetProfit > 0,
			Current: fmt.Sprintf("%.2f", m.NetProfit), Required: "acima de 0",
		})
	}

	var blocked []string
	ready := true
	for _, it := range items {
		if !it.Met {
			ready = false
			blocked = append(blocked, it.Label)
		}
	}
	return items, blocked, ready
}

func checklistProgress(items []ChecklistItem) float64 {
	if len(items) == 0 {
		return 0
	}
	met := 0
	for _, it := range items {
		if it.Met {
			met++
		}
	}
	return round2(float64(met) / float64(len(items)))
}

// computeMaturity calcula o Score de Maturidade da Estratégia (SME, 0-100): Win Rate
// (25%), ROI (20%), Yield (15%), Drawdown (15%), tamanho da amostra (15%) e
// consistência mensal (10%). As normalizações usadas (ex: ROI de 20% já atinge a nota
// máxima do quesito; 200 apostas atingem a nota máxima de amostra) são uma escolha de
// produto — ajuste os divisores conforme o histórico real da plataforma crescer.
func computeMaturity(m Metrics) MaturityScore {
	winRateScore := clamp(m.WinRate, 0, 100)
	roiScore := clamp(m.ROI*5, 0, 100)
	yieldScore := clamp(m.Yield*5, 0, 100)
	drawdownScore := clamp(100-m.MaxDrawdownPct*3, 0, 100)
	sampleScore := clamp(float64(m.SampleSize)/2, 0, 100)
	consistencyScore := clamp(m.MonthlyConsistency*100, 0, 100)

	score := round2(winRateScore*0.25 + roiScore*0.20 + yieldScore*0.15 + drawdownScore*0.15 + sampleScore*0.15 + consistencyScore*0.10)
	stars := int(math.Round(score / 20))

	status := "Estratégia ainda não demonstrou maturidade suficiente."
	switch {
	case score >= 85:
		status = "Estratégia madura. Risco controlado. Apta para evolução da banca."
	case score >= 60:
		status = "Estratégia em consolidação — acompanhe mais alguns ciclos antes de evoluir."
	}
	return MaturityScore{Score: score, Stars: stars, Status: status}
}

// suggestDemotion sinaliza (sem nunca aplicar automaticamente) quando os indicadores
// da fase atual sugerem que a banca deveria retornar à fase anterior.
func suggestDemotion(m Metrics) (bool, string) {
	switch {
	case m.SampleSize >= 20 && m.ROI < 0 && m.DaysInPhase >= 60:
		return true, "ROI negativo nos últimos 60 dias"
	case m.SampleSize >= 20 && m.WinRate < 70:
		return true, "Win Rate abaixo de 70%"
	case m.MaxDrawdownPct > 25:
		return true, "Drawdown acima de 25% do total apostado na fase"
	default:
		return false, ""
	}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Promote confirma manualmente a evolução para a próxima fase disponível. Só é
// permitido quando todos os critérios configurados foram atendidos — a decisão nunca
// é tomada automaticamente pelo sistema.
func (u *Usecase) Promote(ctx context.Context, userID int64, notes string) (*domain.BankrollHistoryEntry, error) {
	status, err := u.Status(ctx, userID)
	if err != nil {
		return nil, err
	}
	if status.NextPhase == nil {
		return nil, errors.New("esta já é a última fase configurada")
	}
	if !status.ReadyToPromote {
		return nil, fmt.Errorf("critérios de evolução ainda não atendidos: %s", strings.Join(status.BlockedReasons, "; "))
	}

	if _, err := u.repo.SetPhase(ctx, userID, status.NextPhase.Sequence); err != nil {
		return nil, err
	}
	entry := &domain.BankrollHistoryEntry{
		UserID: userID, FromAmount: status.CurrentPhase.Amount, ToAmount: status.NextPhase.Amount,
		Direction: "promotion", Reason: "critérios de evolução atendidos", Notes: notes,
	}
	if err := u.repo.AddHistory(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Demote reduz manualmente a banca para a fase anterior — usado quando a estratégia
// demonstra deterioração (ver Status.DemotionSuggested) ou por decisão do próprio
// usuário. Também nunca é automático.
func (u *Usecase) Demote(ctx context.Context, userID int64, reason, notes string) (*domain.BankrollHistoryEntry, error) {
	phases, err := u.Phases(ctx, userID)
	if err != nil {
		return nil, err
	}
	state, err := u.ensureState(ctx, userID)
	if err != nil {
		return nil, err
	}
	current, ok := findPhase(phases, state.CurrentPhaseSequence)
	if !ok {
		return nil, fmt.Errorf("fase atual (sequência %d) não encontrada na configuração de fases", state.CurrentPhaseSequence)
	}
	prev := previousPhase(phases, state.CurrentPhaseSequence)
	if prev == nil {
		return nil, errors.New("esta já é a primeira fase configurada")
	}
	if reason == "" {
		reason = "rebaixamento manual solicitado pelo usuário"
	}

	if _, err := u.repo.SetPhase(ctx, userID, prev.Sequence); err != nil {
		return nil, err
	}
	entry := &domain.BankrollHistoryEntry{
		UserID: userID, FromAmount: current.Amount, ToAmount: prev.Amount,
		Direction: "demotion", Reason: reason, Notes: notes,
	}
	if err := u.repo.AddHistory(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (u *Usecase) History(ctx context.Context, userID int64) ([]domain.BankrollHistoryEntry, error) {
	return u.repo.ListHistory(ctx, userID)
}

// ConfirmRound registra manualmente que uma rodada (fase) foi executada na vida
// real, com o resultado (lucro/prejuízo) obtido. O saldo acumulado é sempre
// calculado a partir do saldo real anterior (última rodada confirmada, ou a banca
// da primeira fase configurada, se ainda não há nenhuma rodada) — nunca do valor
// fixo pré-definido da próxima fase — para refletir a banca de verdade e servir de
// prova histórica de que a estratégia está funcionando.
func (u *Usecase) ConfirmRound(ctx context.Context, userID int64, phaseSequence int, result float64, notes string) (*domain.BankrollRound, error) {
	phases, err := u.Phases(ctx, userID)
	if err != nil {
		return nil, err
	}
	phase, ok := findPhase(phases, phaseSequence)
	if !ok {
		return nil, fmt.Errorf("fase (sequência %d) não encontrada na configuração de fases", phaseSequence)
	}

	rounds, err := u.repo.ListRounds(ctx, userID)
	if err != nil {
		return nil, err
	}
	startBalance := lowestPhaseAmount(phases)
	if len(rounds) > 0 {
		startBalance = rounds[len(rounds)-1].BalanceAfter
	}

	entry := &domain.BankrollRound{
		UserID: userID, PhaseSequence: phase.Sequence, PhaseName: phase.Name,
		Result: round2(result), BalanceAfter: round2(startBalance + result), Notes: notes,
	}
	if err := u.repo.AddRound(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Rounds retorna o registro completo de rodadas confirmadas, em ordem cronológica
// — a prova histórica de consolidação da estratégia a longo prazo.
func (u *Usecase) Rounds(ctx context.Context, userID int64) ([]domain.BankrollRound, error) {
	return u.repo.ListRounds(ctx, userID)
}

func lowestPhaseAmount(phases []domain.BankrollPhase) float64 {
	best := phases[0]
	for _, p := range phases {
		if p.Sequence < best.Sequence {
			best = p
		}
	}
	return best.Amount
}
