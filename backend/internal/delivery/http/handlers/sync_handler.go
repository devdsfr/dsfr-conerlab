package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/progress"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase/statsync"
)

// SyncHandler expõe o botão "Sincronizar agora" do painel Integrações: dispara
// manualmente um ciclo de descoberta + atualização (os mesmos usecases que o Render
// Cron Job roda periodicamente), para o caso de o usuário notar que os dados estão
// desatualizados e não querer esperar o próximo ciclo agendado. Run fica atrás de
// autenticação (cada clique gera chamadas reais à API externa, sujeitas a cota) —
// Status é público, é só leitura do histórico já registrado.
type SyncHandler struct {
	discovery *statsync.DiscoveryUsecase
	update    *statsync.UpdateUsecase
	runs      repository.SyncRunRepository
	progress  *progress.Tracker
}

func NewSyncHandler(discovery *statsync.DiscoveryUsecase, update *statsync.UpdateUsecase, runs repository.SyncRunRepository) *SyncHandler {
	return &SyncHandler{discovery: discovery, update: update, runs: runs, progress: progress.NewTracker()}
}

// syncTimeout limita o ciclo disparado em segundo plano, para não deixar uma
// goroutine presa caso o provedor pendure a conexão.
//
// ERA 20 MINUTOS E ISSO CORTOU UM CICLO PELA METADE. A estimativa original ("um
// ciclo cheio leva perto de 7 minutos") só valia para o caso rotineiro: poucas
// partidas novas, 50 atualizações. Em 12/09, depois de seis semanas sem
// sincronizar, a descoberta trouxe 3.712 partidas de uma vez — e cada partida
// custa várias idas ao banco (duas equipes, dois vínculos liga-equipe, a
// partida). O ciclo bateu exatamente em "20min 0s" e morreu com
// "context deadline exceeded", tendo gravado a descoberta mas sem completar a
// atualização.
//
// O valor virou configurável porque o custo de um ciclo depende de quanto
// atraso existe para recuperar — algo que nenhuma constante consegue prever. O
// padrão de 60min cobre com folga a recuperação de um acúmulo grande; em
// operação normal o ciclo termina em poucos minutos e o teto nunca é atingido.
func syncTimeout() time.Duration {
	if syncTimeoutMinutes > 0 {
		return time.Duration(syncTimeoutMinutes) * time.Minute
	}
	return 60 * time.Minute
}

// syncTimeoutMinutes vem de SYNC_TIMEOUT_MINUTES (ver pkg/config). Fica como
// variável de pacote porque o handler é construído em vários lugares e passar
// isso por parâmetro em todos só para um timeout não pagaria.
var syncTimeoutMinutes int

// SetSyncTimeoutMinutes é chamada na inicialização, a partir da configuração.
func SetSyncTimeoutMinutes(m int) { syncTimeoutMinutes = m }

type syncRunResponse struct {
	Discovery  statsync.DiscoveryResult `json:"discovery"`
	Update     statsync.UpdateResult    `json:"update"`
	DurationMs int64                    `json:"duration_ms"`
}

// Run godoc
// @Summary Disparar manualmente um ciclo de sincronização (descoberta + atualização)
// @Tags sync
// @Router /api/v1/sync/run [post]
// O ciclo roda em SEGUNDO PLANO e a resposta volta na hora (202 Accepted). Antes
// isso era síncrono, o que virou um problema quando o cliente da API-Football ganhou
// throttle para não tomar 429: o ciclo passou de ~13s para ~6min, tempo suficiente
// para proxy ou navegador cortarem a conexão, e o usuário ficava olhando um spinner
// sem saber quanto faltava. Agora o acompanhamento é por GET /sync/progress.
func (h *SyncHandler) Run(c *gin.Context) {
	if !h.progress.Start("Preparando…") {
		// 409: já existe um ciclo rodando. Impede que dois cliques seguidos
		// dobrem o consumo da cota da API externa.
		c.JSON(http.StatusConflict, gin.H{
			"error":    "já existe uma sincronização em andamento",
			"progress": h.progress.Snapshot(),
		})
		return
	}

	// Contexto próprio: o contexto da requisição morre assim que respondemos,
	// e o trabalho precisa sobreviver a isso.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), syncTimeout())
		defer cancel()
		start := time.Now()

		// O ciclo que FALHA também precisa deixar registro. Antes os dois returns
		// abaixo saíam sem gravar nada, e o efeito prático foi observado em
		// produção: o ciclo de 12/09 morreu em "context deadline exceeded" e a
		// tela continuou anunciando a sincronização do dia anterior como a mais
		// recente — o sistema parecia mais saudável por ter falhado.
		//
		// Contexto próprio para gravar: se o ciclo morreu porque ctx expirou,
		// usar o mesmo ctx no INSERT faria o registro da falha falhar também,
		// que é justamente o buraco que se está fechando.
		var discoveryResult statsync.DiscoveryResult
		var updateResult statsync.UpdateResult

		registrarFalha := func(fase string, err error) {
			slog.Error(fase+" falhou no ciclo manual", "error", err)
			ctxReg, cancelReg := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelReg()
			h.recordFailedRun(ctxReg, TriggerManual, discoveryResult, updateResult,
				time.Since(start).Milliseconds(), fmt.Sprintf("%s falhou: %v", fase, err))
			h.progress.Finish(fmt.Errorf("%s falhou: %w", fase, err), nil)
		}

		discoveryResult, err := h.discovery.WithProgress(h.progress).Run(ctx)
		if err != nil {
			registrarFalha("descoberta", err)
			return
		}

		updateResult, err = h.update.WithProgress(h.progress).Run(ctx)
		if err != nil {
			registrarFalha("atualização", err)
			return
		}

		durationMs := time.Since(start).Milliseconds()
		h.recordRun(ctx, TriggerManual, discoveryResult, updateResult, durationMs)
		h.progress.Finish(nil, statsync.SyncOutcome{Discovery: discoveryResult, Update: updateResult})
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"started":  true,
		"message":  "sincronização iniciada — acompanhe em /sync/progress",
		"progress": h.progress.Snapshot(),
	})
}

// Progress godoc
// @Summary Andamento do ciclo de sincronização em execução (para a barra de progresso)
// @Tags sync
// @Router /api/v1/sync/progress [get]
func (h *SyncHandler) Progress(c *gin.Context) {
	c.JSON(http.StatusOK, h.progress.Snapshot())
}

// Status godoc
// @Summary Última sincronização registrada (manual ou via Cron Job) — para o painel
// mostrar "Última sincronização: ..." sem depender de estado local do navegador.
// @Tags sync
// @Router /api/v1/sync/status [get]
func (h *SyncHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()

	last, err := h.runs.LastRun(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// A partir daqui é o que alimenta o aviso de dado desatualizado na interface.
	// Nenhuma dessas consultas pode derrubar o endpoint: se falharem, a tela
	// mostra "última sincronização" como sempre mostrou e o aviso simplesmente
	// não aparece.
	lastOK, _ := h.runs.LastSuccessfulRun(ctx)
	errMsg, errAt, _ := h.runs.LastProviderError(ctx)

	// Separar por origem responde uma pergunta que "último ciclo" não responde:
	// a execução AUTOMÁTICA está viva? Enquanto a tela mostrava só o ciclo mais
	// recente, qualquer clique em "Sincronizar agora" deixava a linha com cara de
	// saudável — e o Cron Job simplesmente não existia no Render.
	lastCron, _ := h.runs.LastRunBySource(ctx, TriggerCron)
	lastManual, _ := h.runs.LastRunBySource(ctx, TriggerManual)

	resp := gin.H{
		"last_run":            last,
		"last_successful_run": lastOK,
		"last_cron_run":       lastCron,
		"last_manual_run":     lastManual,
	}

	if lastCron != nil {
		horas := time.Since(lastCron.CreatedAt).Hours()
		resp["hours_since_cron"] = int(horas)
		// O ciclo automático roda uma vez por dia. Passar de cronStaleAfterHours
		// significa que pelo menos um disparo foi perdido.
		resp["cron_stale"] = horas >= cronStaleAfterHours
	} else {
		// NUNCA rodou. Isto é diferente de "rodou e está atrasado", e a tela
		// precisa dizer as duas coisas de forma diferente: a primeira quase sempre
		// significa que o Cron Job não existe ou está mal configurado, e nenhuma
		// quantidade de espera resolve.
		resp["hours_since_cron"] = nil
		resp["cron_stale"] = true
		resp["cron_never_ran"] = true
	}

	if lastOK != nil {
		horas := time.Since(lastOK.CreatedAt).Hours()
		resp["hours_since_success"] = int(horas)
		resp["stale"] = horas >= staleAfterHours
	} else {
		// Nunca houve um ciclo bem-sucedido registrado: tratar como desatualizado
		// é o padrão seguro — o contrário afirmaria saúde sem evidência.
		resp["hours_since_success"] = nil
		resp["stale"] = true
	}

	// Só reporta o erro do provedor se ele for RECENTE. Um erro de duas semanas
	// atrás, já resolvido, não deve aparecer como causa do problema de hoje.
	if errMsg != "" && time.Since(errAt) < 7*24*time.Hour {
		resp["provider_error"] = errMsg
		resp["provider_error_at"] = errAt
	}

	c.JSON(http.StatusOK, resp)
}

// staleAfterHours é o tempo sem uma sincronização BEM-SUCEDIDA a partir do qual
// a interface avisa. 48h cobre o caso normal (o ciclo roda uma vez por dia, às
// 08:00) com uma folga de um dia — assim uma falha isolada não gera alarme, mas
// duas seguidas geram.
const staleAfterHours = 48

// cronStaleAfterHours é o limite para o ciclo AUTOMÁTICO especificamente. Mais
// curto que staleAfterHours de propósito: o cron roda todo dia às 08:00, então
// 36h já significa que um disparo inteiro foi perdido — informação que se perde
// se a única referência for "algum ciclo rodou nas últimas 48h", já que um
// clique manual satisfaz esse critério sem que o automático tenha voltado.
const cronStaleAfterHours = 36

// Origens registradas em sync_runs.triggered_by — ver domain/sync.go, que é
// onde os valores vivem, já que cmd/worker também os grava.
const (
	TriggerCron   = domain.SyncTriggerCron
	TriggerManual = domain.SyncTriggerManual
)

// recordRun nunca falha a requisição por causa de um erro ao salvar o histórico —
// a sincronização em si já rodou com sucesso, perder o registro não pode virar 500.
func (h *SyncHandler) recordRun(ctx context.Context, triggeredBy string, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64) {
	h.saveRun(ctx, triggeredBy, d, u, durationMs, domain.SyncStatusSuccess, "")
}

// recordFailedRun grava o ciclo INTERROMPIDO, com o que ele alcançou antes de
// morrer. Os números parciais importam: "falhou depois de gravar 3.712 partidas"
// e "falhou sem fazer nada" apontam para causas diferentes.
func (h *SyncHandler) recordFailedRun(ctx context.Context, triggeredBy string, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64, motivo string) {
	h.saveRun(ctx, triggeredBy, d, u, durationMs, domain.SyncStatusFailed, motivo)
}

func (h *SyncHandler) saveRun(ctx context.Context, triggeredBy string, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64, status, motivo string) {
	entry := &domain.SyncRun{
		TriggeredBy:      triggeredBy,
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
	if err := h.runs.AddRun(ctx, entry); err != nil {
		slog.Error("falha ao registrar histórico de sincronização", "error", err, "status", status)
	}
}
