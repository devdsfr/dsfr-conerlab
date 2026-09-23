package handlers

import (
	"net/http"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/gin-gonic/gin"
)

type ComparatorHandler struct {
	comparator *usecase.ComparatorUsecase
}

func NewComparatorHandler(comparator *usecase.ComparatorUsecase) *ComparatorHandler {
	return &ComparatorHandler{comparator: comparator}
}

// Compare godoc
// @Summary Comparar duas equipes (Módulo 2)
// @Tags comparator
// @Produce json
// @Param team_a query int true "ID da equipe A"
// @Param team_b query int true "ID da equipe B"
// @Param league_id query int false "ID do campeonato"
// @Param season_id query int false "ID da temporada — sem ele a amostra atravessa temporadas"
// @Param limit query int false "Quantidade de jogos. Padrão 10"
// @Param venue query string false "geral | casa | fora. Padrão geral"
// @Param metric query string false "corners | goals | shots | shots_on_target | offsides. Padrão corners"
// @Param perspective query string false "produzido | concedido | total. Padrão total"
// @Success 200 {object} usecase.ComparisonResult
// @Router /api/v1/comparator [get]
func (h *ComparatorHandler) Compare(c *gin.Context) {
	teamA, err := strconv.ParseInt(c.Query("team_a"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_a é obrigatório e deve ser numérico"})
		return
	}
	teamB, err := strconv.ParseInt(c.Query("team_b"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_b é obrigatório e deve ser numérico"})
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

	// season_id passa a existir no contrato. Sem ele, a comparação misturava
	// temporadas silenciosamente — ver o comentário em ComparatorUsecase.buildSide.
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

	result, err := h.comparator.Compare(c.Request.Context(), usecase.ComparatorQuery{
		TeamAID:  teamA,
		TeamBID:  teamB,
		LeagueID: leagueID,
		SeasonID: seasonID,
		Limit:    limit,
		// Os três eixos têm padrão seguro: valor desconhecido cai em
		// geral/escanteios/total, que é o comportamento anterior do Comparador.
		Venue:       usecase.ParseVenue(c.Query("venue")),
		Metric:      usecase.ParseMetric(c.Query("metric")),
		Perspective: usecase.ParsePerspective(c.Query("perspective")),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
