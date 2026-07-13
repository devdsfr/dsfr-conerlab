package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/usecase/intelligence"
)

type AlertHandler struct {
	alerts *intelligence.AlertUsecase
}

func NewAlertHandler(alerts *intelligence.AlertUsecase) *AlertHandler {
	return &AlertHandler{alerts: alerts}
}

type createAlertRequest struct {
	Name       string                       `json:"name" binding:"required"`
	Definition intelligence.AlertDefinition `json:"definition" binding:"required"`
}

// Create godoc
// @Summary Criar regra de alerta inteligente (Módulo 7)
// @Tags alerts
// @Router /api/v1/alerts [post]
func (h *AlertHandler) Create(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req createAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.alerts.Create(c.Request.Context(), userID, req.Name, req.Definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// List godoc
// @Summary Listar regras de alerta do usuário
// @Tags alerts
// @Router /api/v1/alerts [get]
func (h *AlertHandler) List(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	rules, err := h.alerts.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// Evaluate godoc
// @Summary Avaliar uma regra de alerta agora
// @Tags alerts
// @Router /api/v1/alerts/{id}/evaluate [post]
func (h *AlertHandler) Evaluate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	result, err := h.alerts.Evaluate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Delete godoc
// @Summary Excluir regra de alerta
// @Tags alerts
// @Router /api/v1/alerts/{id} [delete]
func (h *AlertHandler) Delete(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	if err := h.alerts.Delete(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
