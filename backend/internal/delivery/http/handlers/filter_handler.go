package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/dto"
	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/pkg/jwtutil"
)

// FreeHistoryCapDays é a janela de histórico disponível no plano gratuito para o
// Simulador de Filtros (ver ESTRATEGIA-MONETIZACAO.md — "histórico completo" é
// recurso da Assinatura Premium). Usuários premium/trial não sofrem esse limite.
const FreeHistoryCapDays = 90

type FilterHandler struct {
	filters    *usecase.FilterUsecase
	filterRepo repository.FilterRepository
	history    *usecase.StrategyHistoryUsecase
	users      repository.UserRepository
	jwtSecret  string
}

func NewFilterHandler(filters *usecase.FilterUsecase, filterRepo repository.FilterRepository, history *usecase.StrategyHistoryUsecase, users repository.UserRepository, jwtSecret string) *FilterHandler {
	return &FilterHandler{filters: filters, filterRepo: filterRepo, history: history, users: users, jwtSecret: jwtSecret}
}

// Run godoc
// @Summary Executar filtro/backtest (Módulo 3)
// @Tags filters
// @Accept json
// @Produce json
// @Param request body dto.FilterRunRequest true "Critérios do filtro"
// @Success 200 {object} usecase.BacktestResult
// @Router /api/v1/filters/run [post]
func (h *FilterHandler) Run(c *gin.Context) {
	var req dto.FilterRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	criteria := usecase.FilterCriteria{
		TeamID:           req.TeamID,
		LastNGames:       req.LastNGames,
		HomeAway:         req.HomeAway,
		CornersThreshold: req.CornersThreshold,
		OpponentTier:     req.OpponentTier,
		MaxOdds:          req.MaxOdds,
		Stake:            req.Stake,
	}

	// Cap de histórico do plano gratuito: sem token, ou com token de usuário sem
	// assinatura ativa/trial, o backtest só considera os últimos FreeHistoryCapDays
	// dias. Usuários premium chamam RunBacktest com maxAgeDays=0 (sem limite).
	maxAgeDays := FreeHistoryCapDays
	userID, hasUser := h.optionalUserID(c)
	if hasUser {
		if user, err := h.users.GetByID(c.Request.Context(), userID); err == nil && user.IsPremium() {
			maxAgeDays = 0
		}
	}

	result, err := h.filters.RunBacktest(c.Request.Context(), req.LeagueID, req.SeasonIDs, criteria, maxAgeDays)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Persistência do histórico de estratégia (Módulo de Inteligência Estatística /
	// regra geral do MVP) é opcional: se o usuário estiver autenticado, a execução é
	// registrada para consulta posterior em /api/v1/strategy-history. Sem token, o
	// backtest funciona normalmente, apenas sem o registro histórico.
	if hasUser {
		_ = h.history.RecordRun(c.Request.Context(), userID, nil, req.LeagueID, req.SeasonIDs, criteria, result)
	}

	c.JSON(http.StatusOK, result)
}

func (h *FilterHandler) optionalUserID(c *gin.Context) (int64, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return 0, false
	}
	claims, err := jwtutil.ParseToken(h.jwtSecret, strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		return 0, false
	}
	return claims.UserID, true
}

// Save godoc
// @Summary Salvar filtro personalizado
// @Tags filters
// @Accept json
// @Produce json
// @Param request body dto.SaveFilterRequest true "Filtro"
// @Success 201 {object} domain.SavedFilter
// @Router /api/v1/filters [post]
func (h *FilterHandler) Save(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req dto.SaveFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !json.Valid([]byte(req.Definition)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "definition deve ser um JSON válido"})
		return
	}

	filter := &domain.SavedFilter{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
	}
	if err := h.filterRepo.Create(c.Request.Context(), filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, filter)
}

// List godoc
// @Summary Listar filtros salvos do usuário
// @Tags filters
// @Produce json
// @Success 200 {array} domain.SavedFilter
// @Router /api/v1/filters [get]
func (h *FilterHandler) List(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	filters, err := h.filterRepo.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, filters)
}

// Duplicate godoc
// @Summary Duplicar filtro salvo
// @Tags filters
// @Produce json
// @Param id path int true "Filter ID"
// @Success 201 {object} domain.SavedFilter
// @Router /api/v1/filters/{id}/duplicate [post]
func (h *FilterHandler) Duplicate(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	original, err := h.filterRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "filtro não encontrado"})
		return
	}
	copy := &domain.SavedFilter{
		UserID:      userID,
		Name:        original.Name + " (cópia)",
		Description: original.Description,
		Definition:  original.Definition,
	}
	if err := h.filterRepo.Create(c.Request.Context(), copy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, copy)
}

// Delete godoc
// @Summary Excluir filtro salvo
// @Tags filters
// @Param id path int true "Filter ID"
// @Success 204
// @Router /api/v1/filters/{id} [delete]
func (h *FilterHandler) Delete(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	if err := h.filterRepo.Delete(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
