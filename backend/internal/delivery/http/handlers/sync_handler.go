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

// syncTimeout limita o ciclo disparado em segundo plano. Com o throttle da
// API-Football (~9 req/min) um ciclo cheio — até 50 partidas mais os campeonatos —
// leva perto de 7 minutos; 20 minutos dá folga larga sem deixar uma goroutine presa
// para sempre caso o provedor pendure a conexão.
const syncTimeout = 20 * time.Minute

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
		ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
		defer cancel()
		start := time.Now()

		discoveryResult, err := h.discovery.WithProgress(h.progress).Run(ctx)
		if err != nil {
			slog.Error("descoberta falhou no ciclo manual", "error", err)
			h.progress.Finish(fmt.Errorf("descoberta falhou: %w", err), nil)
			return
		}

		updateResult, err := h.update.WithProgress(h.progress).Run(ctx)
		if err != nil {
			slog.Error("atualização falhou no ciclo manual", "error", err)
			h.progress.Finish(fmt.Errorf("atualização falhou: %w", err), nil)
			return
		}

		durationMs := time.Since(start).Milliseconds()
		h.recordRun(ctx, "manual", discoveryResult, updateResult, durationMs)
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

	resp := gin.H{"last_run": last, "last_successful_run": lastOK}

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

// recordRun nunca falha a requisição por causa de um erro ao salvar o histórico —
// a sincronização em si já rodou com sucesso, perder o registro não pode virar 500.
func (h *SyncHandler) recordRun(ctx context.Context, triggeredBy string, d statsync.DiscoveryResult, u statsync.UpdateResult, durationMs int64) {
	entry := &domain.SyncRun{
		TriggeredBy:      triggeredBy,
		Targets:          d.Targets,
		FixturesFound:    d.FixturesFound,
		FixturesUpserted: d.FixturesUpserted,
		MatchesChecked:   u.Checked,
		MatchesFinalized: u.Finalized,
		Errors:           d.Errors + u.Errors,
		DurationMs:       durationMs,
	}
	if err := h.runs.AddRun(ctx, entry); err != nil {
		slog.Error("falha ao registrar histórico de sincronização", "error", err)
	}
}
