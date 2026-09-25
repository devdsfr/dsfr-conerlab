package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P3 — regressão pontual descoberta no REV-P4 (B2 e B2b).
//
// B2: com a deduplicação da correção 1 do REV-P3, a regra de PARTIDA passou a
// gerar só o candidato do mandante, e a janela "últimos N jogos de cada equipe"
// virou "últimos N jogos EM CASA de cada equipe".
//
// B2b: a janela reconstruía os candidatos iterando um map, cuja ordem em Go é
// aleatória. Partidas do mesmo dia trocavam de posição a cada execução, e
// sequências e drawdown mudavam entre execuções idênticas.
//
// Semântica fixada:
//   - REGRA DE PARTIDA: a janela é por equipe e INDEPENDE de mando; uma partida
//     entra se estiver entre as últimas N de qualquer uma das duas equipes, e
//     entra UMA vez.
//   - REGRA DE EQUIPE: a janela respeita a perspectiva de cada equipe (inalterado).
//   - Mando pedido (casa/fora) filtra DEPOIS da janela, como antes do REV-P3.

// alternando monta 8 jogos entre as equipes 10 e 20, alternando o mando:
// ímpares 10 em casa, pares 20 em casa. Datas 1..8 de março de 2025.
func alternando() []domain.Match {
	var ms []domain.Match
	for i := 1; i <= 8; i++ {
		m := partida(int64(i), 6, 5)
		m.MatchDate = time.Date(2025, 3, i, 0, 0, 0, 0, time.UTC)
		if i%2 == 0 {
			m.HomeTeamID, m.AwayTeamID = 20, 10
		}
		ms = append(ms, m)
	}
	return ms
}

func idsDe(res *BacktestResult) []int64 {
	out := make([]int64, 0, len(res.Entries))
	for _, e := range res.Entries {
		out = append(out, e.MatchID)
	}
	return out
}

func mesmos(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rodar(t *testing.T, ms []domain.Match, c FilterCriteria, seasons ...int64) *BacktestResult {
	t.Helper()
	c.Stake, c.FixedOdd, c.AllowMissingOdds = 1, 1.5, true
	if c.CornersThreshold == 0 {
		c.CornersThreshold = 8
	}
	res, err := newFilterUsecaseWith(ms).RunBacktest(context.Background(), 1, seasons, c, 0)
	if err != nil {
		t.Fatalf("backtest falhou: %v", err)
	}
	return res
}

func TestB2_MatchLevel_LastN_IndependeDeMando(t *testing.T) {
	res := rodar(t, alternando(), FilterCriteria{Metric: "corners", LastNGames: 2})
	if got := idsDe(res); !mesmos(got, []int64{7, 8}) {
		t.Fatalf("partidas %v, esperado [7 8]: as duas últimas de cada equipe são 7 e 8. "+
			"[5 6 7 8] é o defeito — 'últimos 2 jogos EM CASA de cada equipe'", got)
	}
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2", res.MatchCount)
	}
}

func TestB2_MatchLevel_LastN_UmaObservacaoPorPartida(t *testing.T) {
	// Janela maior que o histórico: todas as partidas entram, cada uma UMA vez.
	res := rodar(t, alternando(), FilterCriteria{Metric: "corners", LastNGames: 20})
	if res.MatchCount != 8 {
		t.Fatalf("MatchCount = %d, esperado 8 — com janela a regra de partida não pode voltar a contar em dobro", res.MatchCount)
	}
	vistas := map[int64]int{}
	for _, e := range res.Entries {
		vistas[e.MatchID]++
		if !e.IsHome {
			t.Errorf("partida %d representada pelo visitante; o canônico da regra de partida é o mandante", e.MatchID)
		}
	}
	for id, n := range vistas {
		if n != 1 {
			t.Errorf("partida %d apareceu %d vezes", id, n)
		}
	}
}

func TestB2_SoEmCasa_LastN(t *testing.T) {
	// A janela é escolhida com as duas perspectivas; o filtro de mando vem depois.
	// Equipe 10: últimas 2 = 7 (casa) e 8 (fora). Equipe 20: 7 (fora) e 8 (casa).
	// Só em casa: 7 (pela 10) e 8 (pela 20).
	res := rodar(t, alternando(), FilterCriteria{Metric: "corners", LastNGames: 2, HomeAway: "home"})
	if got := idsDe(res); !mesmos(got, []int64{7, 8}) {
		t.Fatalf("partidas %v, esperado [7 8]", got)
	}
	for _, e := range res.Entries {
		if !e.IsHome {
			t.Errorf("partida %d fora de casa num recorte só em casa", e.MatchID)
		}
	}
}

func TestB2_SoFora_LastN(t *testing.T) {
	res := rodar(t, alternando(), FilterCriteria{Metric: "corners", LastNGames: 2, HomeAway: "away"})
	if got := idsDe(res); !mesmos(got, []int64{7, 8}) {
		t.Fatalf("partidas %v, esperado [7 8]", got)
	}
	for _, e := range res.Entries {
		if e.IsHome {
			t.Errorf("partida %d em casa num recorte só fora", e.MatchID)
		}
	}
}

func TestB2_TeamLevel_Preservado(t *testing.T) {
	// Vitória: as duas perspectivas das partidas 7 e 8, uma por equipe.
	res := rodar(t, alternando(), FilterCriteria{Metric: MetricWin, LastNGames: 2})
	if res.MatchCount != 4 {
		t.Fatalf("MatchCount = %d, esperado 4 (7 e 8, pelas duas equipes)", res.MatchCount)
	}
	if got := idsDe(res); !mesmos(got, []int64{7, 7, 8, 8}) {
		t.Errorf("partidas %v, esperado [7 7 8 8]", got)
	}
}

func TestB2_ComEquipeSelecionada(t *testing.T) {
	// Com equipe, a janela é só dela — caminho que o REV-P3 não alterou.
	dez := int64(10)
	res := rodar(t, alternando(), FilterCriteria{Metric: "corners", LastNGames: 3, TeamID: &dez})
	if got := idsDe(res); !mesmos(got, []int64{6, 7, 8}) {
		t.Fatalf("partidas %v, esperado [6 7 8]", got)
	}
}

func TestB2_SemMisturaDeTemporada(t *testing.T) {
	ms := alternando()
	// Temporada 2 com jogos MAIS RECENTES que a 1. Pedindo só a 1, a janela
	// não pode "ver" a 2.
	for i := 9; i <= 12; i++ {
		m := partida(int64(i), 6, 5)
		m.SeasonID = 2
		m.MatchDate = time.Date(2025, 8, i, 0, 0, 0, 0, time.UTC)
		ms = append(ms, m)
	}
	// O stub não filtra por temporada, então o recorte por temporada é feito
	// aqui do mesmo jeito que o repositório real faz na consulta.
	var soT1 []domain.Match
	for _, m := range ms {
		if m.SeasonID == 1 {
			soT1 = append(soT1, m)
		}
	}
	res := rodar(t, soT1, FilterCriteria{Metric: "corners", LastNGames: 2}, 1)
	if got := idsDe(res); !mesmos(got, []int64{7, 8}) {
		t.Fatalf("partidas %v, esperado [7 8] — a janela não pode alcançar a temporada 2", got)
	}
}

func TestB2b_OrdemCronologicaEDeterministica(t *testing.T) {
	// 9 rodadas de 5 jogos no MESMO dia. Antes: 100 execuções davam 26
	// resultados diferentes de sequência/drawdown.
	var ms []domain.Match
	var ts []domain.Team
	for id := int64(1); id <= 10; id++ {
		ts = append(ts, domain.Team{ID: id, Name: fmt.Sprint(id)})
	}
	id := int64(1)
	for r := 0; r < 9; r++ {
		d := time.Date(2025, 3, 1+r*3, 0, 0, 0, 0, time.UTC)
		for k := int64(0); k < 5; k++ {
			m := partida(id, 6, 5)
			m.MatchDate = d
			m.HomeTeamID = 1 + (k+int64(r))%10
			m.AwayTeamID = 1 + (k+5+int64(r)*3)%10
			if m.AwayTeamID == m.HomeTeamID {
				m.AwayTeamID = 1 + (m.HomeTeamID % 10)
			}
			m.HomeGoals, m.AwayGoals = int((id*7)%3), int((id*5)%3)
			ms = append(ms, m)
			id++
		}
	}
	fu := NewFilterUsecase(&stubMatchRepo{matches: ms}, &stubTeamRepo{teams: ts}, stubLeagueRepo{})

	for _, metric := range []string{MetricWin, "corners"} {
		distintos := map[string]bool{}
		for rep := 0; rep < 60; rep++ {
			res, err := fu.RunBacktest(context.Background(), 1, nil, FilterCriteria{
				Metric: metric, CornersThreshold: 8, LastNGames: 4,
				Stake: 1, FixedOdd: 2.0, AllowMissingOdds: true,
			}, 0)
			if err != nil {
				t.Fatal(err)
			}
			seq := ""
			for i, e := range res.Entries {
				if i > 0 {
					prev := res.Entries[i-1]
					if e.MatchDate < prev.MatchDate ||
						(e.MatchDate == prev.MatchDate && e.MatchID < prev.MatchID) {
						t.Fatalf("%s: entrada %d fora de ordem (data, id)", metric, i)
					}
				}
				seq += fmt.Sprintf("%d%v,", e.MatchID, e.IsHome)
			}
			distintos[fmt.Sprintf("%s|%d|%d|%.2f", seq, res.LongestWinStreak, res.LongestLoseStreak, *res.MaxDrawdown)] = true
		}
		if len(distintos) != 1 {
			t.Errorf("%s: 60 execuções idênticas produziram %d resultados diferentes — "+
				"o backtest precisa ser determinístico", metric, len(distintos))
		}
	}
}
