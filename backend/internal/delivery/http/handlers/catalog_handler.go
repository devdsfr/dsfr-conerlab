package handlers

import (
	"net/http"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/gin-gonic/gin"
)

// CatalogHandler expõe campeonatos, temporadas e equipes — usados para popular os
// seletores do Dashboard, Comparador e Simulador de Filtros.
type CatalogHandler struct {
	leagues repository.LeagueRepository
	teams   repository.TeamRepository
}

func NewCatalogHandler(leagues repository.LeagueRepository, teams repository.TeamRepository) *CatalogHandler {
	return &CatalogHandler{leagues: leagues, teams: teams}
}

// ListLeagues godoc
// @Summary Listar campeonatos
// @Tags catalog
// @Produce json
// @Success 200 {array} domain.League
// @Router /api/v1/leagues [get]
func (h *CatalogHandler) ListLeagues(c *gin.Context) {
	leagues, err := h.leagues.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, leagues)
}

// ListSeasons godoc
// @Summary Listar temporadas de um campeonato
// @Tags catalog
// @Produce json
// @Param id path int true "League ID"
// @Success 200 {array} domain.Season
// @Router /api/v1/leagues/{id}/seasons [get]
func (h *CatalogHandler) ListSeasons(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	seasons, err := h.leagues.ListSeasons(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, seasons)
}

// ListTeams godoc
// @Summary Listar/pesquisar equipes
// @Tags catalog
// @Produce json
// @Param league_id query int false "League ID"
// @Param q query string false "Busca por nome"
// @Success 200 {array} domain.Team
// @Router /api/v1/teams [get]
func (h *CatalogHandler) ListTeams(c *gin.Context) {
	if q := c.Query("q"); q != "" {
		teams, err := h.teams.Search(c.Request.Context(), q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, teams)
		return
	}

	var leagueID *int64
	if lq := c.Query("league_id"); lq != "" {
		id, err := strconv.ParseInt(lq, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "league_id inválido"})
			return
		}
		leagueID = &id
	}
	teams, err := h.teams.List(c.Request.Context(), leagueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, teams)
}
