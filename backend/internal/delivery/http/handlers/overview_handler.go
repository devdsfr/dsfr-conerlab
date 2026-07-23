package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/repository"
)

// OverviewHandler alimenta a página "Visão Geral" — tela inicial do app — com os
// próximos jogos mapeados pelo Worker de Descoberta, para o usuário já ter uma visão
// de calendário assim que abre o CornerLab, sem precisar escolher liga/time antes.
// Leitura pública, mesma política do Dashboard/Comparador.
type OverviewHandler struct {
	matches repository.MatchRepository
}

func NewOverviewHandler(matches repository.MatchRepository) *OverviewHandler {
	return &OverviewHandler{matches: matches}
}

// UpcomingMatches godoc
// @Summary Próximos jogos mapeados (ligas com dado real), para o calendário da Visão Geral
// @Tags overview
// @Produce json
// @Success 200 {object} object{matches=[]domain.UpcomingMatch}
// @Router /api/v1/overview/upcoming [get]
func (h *OverviewHandler) UpcomingMatches(c *gin.Context) {
	matches, err := h.matches.ListUpcoming(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"matches": matches})
}
