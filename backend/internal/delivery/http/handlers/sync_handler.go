package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/domain"
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
}

func NewSyncHandler(discovery *statsync.DiscoveryUsecase, update *statsync.UpdateUsecase, runs repository.SyncRunRepository) *SyncHandler {
	return &SyncHandler{discovery: discovery, update: update, runs: runs}
}

type syncRunResponse struct {
	Discovery statsync.DiscoveryResult `json:"discovery"`
	Update    statsync.UpdateResult    `json:"update"`
}

// Run godoc
// @Summary Disparar manualmente um ciclo de sincronização (descoberta + atualização)
// @Tags sync
// @Router /api/v1/sync/run [post]
func (h *SyncHandler) Run(c *gin.Context) {
	ctx := c.Request.Context()

	discoveryResult, err := h.discovery.Run(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "descoberta falhou: " + err.Error()})
		return
	}
	updateResult, err := h.update.Run(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "atualização falhou: " + err.Error(),
			"discovery": discoveryResult,
		})
		return
	}

	h.recordRun(ctx, "manual", discoveryResult, updateResult)
	c.JSON(http.StatusOK, syncRunResponse{Discovery: discoveryResult, Update: updateResult})
}

// Status godoc
// @Summary Última sincronização registrada (manual ou via Cron Job) — para o painel
// mostrar "Última sincronização: ..." sem depender de estado local do navegador.
// @Tags sync
// @Router /api/v1/sync/status [get]
func (h *SyncHandler) Status(c *gin.Context) {
	last, err := h.runs.LastRun(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"last_run": last})
}

// recordRun nunca falha a requisição por causa de um erro ao salvar o histórico —
// a sincronização em si já rodou com sucesso, perder o registro não pode virar 500.
func (h *SyncHandler) recordRun(ctx context.Context, triggeredBy string, d statsync.DiscoveryResult, u statsync.UpdateResult) {
	entry := &domain.SyncRun{
		TriggeredBy:      triggeredBy,
		Targets:          d.Targets,
		FixturesFound:    d.FixturesFound,
		FixturesUpserted: d.FixturesUpserted,
		MatchesChecked:   u.Checked,
		MatchesFinalized: u.Finalized,
		Errors:           d.Errors + u.Errors,
	}
	if err := h.runs.AddRun(ctx, entry); err != nil {
		slog.Error("falha ao registrar histórico de sincronização", "error", err)
	}
}
