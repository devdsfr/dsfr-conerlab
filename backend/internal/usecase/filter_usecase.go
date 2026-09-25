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

	// AllowMissingOdds mantém no backtest as partidas SEM odd, para que o
	// resultado estatístico (amostra, acertos, taxa) exista mesmo quando não há
	// cenário financeiro possível.
	//
	// É ligado só pelo SIMULADOR. O Strategy Engine e o Discovery deixam em
	// false porque pontuar uma estratégia exige série financeira completa — sem
	// ela o score financeiro seria zero, e zero ali significaria "rendeu nada"
	// em vez de "não dá para medir". São perguntas diferentes, e por isso os
	// dois caminhos divergem aqui em vez de compartilharem um default.
	AllowMissingOdds bool `json:"-"`

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

// Mercados de RESULTADO. Diferente de todas as outras métricas do CornerLab,
// que são limiar sobre um total ("mais de N escanteios"), estas são desfecho da
// partida — e por isso têm tratamento próprio no motor e odd em outro mapa
// (Match.ResultOdds).
//
// Sempre na perspectiva do time analisado: "vitória" é vitória DELE, seja em
// casa ou fora. O motor traduz para mandante/visitante na hora de buscar a odd.
const (
	MetricWin       = "win"         // o time vence
	MetricDraw      = "draw"        // a partida termina empatada
	MetricWinOrDraw = "win_or_draw" // o time não perde (dupla chance)
)

func isResultMetric(metric string) bool {
	switch metric {
	case MetricWin, MetricDraw, MetricWinOrDraw:
		return true
	}
	return false
}

// resultOutcome traduz a métrica (perspectiva do time) para a chave do desfecho
// em Match.ResultOdds (perspectiva da partida).
//
// Empate é o mesmo desfecho dos dois lados; vitória e dupla chance dependem do
// mando.
func resultOutcome(metric string, isHome bool) string {
	switch metric {
	case MetricDraw:
		return domain.ResultOutcomeDraw
	case MetricWinOrDraw:
		if isHome {
			return domain.ResultOutcomeHomeOrDraw
		}
		return domain.ResultOutcomeAwayOrDraw
	default: // MetricWin
		if isHome {
			return domain.ResultOutcomeHome
		}
		return domain.ResultOutcomeAway
	}
}

func (c FilterCriteria) isGoals() bool         { return c.Metric == "goals" }
func (c FilterCriteria) isOffsides() bool      { return c.Metric == "offsides" }
func (c FilterCriteria) isShots() bool         { return c.Metric == "shots" }
func (c FilterCriteria) isShotsOnTarget() bool { return c.Metric == "shots_on_target" }

// fixedOddOrOne devolve a odd fixa simulada (métricas sem odds históricas). 1.0 quando
// não informada — nesse caso o ROI é neutro e vale só a taxa de acerto.
// CLASSIFICAÇÃO DAS REGRAS — match-level × team-level (REV-P3, correção 1).
//
// A distinção decide quantas observações uma partida produz quando NENHUMA
// equipe foi selecionada.
//
//	MATCH-LEVEL — a regra é propriedade da PARTIDA. O valor é idêntico
//	              olhando do mandante ou do visitante, porque soma os dois
//	              lados: escanteios, gols, impedimentos, chutes, chutes no gol.
//	              "Real Madrid 2 x 1 Barcelona, total de gols = 3" é UMA
//	              observação, não duas.
//
//	TEAM-LEVEL  — a perspectiva muda o resultado. Vitória, empate e dupla
//	              chance são desfecho: na mesma partida o Real venceu e o
//	              Barcelona não. As duas observações são legítimas e
//	              diferentes, e continuam valendo duas.
//
// Por isso a correção NÃO é `unique(match_id)` no engine inteiro: isso
// destruiria metade das observações dos mercados de resultado.
func isMatchLevelMetric(metric string) bool {
	return !isResultMetric(metric)
}

// oddParaEntrada resolve a odd de uma partida sem métrica de odd própria
// (gols, impedimentos, chutes, chutes no gol).
//
// ANTES existia fixedOddOrOne, que devolvia 1.0 quando o usuário não informava
// odd. Odd 1.00 produz lucro zero no acerto e −stake no erro: uma série
// estruturalmente negativa que parecia resultado financeiro e não era. Ausência
// de odd não é odd 1,00.
//
// Agora devolve nil, e o ciclo segue calculando a parte ESTATÍSTICA (amostra,
// acertos, taxa) sem inventar a parte financeira.
func oddOpcional(o float64) *float64 {
	if o <= 0 {
		return nil
	}
	v := o
	return &v
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
	case MetricWin, MetricDraw, MetricWinOrDraw:
		// Mercados de resultado não têm limiar: o desfecho da partida já é a
		// resposta. Nada a validar aqui.
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
	MatchID            int64  `json:"match_id"`
	MatchDate          string `json:"match_date"`
	Team               string `json:"team"`
	Opponent           string `json:"opponent"`
	IsHome             bool   `json:"is_home"`
	TotalCorners       int    `json:"total_corners"`
	TotalGoals         int    `json:"total_goals"`
	TotalOffsides      int    `json:"total_offsides"`
	TotalShots         int    `json:"total_shots"`
	TotalShotsOnTarget int    `json:"total_shots_on_target"`

	// Placar na perspectiva do time analisado. Preenchido só nos mercados de
	// resultado (vitória/empate/dupla chance), onde é ele que explica o acerto.
	GoalsFor     int `json:"goals_for,omitempty"`
	GoalsAgainst int `json:"goals_against,omitempty"`

	Hit bool `json:"hit"`

	// Odd e ProfitLoss são NULÁVEIS. nil = não havia odd para esta partida, e
	// portanto não existe cenário financeiro para ela. Antes vinham 1.0 e
	// −stake/0, o que parecia resultado e era artefato.
	Odd        *float64 `json:"odd"`
	ProfitLoss *float64 `json:"profit_loss"`

	// OddsSource da odd usada NESTA entrada: real | synthetic | fixed. Vazio
	// quando não houve odd. Permite auditar entrada a entrada, e não só o
	// resumo do backtest.
	OddsSource string `json:"odds_source,omitempty"`
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
	Entries              []BacktestEntry `json:"entries"`
	Disclaimer           string          `json:"disclaimer"`

	// --- BLOCO FINANCEIRO — todo nulável -------------------------------------
	//
	// A separação entre estatístico e financeiro é o coração da correção 3 do
	// REV-P3. O bloco acima (amostra, acertos, taxa, médias, sequências) existe
	// SEMPRE que houver partidas, com ou sem odd. O bloco abaixo só existe
	// quando há odd — e `nil` quer dizer "não calculável", nunca zero.
	//
	// Antes, sem odd o sistema usava 1.00 e produzia ROI, lucro e drawdown com
	// aparência de medida. Eram artefatos da odd fabricada.
	FinancialsAvailable bool     `json:"financials_available"`
	FinancialsNote      string   `json:"financials_note,omitempty"`
	MaxDrawdown         *float64 `json:"max_drawdown"`
	TotalStaked         *float64 `json:"total_staked"`
	Profit              *float64 `json:"profit"`
	ROI                 *float64 `json:"roi"`

	// Yield repete ROI de propósito e o contrato diz isso em voz alta: sob stake
	// fixa as duas grandezas são a MESMA divisão (lucro ÷ total apostado).
	// Mantido só para não quebrar quem já lê o campo; a interface mostra um
	// rótulo único "ROI / Yield" (AUD-002).
	Yield *float64 `json:"yield"`

	// EV permanece ausente do contrato. Calcular valor esperado exigiria uma
	// probabilidade INDEPENDENTE; usar a taxa de acerto do próprio lote faz o EV
	// colapsar no ROI já realizado. Ver aidocs e AUD-002.

	// --- Recorte efetivamente analisado --------------------------------------
	//
	// O cap do plano gratuito é relativo a time.Now(), então a MESMA
	// configuração analisa um conjunto diferente a cada dia. Devolver as datas
	// efetivas torna o recorte auditável em vez de implícito.
	EffectiveFrom string `json:"effective_from,omitempty"`
	EffectiveTo   string `json:"effective_to,omitempty"`

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

	// REV-P3 / correção 6: o recorte efetivamente analisado precisa ser auditável.
	// O cap do plano gratuito é relativo a time.Now(), então a MESMA configuração
	// analisa um conjunto diferente a cada dia. Guardamos as datas resultantes para
	// devolvê-las no resultado — sem elas, dois backtests "iguais" com números
	// diferentes são indistinguíveis de um bug.
	var efetivoDe, efetivoAte time.Time
	if maxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
		efetivoDe = cutoff
		filtered := allMatches[:0:0]
		for _, m := range allMatches {
			if !m.MatchDate.Before(cutoff) {
				filtered = append(filtered, m)
			}
		}
		allMatches = filtered
	}
	if criteria.DateFrom != nil && (efetivoDe.IsZero() || criteria.DateFrom.After(efetivoDe)) {
		efetivoDe = *criteria.DateFrom
	}
	if criteria.DateTo != nil {
		efetivoAte = *criteria.DateTo
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
	// REV-P3 correção 1 — dupla contagem (AUD-021).
	//
	// Sem equipe selecionada, cada partida era expandida nas duas perspectivas.
	// Para regra MATCH-LEVEL as duas produzem o MESMO valor, então a partida
	// virava duas ocorrências idênticas: medido em produção, La Liga 2026 dava
	// 138 entradas para 69 partidas e o Brasileirão 200 para 100. Isso dobrava
	// contagem, exposição financeira, drawdown e sequências de derrota.
	//
	// A correção NÃO é `unique(match_id)` global: para regra TEAM-LEVEL
	// (vitória/empate/dupla chance) as duas perspectivas são observações
	// legítimas e diferentes — na mesma partida um time venceu e o outro não.
	//
	// Nota sobre o filtro de mando: com regra match-level e sem equipe, "Casa" e
	// "Fora" já produziam uma entrada por partida (o laço abaixo descartava a
	// outra perspectiva). O defeito era exclusivo de "Qualquer", e é só ele que
	// muda aqui — a perspectiva do mandante vira a canônica.
	matchLevel := isMatchLevelMetric(criteria.Metric)
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
		if matchLevel && criteria.HomeAway == "" {
			candidates = append(candidates, asHome(m))
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

		// odd é NULÁVEL: nil significa "esta partida não tem odd", e não
		// "odd 1,00". oddSrc registra a procedência entrada a entrada.
		var odd *float64
		var oddSrc string

		// hit é resolvido de duas formas diferentes, e por isso mora fora do switch.
		//
		// Quase toda métrica do CornerLab é "o total passou de N?" — escanteios,
		// gols, chutes, impedimentos. Mercados de RESULTADO não são: vitória e
		// empate são desfecho, não limiar. Esses ramos decidem hit diretamente e
		// marcam resolvido = true; os demais caem na comparação padrão logo abaixo.
		var hit, resolvido bool

		switch criteria.Metric {
		case MetricWin, MetricDraw, MetricWinOrDraw:
			// Desfecho da partida. O dado já está no banco (home_goals/away_goals),
			// então não custa nenhuma requisição nova ao provedor.
			//
			// goalsF/goalsA já vêm na perspectiva do time analisado (ver asHome e
			// asAway), então "venceu" é a mesma comparação dos dois lados.
			switch criteria.Metric {
			case MetricWin:
				hit = c.goalsF > c.goalsA
			case MetricDraw:
				hit = c.goalsF == c.goalsA
			case MetricWinOrDraw:
				hit = c.goalsF >= c.goalsA
			}
			resolvido = true

			// AUD-001, mesma regra dos escanteios: o Discovery só valida sobre odd
			// de mercado. Aqui ela nunca existe ainda — nenhuma rota grava
			// result_odds —, então o Discovery descarta tudo. É o comportamento
			// correto: sem odd não há hipótese nula e não há o que validar.
			if criteria.RequireRealOdds && !c.match.HasRealResultOdds() {
				continue
			}

			oddMercado, hasOdd := c.match.OddForResultOutcome(resultOutcome(criteria.Metric, c.isHome))
			if hasOdd {
				if criteria.MaxOdds > 0 && oddMercado > criteria.MaxOdds {
					continue
				}
				odd = &oddMercado
				oddSrc = c.match.ResultOddsSource
				oddsSources = append(oddsSources, oddSrc)
				break
			}
			// Odd fixa informada na tela é o caminho do Simulador: deixa o
			// usuário testar um cenário sem que o número vire evidência de
			// mercado (marcado como "fixed", nunca confiável).
			if o := oddOpcional(criteria.FixedOdd); o != nil {
				odd, oddSrc = o, oddsSourceFixed
				break
			}
			// Sem odd: no Simulador a partida CONTINUA, contribuindo para a
			// parte estatística; no Engine/Discovery ela sai. AUD-012: em
			// nenhum dos dois se fabrica 1.0.
			if !criteria.AllowMissingOdds {
				continue
			}
		case "goals":
			// Gols: linha over/under, sem odds históricas — odd fixa simulada (ou 1.0).
			total = c.goalsF + c.goalsA
			threshold = criteria.GoalsThreshold
			if odd = oddOpcional(criteria.FixedOdd); odd != nil {
				oddSrc = oddsSourceFixed
			}
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
			if odd = oddOpcional(criteria.FixedOdd); odd != nil {
				oddSrc = oddsSourceFixed
			}
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

			oddMercado, hasOdd := c.match.OddForThreshold(criteria.CornersThreshold)
			switch {
			case hasOdd:
				// "Odds máximas" é filtro de ELEGIBILIDADE sobre odd de mercado:
				// descarta a partida cuja odd registrada passe do teto.
				if criteria.MaxOdds > 0 && oddMercado > criteria.MaxOdds {
					continue
				}
				odd = &oddMercado
				oddSrc = c.match.OddsSource
				oddsSources = append(oddsSources, oddSrc)

			case criteria.FixedOdd > 0:
				// REV-P3 correção 2. O ramo de escanteios era o ÚNICO que não
				// consultava a odd fixa, e por isso a métrica principal do
				// produto devolvia zero ocorrência em toda simulação recente:
				// as partidas sincronizadas pelo worker não têm corner_odds, e
				// sem odd a partida era descartada.
				//
				// Medido em produção (Brasileirão 2026, anônimo): gols > 2
				// devolvia 200 entradas; escanteios devolvia 0 em TODOS os
				// limiares, inclusive com odd fixa informada.
				//
				// A odd fixa entra como CENÁRIO, marcada "fixed" — jamais
				// "real". Não é odd histórica de mercado.
				odd, oddSrc = oddOpcional(criteria.FixedOdd), oddsSourceFixed

			default:
				// Sem odd de mercado e sem odd fixa.
				//
				// Para o Simulador (AllowMissingOdds) a partida PERMANECE: ela
				// sustenta a parte estatística, e o bloco financeiro fica
				// indisponível. AUD-012 continua valendo — não se fabrica 1.0.
				//
				// Para o Strategy Engine e o Discovery a partida sai, como
				// sempre saiu: pontuar estratégia exige série financeira.
				if !criteria.AllowMissingOdds {
					continue
				}
			}
		}

		if !resolvido {
			hit = total > threshold
		}

		// P/L só existe quando existe odd. WIN = stake × (odd − 1); LOSS = −stake.
		var pl *float64
		if odd != nil {
			v := -stake
			if hit {
				v = stake * (*odd - 1)
			}
			v = round2(v)
			pl = &v
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
			ProfitLoss: pl,
			OddsSource: oddSrc,
		}
		switch criteria.Metric {
		case MetricWin, MetricDraw, MetricWinOrDraw:
			// O placar é o que explica o acerto neste mercado — sem ele, a linha da
			// tabela mostraria "acertou" sem dizer por quê.
			entry.GoalsFor = c.goalsF
			entry.GoalsAgainst = c.goalsA
			entry.TotalGoals = c.goalsF + c.goalsA
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

	// REV-P3 / correção 6: janela efetiva. Preferimos o limite DECLARADO (cap do
	// plano + date_from/date_to), porque é ele que define o que podia ter entrado.
	// Sem limite declarado caímos na primeira/última partida realmente analisada —
	// é o recorte observado, e dizemos qual dos dois é ao devolver só o que existe.
	// Não assumimos ordenação de allMatches: varremos para achar os extremos.
	var obsDe, obsAte time.Time
	for _, m := range allMatches {
		if obsDe.IsZero() || m.MatchDate.Before(obsDe) {
			obsDe = m.MatchDate
		}
		if obsAte.IsZero() || m.MatchDate.After(obsAte) {
			obsAte = m.MatchDate
		}
	}
	if efetivoDe.IsZero() {
		efetivoDe = obsDe
	}
	if efetivoAte.IsZero() {
		efetivoAte = obsAte
	}
	if !efetivoDe.IsZero() {
		result.EffectiveFrom = efetivoDe.Format("2006-01-02")
	}
	if !efetivoAte.IsZero() {
		result.EffectiveTo = efetivoAte.Format("2006-01-02")
	}
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
	// Métricas sem odd por partida (gols, impedimentos, chutes): só existe
	// cenário quando o usuário informa a odd.
	switch criteria.Metric {
	case "goals", "offsides", "shots", "shots_on_target":
		if criteria.FixedOdd > 0 {
			return oddsSourceFixed, false
		}
		return oddsSourceNone, false
	}

	if len(sources) == 0 {
		// Escanteios e mercados de resultado aceitam odd fixa quando não há odd
		// de mercado registrada — e nesse caso o resultado é CENÁRIO, não
		// medição. É o caminho que destravou o backtest de escanteios.
		if criteria.FixedOdd > 0 {
			return oddsSourceFixed, false
		}
		return oddsSourceNone, false
	}
	for _, s := range sources {
		if s != domain.OddsSourceReal {
			return domain.OddsSourceSynthetic, false
		}
	}
	return domain.OddsSourceReal, true
}

// Procedências que não vêm do banco. `fixed` é a odd digitada pelo usuário —
// cenário hipotético; `none` é ausência de qualquer odd.
const (
	oddsSourceFixed = "fixed"
	oddsSourceNone  = "none"
)

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

	// comFinanceiro conta as entradas que têm odd. O bloco financeiro só é
	// publicado quando TODAS as entradas têm — uma série com buracos produziria
	// ROI e drawdown sobre uma amostra diferente da estatística, e as duas
	// apareceriam lado a lado como se falassem da mesma coisa.
	comFinanceiro := 0

	for _, e := range entries {
		totalCorners += e.TotalCorners
		totalGoals += e.TotalGoals
		totalOffsides += e.TotalOffsides
		totalShots += e.TotalShots
		totalSot += e.TotalShotsOnTarget

		if e.ProfitLoss != nil {
			comFinanceiro++
			totalStaked += stake
			profit += *e.ProfitLoss
			cumulative += *e.ProfitLoss
			if cumulative > peak {
				peak = cumulative
			}
			if dd := peak - cumulative; dd > maxDD {
				maxDD = dd
			}
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

	// Bloco financeiro: publicado só com série completa. Fora disso os campos
	// ficam nil — "não calculável" —, nunca zero.
	if comFinanceiro == n && totalStaked > 0 {
		dd := round2(maxDD)
		ts := round2(totalStaked)
		pf := round2(profit)
		// ROI = Yield = lucro ÷ total apostado. Sob stake fixa são a mesma
		// divisão; o contrato expõe os dois só por compatibilidade e a tela usa
		// rótulo único (AUD-002).
		roi := round2(100 * profit / totalStaked)
		result.FinancialsAvailable = true
		result.MaxDrawdown, result.TotalStaked, result.Profit = &dd, &ts, &pf
		result.ROI, result.Yield = &roi, &roi
	} else if n > 0 {
		result.FinancialsNote = "Sem odd para estas partidas: o resultado abaixo é " +
			"estatístico. Informe uma odd fixa para simular um cenário financeiro."
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
