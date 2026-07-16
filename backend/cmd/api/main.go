package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	httpDelivery "github.com/devdsfr/cornerlab/internal/delivery/http"
	"github.com/devdsfr/cornerlab/internal/delivery/http/handlers"
	"github.com/devdsfr/cornerlab/internal/integration/llm"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/sportmonks"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/bankroll"
	"github.com/devdsfr/cornerlab/internal/usecase/billing"
	"github.com/devdsfr/cornerlab/internal/usecase/diagnostics"
	"github.com/devdsfr/cornerlab/internal/usecase/intelligence"
	"github.com/devdsfr/cornerlab/pkg/cache"
	"github.com/devdsfr/cornerlab/pkg/config"
	"github.com/devdsfr/cornerlab/pkg/database"
	"github.com/devdsfr/cornerlab/pkg/logger"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	appLog := logger.New(cfg.Environment)
	slog.SetDefault(appLog)

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		appLog.Error("falha ao conectar no postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient := cache.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword)
	defer redisClient.Close()

	// Repositórios
	leagueRepo := postgres.NewLeagueRepo(pool)
	teamRepo := postgres.NewTeamRepo(pool)
	matchRepo := postgres.NewMatchRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	filterRepo := postgres.NewFilterRepo(pool)
	betRepo := postgres.NewBetRepo(pool)
	alertRepo := postgres.NewAlertRuleRepo(pool)
	strategyHistoryRepo := postgres.NewStrategyHistoryRepo(pool)
	leagueStatsRepo := postgres.NewLeagueStatsRepo(pool)
	usageRepo := postgres.NewUsageRepo(pool)
	bankrollRepo := postgres.NewBankrollRepo(pool)

	// Usecases — módulos originais (Dashboard, Comparador, Filtros, Auth, Apostas)
	authUC := usecase.NewAuthUsecase(userRepo, cfg.JWTSecret, cfg.JWTExpiry)
	dashboardUC := usecase.NewDashboardUsecase(matchRepo, teamRepo)
	comparatorUC := usecase.NewComparatorUsecase(matchRepo, teamRepo)
	filterUC := usecase.NewFilterUsecase(matchRepo, teamRepo, leagueRepo)
	betUC := usecase.NewBetUsecase(betRepo)
	strategyHistoryUC := usecase.NewStrategyHistoryUsecase(strategyHistoryRepo)
	bankrollUC := bankroll.New(bankrollRepo, betRepo)
	billingUC := billing.New(userRepo, cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripePriceID, cfg.StripeTrialDays, cfg.FrontendURL)

	// Usecases — Módulo de Inteligência Estatística
	consistencyUC := intelligence.NewConsistencyUsecase(matchRepo, teamRepo, leagueRepo)
	trendUC := intelligence.NewTrendUsecase(matchRepo, teamRepo, leagueRepo)
	stabilityUC := intelligence.NewStabilityUsecase(matchRepo, teamRepo, leagueRepo)
	scoreUC := intelligence.NewScoreUsecase(matchRepo, teamRepo, leagueRepo, leagueStatsRepo, consistencyUC, trendUC)
	compatibilityUC := intelligence.NewCompatibilityUsecase(matchRepo, teamRepo, leagueRepo)
	opponentUC := intelligence.NewOpponentUsecase(teamRepo, leagueRepo, leagueStatsRepo)
	rankingUC := intelligence.NewRankingUsecase(leagueRepo, leagueStatsRepo)
	executiveUC := intelligence.NewExecutiveDashboardUsecase(leagueRepo, rankingUC, strategyHistoryRepo)
	alertUC := intelligence.NewAlertUsecase(alertRepo, teamRepo, leagueRepo, leagueStatsRepo, matchRepo)

	// Clientes de integração externa — cada chamada real fica registrada em usageRepo
	// (internal/usagelog), alimentando o painel de diagnóstico "Integrações".
	openaiClient := llm.NewOpenAIClient(cfg.OpenAIAPIKey, usageRepo)
	apiFootballClient := apifootball.New(cfg.APIFootballKey, usageRepo)
	sportMonksClient := sportmonks.New(cfg.SportMonksKey, usageRepo)

	explainUC := intelligence.NewExplainUsecase(openaiClient, consistencyUC, trendUC, stabilityUC, scoreUC, opponentUC)
	diagnosticsUC := diagnostics.New(
		usageRepo,
		openaiClient, cfg.OpenAIAPIKey != "",
		apiFootballClient, cfg.APIFootballKey != "",
		sportMonksClient, cfg.SportMonksKey != "",
	)

	// Handlers
	h := httpDelivery.Handlers{
		Auth:       handlers.NewAuthHandler(authUC),
		Catalog:    handlers.NewCatalogHandler(leagueRepo, teamRepo),
		Dashboard:  handlers.NewDashboardHandler(dashboardUC),
		Comparator: handlers.NewComparatorHandler(comparatorUC),
		Filter:     handlers.NewFilterHandler(filterUC, filterRepo, strategyHistoryUC, userRepo, cfg.JWTSecret),
		Bet:        handlers.NewBetHandler(betUC),
		Intelligence: handlers.NewIntelligenceHandler(
			consistencyUC, trendUC, stabilityUC, scoreUC, compatibilityUC, opponentUC,
			rankingUC, executiveUC, explainUC, redisClient, cfg.IntelligenceCacheTTL,
		),
		Alert:           handlers.NewAlertHandler(alertUC),
		StrategyHistory: handlers.NewStrategyHistoryHandler(strategyHistoryUC),
		Export:          handlers.NewExportHandler(dashboardUC, comparatorUC, filterUC, rankingUC),
		Diagnostics:     handlers.NewDiagnosticsHandler(diagnosticsUC),
		Bankroll:        handlers.NewBankrollHandler(bankrollUC),
		Billing:         handlers.NewBillingHandler(billingUC),
	}

	router := httpDelivery.NewRouter(h, cfg.JWTSecret, userRepo)

	appLog.Info("CornerLab API iniciando", "port", cfg.Port, "env", cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		appLog.Error("falha ao iniciar servidor", "error", err)
		os.Exit(1)
	}
}
