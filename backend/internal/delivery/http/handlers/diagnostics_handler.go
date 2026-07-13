package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/usecase/diagnostics"
)

type DiagnosticsHandler struct {
	diag *diagnostics.Usecase
}

func NewDiagnosticsHandler(diag *diagnostics.Usecase) *DiagnosticsHandler {
	return &DiagnosticsHandler{diag: diag}
}

// Usage godoc
// @Summary Status e consumo das integrações externas (OpenAI, API-Football, SportMonks)
// @Tags diagnostics
// @Router /api/v1/diagnostics/usage [get]
func (h *DiagnosticsHandler) Usage(c *gin.Context) {
	summary, err := h.diag.Summary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": summary})
}

// Recent godoc
// @Summary Histórico recente de chamadas às APIs externas
// @Tags diagnostics
// @Router /api/v1/diagnostics/recent [get]
func (h *DiagnosticsHandler) Recent(c *gin.Context) {
	provider := c.Query("provider")
	limit, _ := strconv.Atoi(c.Query("limit"))
	entries, err := h.diag.Recent(c.Request.Context(), provider, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// TestConnection godoc
// @Summary Testa agora a conexão com um provedor externo (chamada mínima e real)
// @Tags diagnostics
// @Router /api/v1/diagnostics/test/{provider} [post]
func (h *DiagnosticsHandler) TestConnection(c *gin.Context) {
	provider := c.Param("provider")
	result, err := h.diag.TestConnection(c.Request.Context(), provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
