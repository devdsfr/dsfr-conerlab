package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// FilterCriteria representa uma regra montada pelo usuário no Simulador de Filtros
// (Módulo 3). Todos os campos são opcionais exceto CornersThreshold.
//
// Semântica de LastNGames: quando informado, cada equipe é avaliada apenas nos seus
// N jogos mais recentes dentro do período selecionado (temporadas), antes da
// aplicação dos demais critérios — reproduzindo o conceito de "amostra recente" usado
// no Dashboard, mas aplicado historicamente para fins de backtest.
type FilterCriteria struct {
	TeamID           *int64  `json:"team_id,omitempty"`
	LastNGames       int     `json:"last_n_games,omitempty"`
	HomeAway         string  `json:"home_away,omitempty"`
	CornersThreshold int     `json:"corners_threshold"`
	OpponentTier     string  `json:"opponent_tier,omitempty"`
	MaxOdds          float64 `json:"max_odds,omitempty"`
	Stake            float64 `json:"stake,omitempty"`

	// Metric escolhe o que backtestar: "corners" (padrão), "goals", "offsides", "shots"
	// (chutes) ou "shots_on_target" (chutes no gol). Cada uma tem seu threshold (linha
	// over/under: 0 = "acima de 0.5", 2 = "acima de 2.5"; escanteios/chutes usam faixas
	// inteiras maiores). Só escanteios têm odds históricas (corner_odds); as demais
	// simulam o retorno com FixedOdd — odd única informada pelo usuário; sem ela é só
	// análise de acerto (sem ROI significativo). Impedimentos/chutes/chutes no gol são
	// nullable no provedor: jogos sem o dado são ignorados no backtest.
	Metric                 string  `json:"metric,omitempty"`
	GoalsThreshold         int     `json:"goals_threshold,omitempty"`
	OffsidesThreshold      int     `json:"offsides_threshold,omitempty"`
	ShotsThreshold         int     `json:"shots_threshold,omitempty"`
	ShotsOnTargetThreshold int     `json:"shots_on_target_threshold,omitempty"`
	FixedOdd               float64 `json:"fixed_odd,omitempty"`

	// RequireRealOdds descarta qualquer partida cuja odd não seja de mercado
	// (AUD-001). Não é ajustável pelo usuário — é ligado internamente pelo
	// Discovery Engine, que só pode validar estratégia sobre odd real. O
	// Simulador roda com false e sinaliza a procedência no resultado.
	RequireRealOdds bool `json:"-"`

	// DateFrom e DateTo restringem o backtest a uma janela temporal fechada à
	// esquerda e ABERTA à direita — [DateFrom, DateTo). Nil = sem limite naquele
	// lado.
	//
	// O intervalo é meio-aberto de propósito (AUD-003): o Discovery divide o
	// histórico em janela de descoberta e janela de validação usando a MESMA data
	// de corte nas duas pontas. Se o intervalo fosse fechado dos dois lados, as
	// partidas do dia do corte cairiam nos dois conjuntos e o resultado da
	// validação já conteria dado que a mineração viu.
	//
	// Não é ajustável pelo usuário: o Simulador sempre roda sobre o período
	// inteiro selecionado na tela. Diferente de maxAgeDays, que é relativo a
	// time.Now() e portanto muda de significado a cada execução, esta janela é
	// absoluta e torna o backtest reproduzível.
	DateFrom *time.Time `json:"-"`
	DateTo   *time.Time `json:"-"`
}

// inWindow diz se a partida cai na janela [DateFrom, DateTo).
func (c FilterCriteria) inWindow(d time.Time) bool {
	if c.DateFrom != nil && d.Before(*c.DateFrom) {
		return false
	}
	if c.DateTo != nil && !d.Before(*c.DateTo) {
		return false
	}
	return true
}

func (c FilterCriteria) isGoals() bool         { return c.Metric == "goals" }
func (c FilterCriteria) isOffsides() bool      { return c.Metric == "offsides" }
func (c FilterCriteria) isShots() bool         { return c.Metric == "shots" }
func (c FilterCriteria) isShotsOnTarget() bool { return c.Metric == "shots_on_target" }

// fixedOddOrOne devolve a odd fixa simulada (métricas sem odds históricas). 1.0 quando
// não informada — nesse caso o ROI é neutro e vale só a taxa de acerto.
func fixedOddOrOne(o float64) float64 {
	if o <= 0 {
		return 1.0
	}
	return o
}

func (c FilterCriteria) Validate() error {
	switch c.Metric {
	case "", "corners":
		if c.CornersThreshold <= 0 {
			return fmt.Errorf("corners_threshold deve ser maior que zero")
		}
	case "goals":
		if c.GoalsThreshold < 0 {
			return fmt.Errorf("goals_threshold não pode ser negativo")
		}
	case "offsides":
		if c.OffsidesThreshold < 0 {
			return fmt.Errorf("offsides_threshold não pode ser negativo")
		}
	case "shots":
		if c.ShotsThreshold < 0 {
			return fmt.Errorf("shots_threshold não pode ser negativo")
		}
	case "shots_on_target":
		if c.ShotsOnTargetThreshold < 0 {
			return fmt.Errorf("shots_on_target_threshold não pode ser negativo")
		}
	default:
		return fmt.Errorf("metric inválida")
	}
	if c.HomeAway != "" && c.HomeAway != "home" && c.HomeAway != "away" {
		return fmt.Errorf("home_away deve ser 'home', 'away' ou vazio")
	}

	// AUD-004: o filtro por força do adversário está desligado, e recusar é
	// melhor do que ignorar em silêncio.
	//
	// A coluna teams.tier nunca conteve classificação: 228 equipes tinham a
	// constante 'G12' gravada no código de sincronização e 110 tinham o número
	// da divisão da liga. Filtrar por ela devolvia um recorte com aparência
	// estatística e sem conteúdo. Aceitar o parâmetro e ignorá-lo produziria um
	// resultado que o usuário leria como "contra o G6" — pior que um erro.
	//
	// Para religar: classificação POR TEMPORADA, apurada só com os jogos
	// anteriores à data de cada partida. Qualquer coisa menos que isso é
	// look-ahead, porque na 5ª rodada ninguém sabia quem terminaria no G6.
	if c.OpponentTier != "" {
		return fmt.Errorf("filtro por força do adversário indisponível: " +
			"o sistema ainda não calcula classificação por temporada (AUD-004)")
	}
	return nil
}

// BacktestEntry representa uma ocorrência individual (um "jogo-equipe") que atendeu
// aos critérios do filtro.
type BacktestEntry struct {
	MatchID            int64   `json:"match_id"`
	MatchDate          string  `json:"match_date"`
	Team               string  `json:"team"`
	Opponent           string  `json:"opponent"`
	IsHome             bool    `json:"is_home"`
	TotalCorners       int     `json:"total_corners"`
	TotalGoals         int     `json:"total_goals"`
	TotalOffsides      int     `json:"total_offsides"`
	TotalShots         int     `json:"total_shots"`
	TotalShotsOnTarget int     `json:"total_shots_on_target"`
	Hit                bool    `json:"hit"`
	Odd                float64 `json:"odd"`
	ProfitLoss         float64 `json:"profit_loss"`
}

// BacktestResult agrega as métricas do Módulo 3 exigidas pelos critérios de aceite:
// quantidade de partidas, taxa de acerto/erro, média, maior/menor sequência, drawdown,
// ROI, yield e lucro.
type BacktestResult struct {
	Criteria             FilterCriteria  `json:"criteria"`
	Period               string          `json:"period"`
	MatchCount           int             `json:"match_count"`
	Hits                 int             `json:"hits"`
	Misses               int             `json:"misses"`
	HitRate              float64         `json:"hit_rate"`
	MissRate             float64         `json:"miss_rate"`
	AverageCorners       float64         `json:"average_corners"`
	AverageGoals         float64         `json:"average_goals"`
	AverageOffsides      float64         `json:"average_offsides"`
	AverageShots         float64         `json:"average_shots"`
	AverageShotsOnTarget float64         `json:"average_shots_on_target"`
	Metric               string          `json:"metric"`
	LongestWinStreak     int             `json:"longest_win_streak"`
	LongestLoseStreak    int             `json:"longest_lose_streak"`
	MaxDrawdown          float64         `json:"max_drawdown"`
	TotalStaked          float64         `json:"total_staked"`
	Profit               float64         `json:"profit"`
	ROI                  float64         `json:"roi"`
	Yield                float64         `json:"yield"`
	Entries              []BacktestEntry `json:"entries"`
	Disclaimer           string          `json:"disclaimer"`

	// HistoryCapped indica se o resultado foi limitado ao histórico recente (plano
	// gratuito); HistoryCapDays informa o tamanho da janela aplicada. O frontend usa
	// isso para mostrar um aviso "resultado limitado aos últimos N dias — assine o
	// Premium para ver o histórico completo" sem precisar duplicar essa regra.
	HistoryCapped  bool `json:"history_capped"`
	HistoryCapDays int  `json:"history_cap_days,omitempty"`

	// OddsSource e FinancialsReliable expõem a procedência das odds usadas neste
	// backtest (AUD-001).
	//
	//   real      — toda odd veio de mercado; ROI/yield/lucro são medida
	//   synthetic — ao menos uma odd foi derivada do próprio histórico
	//   fixed     — odd única informada pelo usuário (métricas sem odd por partida)
	//   none      — nenhuma odd envolvida
	//
	// FinancialsReliable só é true no caso "real". Nos demais, os campos
	// financeiros descrevem um cenário hipotético e o frontend deve rotulá-los
	// como simulação — nunca como desempenho observado.
	OddsSource         string `json:"odds_source"`
	FinancialsReliable bool   `json:"financials_reliable"`
}

type FilterUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
	leagues repository.LeagueRepository
}

func NewFilterUsecase(matches repository.MatchRepository, teams repository.TeamRepository, leagues repository.LeagueRepository) *FilterUsecase {
	return &FilterUsecase{matches: matches, teams: teams, leagues: leagues}
}

// RunBacktest localiza automaticamente todos os jogos históricos que atendem aos
// filtros informados, dentro do campeonato e das temporadas selecionadas, e calcula
// as métricas de desempenho. Todos os cálculos são determinísticos e reproduzíveis.
//
// maxAgeDays limita a análise às partidas dos últimos N dias (0 = sem limite,
// histórico completo). Usado pelo FilterHandler para aplicar o cap de 90 dias do
// plano gratuito (ver ESTRATEGIA-MONETIZACAO.md — "histórico completo" é recurso
// da Assinatura Premium); usuários premium sempre chamam com 0.
func (u *FilterUsecase) RunBacktest(ctx context.Context, leagueID int64, seasonIDs []int64, criteria FilterCriteria, maxAgeDays int) (*BacktestResult, error) {
	if err := criteria.Validate(); err != nil {
		return nil, err
	}
	stake := criteria.Stake
	if stake <= 0 {
		stake = 1
	}

	allMatches, err := u.matches.AllMatches(ctx, leagueID, seasonIDs)
	if err != nil {
		return nil, err
	}

	if maxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
		filtered := allMatches[:0:0]
		for _, m := range allMatches {
			if !m.MatchDate.Before(cutoff) {
				filtered = append(filtered, m)
			}
		}
		allMatches = filtered
	}

	// AUD-003: janela temporal absoluta. Aplicada ANTES de qualquer outro
	// critério — inclusive antes de LastNGames — para que "últimos N jogos"
	// signifique "os N mais recentes DENTRO da janela". Se fosse aplicada
	// depois, a janela de descoberta poderia selecionar jogos que só existem
	// porque a de validação foi lida.
	if criteria.DateFrom != nil || criteria.DateTo != nil {
		filtered := allMatches[:0:0]
		for _, m := range allMatches {
			if criteria.inWindow(m.MatchDate) {
				filtered = append(filtered, m)
			}
		}
		allMatches = filtered
	}

	teamsByID, err := u.teamIndex(ctx, leagueID)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		match     domain.Match
		teamID    int64
		oppID     int64
		isHome    bool
		cornersF  int
		cornersA  int
		goalsF    int
		goalsA    int
		offsidesF *int
		offsidesA *int
		shotsF    *int
		shotsA    *int
		sotF      *int
		sotA      *int
	}
	var candidates []candidate
	// home = perspectiva do mandante; away = do visitante (for/against invertidos).
	asHome := func(m domain.Match) candidate {
		return candidate{m, m.HomeTeamID, m.AwayTeamID, true, m.HomeCorners, m.AwayCorners, m.HomeGoals, m.AwayGoals, m.HomeOffsides, m.AwayOffsides, m.HomeShots, m.AwayShots, m.HomeShotsOnTarget, m.AwayShotsOnTarget}
	}
	asAway := func(m domain.Match) candidate {
		return candidate{m, m.AwayTeamID, m.HomeTeamID, false, m.AwayCorners, m.HomeCorners, m.AwayGoals, m.HomeGoals, m.AwayOffsides, m.HomeOffsides, m.AwayShots, m.HomeShots, m.AwayShotsOnTarget, m.HomeShotsOnTarget}
	}
	for _, m := range allMatches {
		if criteria.TeamID != nil {
			if *criteria.TeamID == m.HomeTeamID {
				candidates = append(candidates, asHome(m))
			}
			if *criteria.TeamID == m.AwayTeamID {
				candidates = append(candidates, asAway(m))
			}
			continue
		}
		candidates = append(candidates, asHome(m), asAway(m))
	}

	if criteria.LastNGames > 0 {
		byTeam := map[int64][]candidate{}
		for _, c := range candidates {
			byTeam[c.teamID] = append(byTeam[c.teamID], c)
		}
		candidates = nil
		for _, list := range byTeam {
			if len(list) > criteria.LastNGames {
				list = list[len(list)-criteria.LastNGames:]
			}
			candidates = append(candidates, list...)
		}
	}

	entries := make([]BacktestEntry, 0)
	// Procedência das odds efetivamente usadas — alimenta oddsSourceSummary.
	oddsSources := make([]string, 0)
	for _, c := range candidates {
		if criteria.HomeAway == "home" && !c.isHome {
			continue
		}
		if criteria.HomeAway == "away" && c.isHome {
			continue
		}
		// AUD-004: o recorte por força do adversário existia aqui e comparava
		// criteria.OpponentTier com teams.tier — uma coluna que continha uma
		// constante gravada no código. Foi removido junto com o eixo; Validate()
		// agora recusa a requisição antes de chegar neste laço.
		var total, threshold int
		var odd float64
		switch criteria.Metric {
		case "goals":
			// Gols: linha over/under, sem odds históricas — odd fixa simulada (ou 1.0).
			total = c.goalsF + c.goalsA
			threshold = criteria.GoalsThreshold
			odd = fixedOddOrOne(criteria.FixedOdd)
		case "offsides", "shots", "shots_on_target":
			// Métricas nullable no provedor: sem o dado o jogo não entra. Odd fixa simulada.
			var f, a *int
			switch criteria.Metric {
			case "offsides":
				f, a, threshold = c.offsidesF, c.offsidesA, criteria.OffsidesThreshold
			case "shots":
				f, a, threshold = c.shotsF, c.shotsA, criteria.ShotsThreshold
			case "shots_on_target":
				f, a, threshold = c.sotF, c.sotA, criteria.ShotsOnTargetThreshold
			}
			if f == nil || a == nil {
				continue
			}
			total = *f + *a
			odd = fixedOddOrOne(criteria.FixedOdd)
		default:
			// Escanteios: única métrica com odd registrada por partida.
			total = c.cornersF + c.cornersA
			threshold = criteria.CornersThreshold

			// AUD-001: odd sintética é derivada da média de escanteios do próprio
			// lote histórico. Usá-la para validar estratégia é medir o dado com
			// ele mesmo. O Discovery exige procedência real; o Simulador aceita,
			// mas o resultado sai marcado (ver oddsSourceSummary).
			if criteria.RequireRealOdds && !c.match.HasRealOdds() {
				continue
			}

			var hasOdd bool
			odd, hasOdd = c.match.OddForThreshold(criteria.CornersThreshold)
			if criteria.MaxOdds > 0 {
				if !hasOdd || odd > criteria.MaxOdds {
					continue
				}
			}
			if !hasOdd {
				// AUD-012: fabricar odd 1.0 produz P/L estruturalmente negativo e
				// sem significado. Sem odd, a partida não sustenta cálculo
				// financeiro e fica fora do backtest.
				continue
			}
			oddsSources = append(oddsSources, c.match.OddsSource)
		}

		hit := total > threshold

		pl := -stake
		if hit {
			pl = stake * (odd - 1)
		}

		teamName := teamsByID[c.teamID].Name
		oppName := teamsByID[c.oppID].Name

		entry := BacktestEntry{
			MatchID:    c.match.ID,
			MatchDate:  c.match.MatchDate.Format("2006-01-02"),
			Team:       teamName,
			Opponent:   oppName,
			IsHome:     c.isHome,
			Hit:        hit,
			Odd:        odd,
			ProfitLoss: round2(pl),
		}
		switch criteria.Metric {
		case "goals":
			entry.TotalGoals = total
		case "offsides":
			entry.TotalOffsides = total
		case "shots":
			entry.TotalShots = total
		case "shots_on_target":
			entry.TotalShotsOnTarget = total
		default:
			entry.TotalCorners = total
		}
		entries = append(entries, entry)
	}

	result := buildBacktestResult(criteria, entries, stake)
	result.HistoryCapped = maxAgeDays > 0
	result.HistoryCapDays = maxAgeDays
	result.OddsSource, result.FinancialsReliable = oddsSourceSummary(criteria, oddsSources)
	return result, nil
}

// oddsSourceSummary resume a procedência das odds do backtest e decide se os
// números financeiros (ROI, yield, lucro, drawdown) podem ser tratados como
// medida de mercado.
//
// Regra (AUD-001): financeiro só é confiável quando TODAS as odds usadas são
// reais. Basta uma sintética para o conjunto virar cenário hipotético — misturar
// as duas produziria um número sem interpretação possível.
func oddsSourceSummary(criteria FilterCriteria, sources []string) (string, bool) {
	// Métricas sem odd por partida usam a odd fixa informada pelo usuário: é
	// explicitamente uma simulação, nunca uma medida de mercado.
	switch criteria.Metric {
	case "goals", "offsides", "shots", "shots_on_target":
		if criteria.FixedOdd > 0 {
			return "fixed", false
		}
		return "none", false
	}

	if len(sources) == 0 {
		return "none", false
	}
	for _, s := range sources {
		if s != domain.OddsSourceReal {
			return domain.OddsSourceSynthetic, false
		}
	}
	return domain.OddsSourceReal, true
}

func (u *FilterUsecase) teamIndex(ctx context.Context, leagueID int64) (map[int64]domain.Team, error) {
	teams, err := u.teams.List(ctx, &leagueID)
	if err != nil {
		return nil, err
	}
	idx := make(map[int64]domain.Team, len(teams))
	for _, t := range teams {
		idx[t.ID] = t
	}
	return idx, nil
}

func buildBacktestResult(criteria FilterCriteria, entries []BacktestEntry, stake float64) *BacktestResult {
	n := len(entries)
	result := &BacktestResult{
		Criteria:   criteria,
		Period:     "Baseado no campeonato e temporadas selecionadas",
		MatchCount: n,
		Entries:    entries,
		Disclaimer: "Resultado baseado em dados históricos. Não constitui recomendação de aposta nem previsão de resultados futuros.",
	}
	if n == 0 {
		return result
	}

	hits := 0
	totalCorners := 0
	totalGoals := 0
	totalOffsides := 0
	totalShots := 0
	totalSot := 0
	profit := 0.0
	totalStaked := 0.0

	sortByDate(entries)

	curWin, curLose := 0, 0
	maxWin, maxLose := 0, 0
	cumulative := 0.0
	peak := 0.0
	maxDD := 0.0

	for _, e := range entries {
		totalCorners += e.TotalCorners
		totalGoals += e.TotalGoals
		totalOffsides += e.TotalOffsides
		totalShots += e.TotalShots
		totalSot += e.TotalShotsOnTarget
		totalStaked += stake
		profit += e.ProfitLoss
		cumulative += e.ProfitLoss
		if cumulative > peak {
			peak = cumulative
		}
		dd := peak - cumulative
		if dd > maxDD {
			maxDD = dd
		}

		if e.Hit {
			hits++
			curWin++
			curLose = 0
		} else {
			curLose++
			curWin = 0
		}
		if curWin > maxWin {
			maxWin = curWin
		}
		if curLose > maxLose {
			maxLose = curLose
		}
	}

	misses := n - hits
	result.Hits = hits
	result.Misses = misses
	result.HitRate = round2(100 * float64(hits) / float64(n))
	result.MissRate = round2(100 * float64(misses) / float64(n))
	result.AverageCorners = round2(float64(totalCorners) / float64(n))
	result.AverageGoals = round2(float64(totalGoals) / float64(n))
	result.AverageOffsides = round2(float64(totalOffsides) / float64(n))
	result.AverageShots = round2(float64(totalShots) / float64(n))
	result.AverageShotsOnTarget = round2(float64(totalSot) / float64(n))
	result.Metric = criteria.Metric
	result.LongestWinStreak = maxWin
	result.LongestLoseStreak = maxLose
	result.MaxDrawdown = round2(maxDD)
	result.TotalStaked = round2(totalStaked)
	result.Profit = round2(profit)
	if totalStaked > 0 {
		roi := 100 * profit / totalStaked
		result.ROI = round2(roi)
		result.Yield = round2(roi)
	}
	return result
}

func sortByDate(entries []BacktestEntry) {
	for i := 1; i < len(entries); i++ {
		j := i
		for j > 0 && entries[j-1].MatchDate > entries[j].MatchDate {
			entries[j-1], entries[j] = entries[j], entries[j-1]
			j--
		}
	}
}
