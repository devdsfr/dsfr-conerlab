// cmd/worker é o processo de background do Módulo de Sincronização de Dados
// (Statistics Provider). Roda separado da API HTTP (cmd/api) e mantém o Postgres
// continuamente sincronizado com o provedor de estatísticas configurado, sem que
// nenhuma requisição da API precise esperar por uma chamada externa: os handlers
// HTTP sempre leem só do Postgres (ver internal/usecase/dashboard.go e afins), então
// uma falha aqui nunca derruba o app — na pior hipótese, os dados só param de ficar
// tão frescos.
//
// Em produção roda como um Render Cron Job (barato, ~US$1/mês) com a variável
// SYNC_RUN_ONCE=true: cada disparo do cron sobe este binário, roda um ciclo completo
// e sai. Sem essa variável, o binário vira um loop infinito com tickers — pensado
// para rodar como Render Background Worker (a partir de US$7/mês) se um dia o
// produto precisar de dados quase em tempo real em vez de periódicos.
//
// Workers desta fase (núcleo do módulo — ver critério de aceite):
//  1. Descoberta       — a cada 30min, encontra jogos novos (AGENDADO), sem duplicar.
//  2. Atualização       — a cada 15min, finaliza jogos cuja data já passou.
//  3. Health Check       — a cada 1h, verifica a saúde do provedor (pedido explícito
//     do usuário, essencial por depender de uma API sem contrato de estabilidade).
//
// Os Workers de recálculo de estatísticas agregadas (last5/10/20 etc.) e de
// atualização de IA/rankings/alertas, além do Dashboard Administrativo, ficam para a
// fase 2 deste módulo (escopo combinado com o usuário).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
	"github.com/devdsfr/cornerlab/internal/integration/statsprovider/sofascore"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
	"github.com/devdsfr/cornerlab/internal/usagelog"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/analytics"
	"github.com/devdsfr/cornerlab/internal/usecase/statsync"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
	"github.com/devdsfr/cornerlab/pkg/config"
	"github.com/devdsfr/cornerlab/pkg/database"
	"github.com/devdsfr/cornerlab/pkg/logger"
)

const (
	discoveryInterval   = 30 * time.Minute
	updateInterval      = 15 * time.Minute
	healthCheckInterval = 1 * time.Hour
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	appLog := logger.New(cfg.Environment)
	slog.SetDefault(appLog)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		appLog.Error("falha ao conectar no postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	usageRepo := postgres.NewUsageRepo(pool)
	statSyncRepo := postgres.NewStatSyncRepo(pool)
	incidentRepo := postgres.NewProviderIncidentRepo(pool)
	syncRunRepo := postgres.NewSyncRunRepo(pool)

	provider, err := buildProvider(cfg.StatisticsProvider, cfg.APIFootballKey, usageRepo)
	if err != nil {
		appLog.Error("falha ao configurar provedor de estatísticas", "error", err)
		os.Exit(1)
	}

	discoveryUC := statsync.NewDiscoveryUsecase(provider, statSyncRepo, incidentRepo)
	updateUC := statsync.NewUpdateUsecase(provider, statSyncRepo, incidentRepo)
	healthUC := statsync.NewHealthCheckUsecase(provider, incidentRepo)

	// Analytics Worker (Remodelagem F3, doc 15): pré-calcula team_metrics após
	// cada ciclo de sincronização — "o usuário consulta, workers calculam".
	analyticsRepo := postgres.NewAnalyticsRepo(pool)
	matchRepo := postgres.NewMatchRepo(pool)
	teamRepo := postgres.NewTeamRepo(pool)
	analyticsWorker := analytics.NewWorker(matchRepo, teamRepo, analyticsRepo)

	// Strategy Worker (Remodelagem F4, Workers 04/05/07 do doc 15): reexecuta o
	// backtest de todas as estratégias ativas e recalcula health + scores.
	strategyRepo := postgres.NewStrategyRepo(pool)
	filterUC := usecase.NewFilterUsecase(matchRepo, teamRepo, postgres.NewLeagueRepo(pool))
	strategyEngine := strategyengine.NewEngine(filterUC, strategyRepo)

	appLog.Info("CornerLab worker de sincronização iniciando", "provider", provider.Name(),
		"discovery_interval", discoveryInterval.String(), "update_interval", updateInterval.String(),
		"health_check_interval", healthCheckInterval.String())

	// Health check roda uma vez antes de tudo, para já saber se o provedor está
	// saudável antes do primeiro ciclo de descoberta/atualização.
	runHealthCheck(ctx, healthUC)
	cycleStart := time.Now()
	discoveryResult := runDiscovery(ctx, discoveryUC)
	updateResult := runUpdate(ctx, updateUC)
	recordRun(ctx, syncRunRepo, discoveryResult, updateResult, time.Since(cycleStart).Milliseconds())
	runAnalytics(ctx, analyticsWorker, analyticsRepo)
	runStrategies(ctx, strategyEngine, analyticsRepo)

	// SYNC_RUN_ONCE=true faz este mesmo binário rodar um único ciclo e sair — é o
	// "Command" usado pelo Render Cron Job (barato, roda periodicamente em vez de um
	// processo 24h). Sem essa variável, comportamento original: loop infinito com
	// tickers, pensado para rodar como Render Background Worker (mais caro, dados
	// quase em tempo real) caso o produto precise disso no futuro.
	if os.Getenv("SYNC_RUN_ONCE") == "true" {
		appLog.Info("worker de sincronização: execução única concluída (SYNC_RUN_ONCE=true)")
		return
	}

	discoveryTicker := time.NewTicker(discoveryInterval)
	updateTicker := time.NewTicker(updateInterval)
	healthTicker := time.NewTicker(healthCheckInterval)
	defer discoveryTicker.Stop()
	defer updateTicker.Stop()
	defer healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			appLog.Info("worker de sincronização encerrando")
			return
		case <-discoveryTicker.C:
			runDiscovery(ctx, discoveryUC)
		case <-updateTicker.C:
			runUpdate(ctx, updateUC)
			runAnalytics(ctx, analyticsWorker, analyticsRepo)
			runStrategies(ctx, strategyEngine, analyticsRepo)
		case <-healthTicker.C:
			runHealthCheck(ctx, healthUC)
		}
	}
}

// runDiscovery, runUpdate e runHealthCheck sempre recuperam de panic — uma falha
// inesperada em um ciclo nunca deve derrubar o processo inteiro (regra do critério de
// aceite: "falhas do provider não podem quebrar a aplicação"). Os resultados voltam
// (zero-value em caso de panic/erro) para alimentar recordRun.
func runDiscovery(ctx context.Context, uc *statsync.DiscoveryUsecase) (result statsync.DiscoveryResult) {
	defer recoverAndLog("descoberta")
	start := time.Now()
	var err error
	result, err = uc.Run(ctx)
	fields := []any{
		"duration_ms", time.Since(start).Milliseconds(),
		"targets", result.Targets, "fixtures_found", result.FixturesFound,
		"fixtures_upserted", result.FixturesUpserted, "errors", result.Errors,
	}
	if err != nil {
		slog.Error("ciclo de descoberta falhou", append(fields, "error", err)...)
		return
	}
	slog.Info("ciclo de descoberta concluído", fields...)
	return
}

func runUpdate(ctx context.Context, uc *statsync.UpdateUsecase) (result statsync.UpdateResult) {
	defer recoverAndLog("atualização")
	start := time.Now()
	var err error
	result, err = uc.Run(ctx)
	fields := []any{
		"duration_ms", time.Since(start).Milliseconds(),
		"checked", result.Checked, "finalized", result.Finalized,
		"still_open", result.StillOpen, "errors", result.Errors,
	}
	if err != nil {
		slog.Error("ciclo de atualização falhou", append(fields, "error", err)...)
		return
	}
	slog.Info("ciclo de atualização concluído", fields...)
	return
}

// recordRun grava o histórico desta execução (ver domain.SyncRun) para o painel
// Integrações mostrar "Última sincronização: ...". Nunca derruba o worker por causa
// de uma falha ao salvar — a sincronização em si já rodou.
func recordRun(ctx context.Context, repo *postgres.SyncRunRepo, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64) {
	entry := &domain.SyncRun{
		TriggeredBy:      "cron",
		Targets:          d.Targets,
		FixturesFound:    d.FixturesFound,
		FixturesUpserted: d.FixturesUpserted,
		MatchesChecked:   u.Checked,
		MatchesFinalized: u.Finalized,
		Errors:           d.Errors + u.Errors,
		DurationMs:       durationMs,
	}
	if err := repo.AddRun(ctx, entry); err != nil {
		slog.Error("falha ao registrar histórico de sincronização", "error", err)
	}
}

// runAnalytics roda o Analytics Worker (Remodelagem F3) com a mesma resiliência
// dos demais ciclos (recover + log) e observabilidade em worker_runs (doc 15:
// cada worker registra tempo, quantidade processada e erros).
func runAnalytics(ctx context.Context, w *analytics.Worker, repo *postgres.AnalyticsRepo) {
	defer recoverAndLog("analytics")
	start := time.Now()
	runID, idErr := repo.StartWorkerRun(ctx, "analytics")
	result, err := w.Run(ctx)
	fields := []any{
		"duration_ms", time.Since(start).Milliseconds(),
		"league_seasons", result.LeagueSeasons, "teams", result.TeamsSeen,
		"metrics_saved", result.MetricsSaved, "errors", result.Errors,
	}
	status := "ok"
	if err != nil {
		status = "error"
		slog.Error("ciclo de analytics falhou", append(fields, "error", err)...)
	} else {
		slog.Info("ciclo de analytics concluído", fields...)
	}
	if idErr == nil {
		details := map[string]any{
			"league_seasons": result.LeagueSeasons,
			"teams":          result.TeamsSeen,
			"metrics_saved":  result.MetricsSaved,
		}
		if err := repo.FinishWorkerRun(ctx, runID, status, result.MetricsSaved, result.Errors, start, details); err != nil {
			slog.Error("falha ao registrar worker_run de analytics", "error", err)
		}
	}
}

// runStrategies roda o Strategy Worker (Remodelagem F4): backtest + health +
// scores de todas as estratégias ativas, com observabilidade em worker_runs.
func runStrategies(ctx context.Context, e *strategyengine.Engine, repo *postgres.AnalyticsRepo) {
	defer recoverAndLog("strategy engine")
	start := time.Now()
	runID, idErr := repo.StartWorkerRun(ctx, "strategy")
	result, err := e.RunAll(ctx)
	fields := []any{
		"duration_ms", time.Since(start).Milliseconds(),
		"strategies", result.Strategies, "evaluated", result.Evaluated, "errors", result.Errors,
	}
	status := "ok"
	if err != nil {
		status = "error"
		slog.Error("ciclo do strategy engine falhou", append(fields, "error", err)...)
	} else {
		slog.Info("ciclo do strategy engine concluído", fields...)
	}
	if idErr == nil {
		details := map[string]any{"strategies": result.Strategies, "evaluated": result.Evaluated}
		if err := repo.FinishWorkerRun(ctx, runID, status, result.Evaluated, result.Errors, start, details); err != nil {
			slog.Error("falha ao registrar worker_run do strategy engine", "error", err)
		}
	}
}

func runHealthCheck(ctx context.Context, uc *statsync.HealthCheckUsecase) {
	defer recoverAndLog("health check")
	start := time.Now()
	result, err := uc.Run(ctx)
	fields := []any{"duration_ms", time.Since(start).Milliseconds(), "ok", result.OK}
	if err != nil {
		slog.Error("ciclo de health check falhou", append(fields, "error", err)...)
		return
	}
	slog.Info("ciclo de health check concluído", fields...)
}

func recoverAndLog(cycle string) {
	if r := recover(); r != nil {
		slog.Error("panic recuperado em ciclo do worker — processo continua rodando", "cycle", cycle, "panic", r)
	}
}

// buildProvider escolhe a implementação de statsprovider.StatisticsProvider conforme
// STATISTICS_PROVIDER. "sofascore" já é aceito aqui (a interface está pronta), mas
// hoje devolve sempre ErrNotImplemented — ver comentário de pacote em
// internal/integration/statsprovider/sofascore/client.go sobre o motivo.
func buildProvider(name, apiFootballKey string, recorder usagelog.Recorder) (statsprovider.StatisticsProvider, error) {
	switch name {
	case "sofascore":
		return sofascore.New(), nil
	case "api_football", "":
		return apifootball.New(apiFootballKey, recorder), nil
	default:
		return apifootball.New(apiFootballKey, recorder), nil
	}
}
