// cmd/seed gera um conjunto de dados de exemplo (times, campeonato, temporadas,
// partidas com escanteios e odds sintéticas) para permitir validar toda a lógica
// estatística, de backtesting e de simulação financeira do CornerLab sem depender
// de uma fonte de dados real. Os valores são fictícios/realistas — não representam
// jogos reais.
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/devdsfr/cornerlab/pkg/config"
	"github.com/devdsfr/cornerlab/pkg/database"
)

type teamSeed struct {
	name      string
	shortName string
	tier      string
	attack    float64 // tendência de escanteios a favor
	defense   float64 // tendência de escanteios cedidos ao adversário
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("erro ao conectar no postgres: %v", err)
	}
	defer pool.Close()

	rng := rand.New(rand.NewSource(42))

	leagueID := seedLeague(ctx, pool)
	seasonIDs := seedSeasons(ctx, pool, leagueID, []int{2022, 2023, 2024, 2025})
	teamIDs := seedTeams(ctx, pool, leagueID, teams())
	seedDemoUser(ctx, pool)

	total := 0
	for year, seasonID := range seasonIDs {
		n := seedSeasonMatches(ctx, pool, rng, leagueID, seasonID, teamIDs, year)
		total += n
		fmt.Printf("Temporada %d: %d partidas geradas\n", year, n)
	}
	fmt.Printf("Seed concluído: %d partidas no total.\n", total)
}

func seedLeague(ctx context.Context, pool *pgxpool.Pool) int64 {
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO leagues (name, country, tier) VALUES ($1, $2, $3)
		RETURNING id`, "Brasileirão Série A (exemplo)", "Brasil", "G6").Scan(&id)
	if err != nil {
		log.Fatalf("erro ao inserir campeonato: %v", err)
	}
	return id
}

func seedSeasons(ctx context.Context, pool *pgxpool.Pool, leagueID int64, years []int) map[int]int64 {
	result := map[int]int64{}
	for _, y := range years {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO seasons (league_id, year, label) VALUES ($1, $2, $3)
			RETURNING id`, leagueID, y, fmt.Sprintf("%d", y)).Scan(&id)
		if err != nil {
			log.Fatalf("erro ao inserir temporada %d: %v", y, err)
		}
		result[y] = id
	}
	return result
}

func teams() []teamSeed {
	return []teamSeed{
		{"Atlético Fenix", "AFX", "G6", 6.4, 4.6},
		{"Vasco da Serra", "VDS", "G6", 6.1, 4.8},
		{"Palmeiras do Vale", "PDV", "G6", 6.6, 4.3},
		{"Flamengo Litoral", "FLL", "G6", 6.3, 4.7},
		{"São Bento FC", "SBF", "G6", 6.0, 5.0},
		{"Corinthians Norte", "CTN", "G6", 5.9, 5.1},
		{"Grêmio das Águas", "GDA", "G12", 5.4, 5.4},
		{"Internacional Sul", "ISU", "G12", 5.3, 5.5},
		{"Cruzeiro Estrela", "CZE", "G12", 5.2, 5.3},
		{"Bahia Litorâneo", "BLT", "G12", 5.1, 5.6},
		{"Fortaleza Praia", "FTP", "G12", 5.0, 5.5},
		{"Athletico Serra", "ATS", "G12", 5.2, 5.4},
		{"Botafogo Central", "BFC", "G12", 4.9, 5.7},
		{"Fluminense Rio", "FLR", "G12", 5.1, 5.2},
		{"Santos Baixada", "STB", "G12", 4.8, 5.8},
		{"Sport Recifense", "SPR", "Z4", 4.4, 6.2},
		{"Ceará Vozão", "CRV", "Z4", 4.3, 6.4},
		{"Coritiba Verde", "CTB", "Z4", 4.2, 6.5},
		{"Goiás Esmeraldino", "GOE", "Z4", 4.1, 6.6},
		{"Vitória Leão", "VTL", "Z4", 4.5, 6.1},
	}
}

func seedTeams(ctx context.Context, pool *pgxpool.Pool, leagueID int64, list []teamSeed) []int64 {
	ids := make([]int64, len(list))
	for i, t := range list {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO teams (name, short_name, country, tier) VALUES ($1, $2, $3, $4)
			RETURNING id`, t.name, t.shortName, "Brasil", t.tier).Scan(&id)
		if err != nil {
			log.Fatalf("erro ao inserir equipe %s: %v", t.name, err)
		}
		ids[i] = id
		if _, err := pool.Exec(ctx, `INSERT INTO league_teams (league_id, team_id) VALUES ($1, $2)`, leagueID, id); err != nil {
			log.Fatalf("erro ao vincular equipe ao campeonato: %v", err)
		}
	}
	return ids
}

func seedDemoUser(ctx context.Context, pool *pgxpool.Pool) {
	hash, err := bcrypt.GenerateFromPassword([]byte("demo12345"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("erro ao gerar hash de senha: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING`, "Usuário Demo", "demo@cornerlab.app", string(hash))
	if err != nil {
		log.Fatalf("erro ao inserir usuário demo: %v", err)
	}
	fmt.Println("Usuário demo: demo@cornerlab.app / senha: demo12345")
}

// seedSeasonMatches gera um turno-returno simples (round robin duplo) entre as
// equipes fornecidas, com datas espaçadas ao longo do ano, e insere as partidas.
func seedSeasonMatches(ctx context.Context, pool *pgxpool.Pool, rng *rand.Rand, leagueID, seasonID int64, teamIDs []int64, year int) int {
	teamList := teams()
	idxByID := map[int64]teamSeed{}
	for i, id := range teamIDs {
		idxByID[id] = teamList[i]
	}

	schedule := roundRobin(teamIDs)
	startDate := time.Date(year, time.April, 1, 16, 0, 0, 0, time.UTC)

	count := 0
	round := 1
	for _, day := range schedule {
		matchDate := startDate.AddDate(0, 0, (round-1)*7)
		for _, pair := range day {
			home, away := pair[0], pair[1]
			ht, at := idxByID[home], idxByID[away]

			homeMu := (ht.attack + at.defense) / 2.0 * 1.08 // vantagem de mandante
			awayMu := (at.attack + ht.defense) / 2.0 * 0.94

			homeCorners := poisson(rng, homeMu)
			awayCorners := poisson(rng, awayMu)
			homeGoals := poisson(rng, 1.3)
			awayGoals := poisson(rng, 1.1)

			totalMu := homeMu + awayMu
			odds := buildCornerOdds(totalMu)

			var matchID int64
			err := pool.QueryRow(ctx, `
				INSERT INTO matches (league_id, season_id, round, match_date, home_team_id, away_team_id,
					home_corners, away_corners, home_goals, away_goals, corner_odds)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
				RETURNING id`,
				leagueID, seasonID, round, matchDate, home, away,
				homeCorners, awayCorners, homeGoals, awayGoals, oddsToJSON(odds)).Scan(&matchID)
			if err != nil {
				log.Fatalf("erro ao inserir partida: %v", err)
			}
			count++
		}
		round++
	}
	return count
}

// roundRobin gera as rodadas de um turno-returno completo (ida e volta) usando o
// método do círculo, retornando uma lista de rodadas, cada uma com pares [home, away].
func roundRobin(teamIDs []int64) [][][2]int64 {
	n := len(teamIDs)
	if n%2 != 0 {
		teamIDs = append(teamIDs, -1) // bye
		n++
	}
	half := n / 2
	arr := append([]int64(nil), teamIDs...)

	var firstLeg [][][2]int64
	for round := 0; round < n-1; round++ {
		var pairs [][2]int64
		for i := 0; i < half; i++ {
			a, b := arr[i], arr[n-1-i]
			if a != -1 && b != -1 {
				if round%2 == 0 {
					pairs = append(pairs, [2]int64{a, b})
				} else {
					pairs = append(pairs, [2]int64{b, a})
				}
			}
		}
		firstLeg = append(firstLeg, pairs)
		// rotate (mantém arr[0] fixo)
		last := arr[n-1]
		copy(arr[2:], arr[1:n-1])
		arr[1] = last
	}

	// turno-returno: inverte mandante/visitante
	var secondLeg [][][2]int64
	for _, pairs := range firstLeg {
		var inverted [][2]int64
		for _, p := range pairs {
			inverted = append(inverted, [2]int64{p[1], p[0]})
		}
		secondLeg = append(secondLeg, inverted)
	}

	return append(firstLeg, secondLeg...)
}

// poisson amostra um valor inteiro >=0 de uma distribuição de Poisson (algoritmo de Knuth).
func poisson(rng *rand.Rand, lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	l := math.Exp(-lambda)
	k := 0
	p := 1.0
	for {
		k++
		p *= rng.Float64()
		if p <= l {
			break
		}
	}
	return k - 1
}

// buildCornerOdds calcula odds sintéticas para as linhas 4.5 a 10.5 escanteios,
// a partir de uma aproximação normal (média = totalMu, desvio padrão = 3.0) com uma
// margem de casa de apostas de ~8%, apenas para fins de simulação/backtesting.
func buildCornerOdds(totalMu float64) map[string]float64 {
	stdDev := 3.0
	margin := 1.08
	odds := map[string]float64{}
	for _, line := range []float64{4.5, 5.5, 6.5, 7.5, 8.5, 9.5, 10.5} {
		prob := 1 - normalCDF(line, totalMu, stdDev)
		if prob < 0.03 {
			prob = 0.03
		}
		if prob > 0.97 {
			prob = 0.97
		}
		odd := math.Round((1/prob)*margin*100) / 100
		odds[fmt.Sprintf("%.1f", line)] = odd
	}
	return odds
}

func normalCDF(x, mean, stdDev float64) float64 {
	return 0.5 * (1 + math.Erf((x-mean)/(stdDev*math.Sqrt2)))
}

func oddsToJSON(odds map[string]float64) string {
	s := "{"
	first := true
	for _, line := range []string{"4.5", "5.5", "6.5", "7.5", "8.5", "9.5", "10.5"} {
		if !first {
			s += ","
		}
		first = false
		s += fmt.Sprintf(`"%s":%.2f`, line, odds[line])
	}
	s += "}"
	return s
}
