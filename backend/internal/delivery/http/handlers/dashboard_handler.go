package handlers

import (
	"net/http"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	dashboard *usecase.DashboardUsecase
}

func NewDashboardHandler(dashboard *usecase.DashboardUsecase) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

// GetDashboard godoc
// @Summary Dashboard principal de uma equipe (Módulo 1)
// @Tags dashboard
// @Produce json
// @Param team_id query int true "ID da equipe"
// @Param league_id query int false "ID do campeonato"
// @Param season_id query int false "ID da temporada"
// @Param limit query int false "Quantidade de jogos (5, 10, 15, 20). Padrão 10"
// @Success 200 {object} usecase.DashboardResult
// @Router /api/v1/dashboard [get]
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório e deve ser numérico"})
		return
	}

	var leagueID *int64
	if v := c.Query("league_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "league_id inválido"})
			return
		}
		leagueID = &id
	}

	var seasonID *int64
	if v := c.Query("season_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "season_id inválido"})
			return
		}
		seasonID = &id
	}

	limit := 10
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			limit = l
		}
	}

	result, err := h.dashboard.GetDashboard(c.Request.Context(), teamID, leagueID, seasonID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
