package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/usecase"
)

type StrategyHistoryHandler struct {
	history *usecase.StrategyHistoryUsecase
}

func NewStrategyHistoryHandler(history *usecase.StrategyHistoryUsecase) *StrategyHistoryHandler {
	return &StrategyHistoryHandler{history: history}
}

// List godoc
// @Summary Histórico de execuções de backtest do usuário
// @Tags strategy-history
// @Router /api/v1/strategy-history [get]
func (h *StrategyHistoryHandler) List(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	entries, err := h.history.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}
