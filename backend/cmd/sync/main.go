// cmd/sync sincroniza campeonatos/temporadas reais a partir de um provedor externo
// (API-Football e/ou SportMonks) para o banco do CornerLab. Requer as chaves de API
// correspondentes configuradas via variáveis de ambiente (ver backend/.env.example).
//
// Exemplo:
//
//	go run ./cmd/sync -league "Brasileirão Série A" -country Brazil -seasons 2024,2025,2026
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/sportmonks"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
	"github.com/devdsfr/cornerlab/internal/usagelog"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/pkg/config"
	"github.com/devdsfr/cornerlab/pkg/database"
)

func main() {
	leagueName := flag.String("league", "Brasileirão Série A", "nome do campeonato a sincronizar")
	country := flag.String("country", "Brazil", "país do campeonato")
	seasonsFlag := flag.String("seasons", "", "anos das temporadas, separados por vírgula (ex: 2024,2025,2026)")
	providerFlag := flag.String("provider", "", "provedor a usar: api_football | sportmonks | fallback (padrão: valor de SPORTS_DATA_PROVIDER)")
	flag.Parse()

	if *seasonsFlag == "" {
		log.Fatal("informe pelo menos uma temporada com -seasons (ex: -seasons 2024,2025,2026)")
	}
	var seasons []int
	for _, s := range strings.Split(*seasonsFlag, ",") {
		year, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			log.Fatalf("temporada inválida: %s", s)
		}
		seasons = append(seasons, year)
	}

	cfg := config.Load()
	providerName := cfg.SportsDataProvider
	if *providerFlag != "" {
		providerName = *providerFlag
	}

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("erro ao conectar no postgres: %v", err)
	}
	defer pool.Close()

	// Registra também aqui (não só na API web) o consumo das chaves de API-Football/
	// SportMonks, para que o painel de diagnóstico "Integrações" reflita chamadas
	// feitas por este comando de sincronização manual.
	usageRepo := postgres.NewUsageRepo(pool)

	provider, err := buildProvider(providerName, cfg.APIFootballKey, cfg.SportMonksKey, usageRepo)
	if err != nil {
		log.Fatalf("erro ao configurar provedor de dados: %v", err)
	}

	syncRepo := postgres.NewSyncRepo(pool)
	syncUC := usecase.NewSyncUsecase(provider, syncRepo)

	fmt.Printf("Sincronizando '%s' (%s) via %s — temporadas: %v\n", *leagueName, *country, provider.Name(), seasons)
	for _, year := range seasons {
		result, err := syncUC.SyncSeason(ctx, *leagueName, *country, year)
		if err != nil {
			log.Fatalf("erro ao sincronizar temporada %d: %v", year, err)
		}
		fmt.Printf("Temporada %d: %d partidas encontradas, %d sincronizadas, %d sem escanteios disponíveis\n",
			result.Season, result.FixturesFound, result.MatchesSynced, result.CornersMissing)
	}
	fmt.Println("Sincronização concluída.")
}

func buildProvider(name, apiFootballKey, sportMonksKey string, recorder usagelog.Recorder) (sportsdata.Provider, error) {
	var primary, secondary sportsdata.Provider
	if apiFootballKey != "" {
		primary = apifootball.New(apiFootballKey, recorder)
	}
	if sportMonksKey != "" {
		secondary = sportmonks.New(sportMonksKey, recorder)
	}

	switch name {
	case "api_football":
		if apiFootballKey == "" {
			return nil, fmt.Errorf("API_FOOTBALL_KEY não configurada")
		}
		return apifootball.New(apiFootballKey, recorder), nil
	case "sportmonks":
		if sportMonksKey == "" {
			return nil, fmt.Errorf("SPORTMONKS_KEY não configurada")
		}
		return sportmonks.New(sportMonksKey, recorder), nil
	case "fallback", "":
		if primary == nil && secondary == nil {
			return nil, fmt.Errorf("nenhuma chave de API configurada (API_FOOTBALL_KEY ou SPORTMONKS_KEY)")
		}
		return &sportsdata.FallbackProvider{Primary: primary, Secondary: secondary}, nil
	default:
		return nil, fmt.Errorf("provedor desconhecido: %s (use api_football, sportmonks ou fallback)", name)
	}
}
