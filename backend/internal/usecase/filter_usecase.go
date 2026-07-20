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
}

func (c FilterCriteria) Validate() error {
	if c.CornersThreshold <= 0 {
		return fmt.Errorf("corners_threshold deve ser maior que zero")
	}
	if c.HomeAway != "" && c.HomeAway != "home" && c.HomeAway != "away" {
		return fmt.Errorf("home_away deve ser 'home', 'away' ou vazio")
	}
	return nil
}

// BacktestEntry representa uma ocorrência individual (um "jogo-equipe") que atendeu
// aos critérios do filtro.
type BacktestEntry struct {
	MatchID      int64   `json:"match_id"`
	MatchDate    string  `json:"match_date"`
	Team         string  `json:"team"`
	Opponent     string  `json:"opponent"`
	IsHome       bool    `json:"is_home"`
	TotalCorners int     `json:"total_corners"`
	Hit          bool    `json:"hit"`
	Odd          float64 `json:"odd"`
	ProfitLoss   float64 `json:"profit_loss"`
}

// BacktestResult agrega as métricas do Módulo 3 exigidas pelos critérios de aceite:
// quantidade de partidas, taxa de acerto/erro, média, maior/menor sequência, drawdown,
// ROI, yield e lucro.
type BacktestResult struct {
	Criteria          FilterCriteria  `json:"criteria"`
	Period            string          `json:"period"`
	MatchCount        int             `json:"match_count"`
	Hits              int             `json:"hits"`
	Misses            int             `json:"misses"`
	HitRate           float64         `json:"hit_rate"`
	MissRate          float64         `json:"miss_rate"`
	AverageCorners    float64         `json:"average_corners"`
	LongestWinStreak  int             `json:"longest_win_streak"`
	LongestLoseStreak int             `json:"longest_lose_streak"`
	MaxDrawdown       float64         `json:"max_drawdown"`
	TotalStaked       float64         `json:"total_staked"`
	Profit            float64         `json:"profit"`
	ROI               float64         `json:"roi"`
	Yield             float64         `json:"yield"`
	Entries           []BacktestEntry `json:"entries"`
	Disclaimer        string          `json:"disclaimer"`

	// HistoryCapped indica se o resultado foi limitado ao histórico recente (plano
	// gratuito); HistoryCapDays informa o tamanho da janela aplicada. O frontend usa
	// isso para mostrar um aviso "resultado limitado aos últimos N dias — assine o
	// Premium para ver o histórico completo" sem precisar duplicar essa regra.
	HistoryCapped  bool `json:"history_capped"`
	HistoryCapDays int  `json:"history_cap_days,omitempty"`
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

	teamsByID, err := u.teamIndex(ctx, leagueID)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		match    domain.Match
		teamID   int64
		oppID    int64
		isHome   bool
		cornersF int
		cornersA int
	}
	var candidates []candidate
	for _, m := range allMatches {
		if criteria.TeamID != nil {
			if *criteria.TeamID == m.HomeTeamID {
				candidates = append(candidates, candidate{m, m.HomeTeamID, m.AwayTeamID, true, m.HomeCorners, m.AwayCorners})
			}
			if *criteria.TeamID == m.AwayTeamID {
				candidates = append(candidates, candidate{m, m.AwayTeamID, m.HomeTeamID, false, m.AwayCorners, m.HomeCorners})
			}
			continue
		}
		candidates = append(candidates, candidate{m, m.HomeTeamID, m.AwayTeamID, true, m.HomeCorners, m.AwayCorners})
		candidates = append(candidates, candidate{m, m.AwayTeamID, m.HomeTeamID, false, m.AwayCorners, m.HomeCorners})
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
	for _, c := range candidates {
		if criteria.HomeAway == "home" && !c.isHome {
			continue
		}
		if criteria.HomeAway == "away" && c.isHome {
			continue
		}
		if criteria.OpponentTier != "" {
			opp, ok := teamsByID[c.oppID]
			if !ok || opp.Tier != criteria.OpponentTier {
				continue
			}
		}
		odd, hasOdd := c.match.OddForThreshold(criteria.CornersThreshold)
		if criteria.MaxOdds > 0 {
			if !hasOdd || odd > criteria.MaxOdds {
				continue
			}
		}
		if !hasOdd {
			odd = 1.0
		}

		total := c.cornersF + c.cornersA
		hit := total > criteria.CornersThreshold

		pl := -stake
		if hit {
			pl = stake * (odd - 1)
		}

		teamName := teamsByID[c.teamID].Name
		oppName := teamsByID[c.oppID].Name

		entries = append(entries, BacktestEntry{
			MatchID:      c.match.ID,
			MatchDate:    c.match.MatchDate.Format("2006-01-02"),
			Team:         teamName,
			Opponent:     oppName,
			IsHome:       c.isHome,
			TotalCorners: total,
			Hit:          hit,
			Odd:          odd,
			ProfitLoss:   round2(pl),
		})
	}

	result := buildBacktestResult(criteria, entries, stake)
	result.HistoryCapped = maxAgeDays > 0
	result.HistoryCapDays = maxAgeDays
	return result, nil
}

func (u *FilterUsecase) teamIndex(ctx context.Context, leagueID int64) (map[int64]domain.Team, error) {
	teams, err := u.teams.List(ctx, &leagueID, nil)
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
