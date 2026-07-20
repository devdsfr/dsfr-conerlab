package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/usecase/statsync"
)

// SyncHandler expõe o botão "Sincronizar agora" do painel Integrações: dispara
// manualmente um ciclo de descoberta + atualização (os mesmos usecases que o Render
// Cron Job roda periodicamente), para o caso de o usuário notar que os dados estão
// desatualizados e não querer esperar o próximo ciclo agendado. Fica atrás de
// autenticação (não é exposto publicamente como o resto do painel de diagnóstico)
// porque cada clique gera chamadas reais à API externa, sujeitas a cota.
type SyncHandler struct {
	discovery *statsync.DiscoveryUsecase
	update    *statsync.UpdateUsecase
}

func NewSyncHandler(discovery *statsync.DiscoveryUsecase, update *statsync.UpdateUsecase) *SyncHandler {
	return &SyncHandler{discovery: discovery, update: update}
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

	c.JSON(http.StatusOK, syncRunResponse{Discovery: discoveryResult, Update: updateResult})
}
