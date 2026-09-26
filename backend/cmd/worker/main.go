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
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	cornerlab "github.com/devdsfr/cornerlab"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
	"github.com/devdsfr/cornerlab/internal/integration/statsprovider/sofascore"
	"github.com/devdsfr/cornerlab/internal/migrate"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
	"github.com/devdsfr/cornerlab/internal/usagelog"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/analytics"
	"github.com/devdsfr/cornerlab/internal/usecase/discovery"
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

	// strategyDiscoveryInterval seguem o doc 08 ("Atualização: todos os dias"):
	// minerar combinações é caro e o resultado só muda quando entram jogos novos,
	// então não faz sentido rodar junto dos ciclos curtos de sincronização.
	strategyDiscoveryInterval = 24 * time.Hour
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	appLog := logger.New(cfg.Environment)
	slog.SetDefault(appLog)

	// Falhar aqui, dizendo QUAL variável falta, em vez de falhar três passos
	// adiante com "connection refused em 127.0.0.1" — ver config.Validate.
	if err := cfg.Validate(); err != nil {
		appLog.Error("worker não vai rodar", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		appLog.Error("falha ao conectar no postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Mesmo contrato da API: o schema é acertado antes de qualquer trabalho.
	// O worker costuma subir junto num deploy, então qualquer um dos dois pode
	// ser o primeiro a aplicar — o advisory lock resolve a corrida.
	if err := migrate.Run(ctx, pool, cornerlab.MigrationsFS, appLog); err != nil {
		appLog.Error("falha ao aplicar migrations — o worker não vai rodar", "error", err)
		os.Exit(1)
	}

	usageRepo := postgres.NewUsageRepo(pool)
	statSyncRepo := postgres.NewStatSyncRepo(pool)
	incidentRepo := postgres.NewProviderIncidentRepo(pool)
	syncRunRepo := postgres.NewSyncRunRepo(pool)

	provider, err := buildProvider(cfg.StatisticsProvider, cfg.APIFootballKey,
		cfg.APIFootballRateLimitPerMin, usageRepo)
	if err != nil {
		appLog.Error("falha ao configurar provedor de estatísticas", "error", err)
		os.Exit(1)
	}

	discoveryUC := statsync.NewDiscoveryUsecase(provider, statSyncRepo, incidentRepo)
	// O teto por ciclo acompanha a frequência do disparo: com um ciclo por dia,
	// 50 partidas não dão conta de um fim de semana (ver SYNC_MAX_PER_CYCLE).
	updateUC := statsync.NewUpdateUsecase(provider, statSyncRepo, incidentRepo).
		WithMaxPerCycle(cfg.SyncMaxPerCycle)
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
	leagueRepo := postgres.NewLeagueRepo(pool)
	filterUC := usecase.NewFilterUsecase(matchRepo, teamRepo, leagueRepo)
	strategyEngine := strategyengine.NewEngine(filterUC, strategyRepo)

	// Strategy Discovery Engine (Remodelagem F6, doc 08): minera combinações de
	// filtros, valida contra os critérios mínimos e publica as sobreviventes como
	// estratégias do sistema. IncludeTeams ligado aqui (e desligado na API): o ciclo
	// noturno pode pagar o custo da varredura por equipe, uma requisição HTTP não.
	discoveryEngine := discovery.NewEngine(
		matchRepo, teamRepo, leagueRepo, strategyRepo, strategyEngine,
		discovery.Options{Criteria: discovery.DefaultCriteria(), IncludeTeams: true},
	)

	appLog.Info("CornerLab worker de sincronização iniciando", "provider", provider.Name(),
		"discovery_interval", discoveryInterval.String(), "update_interval", updateInterval.String(),
		"health_check_interval", healthCheckInterval.String())

	// Health check roda uma vez antes de tudo, para já saber se o provedor está
	// saudável antes do primeiro ciclo de descoberta/atualização.
	runHealthCheck(ctx, healthUC)
	cycleStart := time.Now()
	discoveryResult, errDiscovery := runDiscovery(ctx, discoveryUC)
	updateResult, errUpdate := runUpdate(ctx, updateUC)
	// O ciclo é registrado SEMPRE, e agora com a distinção entre "completou" e
	// "foi interrompido". Antes as duas funções engoliam o erro e devolviam
	// zero-value, então uma falha virava uma linha de aparência normal com
	// números zerados — indistinguível de um dia em que não havia nada a fazer.
	recordRun(ctx, syncRunRepo, discoveryResult, updateResult,
		time.Since(cycleStart).Milliseconds(), motivoDaFalha(errDiscovery, errUpdate))
	runAnalytics(ctx, analyticsWorker, analyticsRepo)
	runStrategies(ctx, strategyEngine, analyticsRepo)

	// No modo cron (execução única) a descoberta só roda quando explicitamente
	// pedida por DISCOVERY_RUN=true, porque o cron de sincronização dispara a cada
	// poucos minutos e a varredura completa é diária: a intenção é ter um segundo
	// Cron Job, agendado uma vez por dia, apontando para este mesmo binário.
	// No modo loop ela roda no boot e depois no ticker de 24h.
	runOnce := os.Getenv("SYNC_RUN_ONCE") == "true"
	if !runOnce || os.Getenv("DISCOVERY_RUN") == "true" {
		runStrategyDiscovery(ctx, discoveryEngine, analyticsRepo)
	}

	// Manutenção: api_usage_log é a única tabela que cresce sem teto (uma linha por
	// chamada externa). Roda no fim do ciclo, quando nada mais depende dela.
	purgeUsageLog(ctx, usageRepo)

	// SYNC_RUN_ONCE=true faz este mesmo binário rodar um único ciclo e sair — é o
	// "Command" usado pelo Render Cron Job (barato, roda periodicamente em vez de um
	// processo 24h). Sem essa variável, comportamento original: loop infinito com
	// tickers, pensado para rodar como Render Background Worker (mais caro, dados
	// quase em tempo real) caso o produto precise disso no futuro.
	if runOnce {
		appLog.Info("worker de sincronização: execução única concluída (SYNC_RUN_ONCE=true)")
		return
	}

	discoveryTicker := time.NewTicker(discoveryInterval)
	updateTicker := time.NewTicker(updateInterval)
	healthTicker := time.NewTicker(healthCheckInterval)
	strategyDiscoveryTicker := time.NewTicker(strategyDiscoveryInterval)
	defer discoveryTicker.Stop()
	defer updateTicker.Stop()
	defer healthTicker.Stop()
	defer strategyDiscoveryTicker.Stop()

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
		case <-strategyDiscoveryTicker.C:
			runStrategyDiscovery(ctx, discoveryEngine, analyticsRepo)
			purgeUsageLog(ctx, usageRepo)
		}
	}
}

// purgeUsageLog apaga o histórico de chamadas mais antigo que a janela de retenção
// (ver postgres.UsageLogRetentionDays). Nunca derruba o ciclo: falhar em limpar log
// é irrelevante perto de falhar em sincronizar dados, então o erro só é registrado.
func purgeUsageLog(ctx context.Context, repo *postgres.UsageRepo) {
	defer recoverAndLog("limpeza do histórico de uso")

	removidos, err := repo.PurgeOldUsage(ctx)
	if err != nil {
		slog.Warn("nao foi possivel limpar o historico de uso", "error", err)
		return
	}
	if removidos > 0 {
		slog.Info("historico de uso limpo", "registros_removidos", removidos,
			"retencao_dias", postgres.UsageLogRetentionDays)
	}
}

// runDiscovery, runUpdate e runHealthCheck sempre recuperam de panic — uma falha
// inesperada em um ciclo nunca deve derrubar o processo inteiro (regra do critério de
// aceite: "falhas do provider não podem quebrar a aplicação"). Os resultados voltam
// (zero-value em caso de panic/erro) para alimentar recordRun.
//
// O ERRO TAMBÉM VOLTA, e não só para o log. Enquanto ele era apenas logado, o
// registro em sync_runs marcava como bem-sucedido um ciclo que tinha morrido: a
// linha ficava com números zerados, indistinguível de um dia sem nada a fazer.
// A tela de Integrações lê sync_runs, não os logs do Render — então a falha era
// invisível exatamente para quem precisava vê-la.
func runDiscovery(ctx context.Context, uc *statsync.DiscoveryUsecase) (result statsync.DiscoveryResult, err error) {
	defer recoverAndLog("descoberta")
	start := time.Now()
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

func runUpdate(ctx context.Context, uc *statsync.UpdateUsecase) (result statsync.UpdateResult, err error) {
	defer recoverAndLog("atualização")
	start := time.Now()
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

// motivoDaFalha monta a mensagem gravada em sync_runs.error_message. Devolve
// string vazia quando as duas fases terminaram — é esse vazio que faz recordRun
// marcar o ciclo como bem-sucedido.
//
// As duas fases são reportadas juntas quando ambas falham, porque saber que a
// descoberta E a atualização caíram aponta para o provedor ou para a rede, não
// para um defeito específico de uma delas.
func motivoDaFalha(errDiscovery, errUpdate error) string {
	var partes []string
	if errDiscovery != nil {
		partes = append(partes, "descoberta: "+errDiscovery.Error())
	}
	if errUpdate != nil {
		partes = append(partes, "atualização: "+errUpdate.Error())
	}
	return strings.Join(partes, " | ")
}

// recordRun grava o histórico desta execução (ver domain.SyncRun) para o painel
// Integrações mostrar "Última sincronização: ...". Nunca derruba o worker por causa
// de uma falha ao salvar — o ciclo em si já aconteceu, e perder o registro não
// pode virar um segundo problema.
//
// motivo vazio = ciclo completou; motivo preenchido = ciclo interrompido, e o
// texto vai para sync_runs.error_message.
func recordRun(ctx context.Context, repo *postgres.SyncRunRepo, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64, motivo string) {
	status := domain.SyncStatusSuccess
	if motivo != "" {
		status = domain.SyncStatusFailed
	}

	entry := &domain.SyncRun{
		TriggeredBy:      domain.SyncTriggerCron,
		Targets:          d.Targets,
		FixturesFound:    d.FixturesFound,
		FixturesUpserted: d.FixturesUpserted,
		MatchesChecked:   u.Checked,
		MatchesFinalized: u.Finalized,
		Errors:           d.Errors + u.Errors,
		DurationMs:       durationMs,
		Status:           status,
		ErrorMessage:     motivo,
	}

	// Contexto próprio: quando o ciclo cai porque ctx expirou, gravar com o mesmo
	// ctx faria o registro da falha falhar junto — perdendo exatamente a
	// informação que interessa.
	ctxReg, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := repo.AddRun(ctxReg, entry); err != nil {
		slog.Error("falha ao registrar histórico de sincronização", "error", err, "status", status)
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

// runStrategyDiscovery roda o Discovery Worker (Remodelagem F6, doc 08) com a
// mesma resiliência e observabilidade dos demais ciclos. É o worker mais caro do
// pipeline (centenas de backtests por liga), por isso o tempo e a contagem de
// combinações avaliadas ficam registrados em worker_runs.
func runStrategyDiscovery(ctx context.Context, e *discovery.Engine, repo *postgres.AnalyticsRepo) {
	defer recoverAndLog("strategy discovery")
	start := time.Now()
	runID, idErr := repo.StartWorkerRun(ctx, "discovery")
	result, err := e.RunAll(ctx)
	fields := []any{
		"duration_ms", time.Since(start).Milliseconds(),
		"leagues", result.Leagues, "combinations", result.Combinations,
		"published", result.Published, "deactivated", result.Deactivated,
		"errors", result.Errors,
	}
	// REV-P4 (item 27): status e details vêm das MESMAS funções usadas pela
	// execução manual — um formato só em worker_runs, lido pela tela. Ciclo
	// interrompido é "error", nunca "ok".
	status := discovery.RunStatus(result, err)
	if status != "ok" {
		slog.Error("ciclo de descoberta de estratégias falhou ou foi interrompido", append(fields, "error", err)...)
	} else {
		slog.Info("ciclo de descoberta de estratégias concluído", fields...)
	}
	if idErr == nil {
		details := result.RunDetails(discovery.TriggerCron)
		if err := repo.FinishWorkerRun(ctx, runID, status, result.Published, result.Errors, start, details); err != nil {
			slog.Error("falha ao registrar worker_run de descoberta", "error", err)
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
//
// ratePerMin é o teto de requisições por minuto do plano CONTRATADO na
// API-Football — sem ele o cliente anda no ritmo do plano gratuito (ver
// apifootball.WithRateLimitPerMinute).
func buildProvider(name, apiFootballKey string, ratePerMin int, recorder usagelog.Recorder) (statsprovider.StatisticsProvider, error) {
	switch name {
	case "sofascore":
		return sofascore.New(), nil
	default:
		return apifootball.New(apiFootballKey, recorder,
			apifootball.WithRateLimitPerMinute(ratePerMin)), nil
	}
}
