package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
)

// StrategyHandler expõe o Strategy Engine (Remodelagem F4): CRUD de estratégias
// e execução sob demanda. Os recálculos periódicos ficam com o Strategy Worker
// (cmd/worker) — regra do doc 15: nada pesado dentro de requisição HTTP, exceto
// o "run" explícito disparado pelo próprio usuário.
type StrategyHandler struct {
	repo   repository.StrategyRepository
	engine *strategyengine.Engine
}

func NewStrategyHandler(repo repository.StrategyRepository, engine *strategyengine.Engine) *StrategyHandler {
	return &StrategyHandler{repo: repo, engine: engine}
}

type createStrategyRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Definition  string `json:"definition" binding:"required"` // JSON (mesmo formato do Simulador)
	Favorite    bool   `json:"favorite"`
}

// Create godoc
// @Summary Criar estratégia
// @Tags strategies
// @Router /api/v1/strategies [post]
func (h *StrategyHandler) Create(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req createStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := strategyengine.ParseDefinition(req.Definition); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s := &domain.Strategy{
		OwnerID:     &userID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
		Origin:      "user",
		Visibility:  "private",
		Active:      true,
		Favorite:    req.Favorite,
	}
	if err := h.repo.Create(c.Request.Context(), s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

// List godoc
// @Summary Listar estratégias do usuário (+ públicas do sistema)
// @Tags strategies
// @Router /api/v1/strategies [get]
func (h *StrategyHandler) List(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	list, err := h.repo.ListForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// strategyBundle agrega estratégia + artefatos calculados em uma resposta só
// (base do Strategy Workspace do doc 14: "tudo em uma única página").
type strategyBundle struct {
	Strategy  *domain.Strategy       `json:"strategy"`
	Health    *domain.StrategyHealth `json:"health,omitempty"`
	Scores    *domain.StrategyScores `json:"scores,omitempty"`
	Backtests []domain.Backtest      `json:"backtests"`
}

// Get godoc
// @Summary Detalhe da estratégia com health, scores e últimos backtests
// @Tags strategies
// @Router /api/v1/strategies/{id} [get]
func (h *StrategyHandler) Get(c *gin.Context) {
	s, ok := h.ownedStrategy(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	health, _ := h.repo.GetHealth(ctx, s.ID)
	scores, _ := h.repo.GetScores(ctx, s.ID)
	backtests, _ := h.repo.LastBacktests(ctx, s.ID, 10)
	if backtests == nil {
		backtests = []domain.Backtest{}
	}
	c.JSON(http.StatusOK, strategyBundle{Strategy: s, Health: health, Scores: scores, Backtests: backtests})
}

// Run godoc
// @Summary Executar a estratégia agora (backtest + health + scores)
// @Tags strategies
// @Router /api/v1/strategies/{id}/run [post]
func (h *StrategyHandler) Run(c *gin.Context) {
	s, ok := h.ownedStrategy(c)
	if !ok {
		return
	}
	eval, err := h.engine.RunStrategy(c.Request.Context(), s)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, eval)
}

type updateFlagsRequest struct {
	Active   *bool `json:"active"`
	Favorite *bool `json:"favorite"`
}

// UpdateFlags godoc
// @Summary Ativar/desativar ou favoritar estratégia
// @Tags strategies
// @Router /api/v1/strategies/{id} [patch]
func (h *StrategyHandler) UpdateFlags(c *gin.Context) {
	s, ok := h.ownedStrategy(c)
	if !ok {
		return
	}
	var req updateFlagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	active, favorite := s.Active, s.Favorite
	if req.Active != nil {
		active = *req.Active
	}
	if req.Favorite != nil {
		favorite = *req.Favorite
	}
	if err := h.repo.SetFlags(c.Request.Context(), s.ID, active, favorite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.Active, s.Favorite = active, favorite
	c.JSON(http.StatusOK, s)
}

// Delete godoc
// @Summary Excluir estratégia
// @Tags strategies
// @Router /api/v1/strategies/{id} [delete]
func (h *StrategyHandler) Delete(c *gin.Context) {
	s, ok := h.ownedStrategy(c)
	if !ok {
		return
	}
	userID := middleware.UserIDFromContext(c)
	if err := h.repo.Delete(c.Request.Context(), s.ID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ownedStrategy carrega a estratégia do path e garante que o usuário pode vê-la
// (dono, ou pública do sistema — esta última só para leitura/execução).
func (h *StrategyHandler) ownedStrategy(c *gin.Context) (*domain.Strategy, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return nil, false
	}
	s, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	if s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "estratégia não encontrada"})
		return nil, false
	}
	userID := middleware.UserIDFromContext(c)
	isOwner := s.OwnerID != nil && *s.OwnerID == userID
	isPublicSystem := s.OwnerID == nil && s.Visibility == "public"
	if !isOwner && !isPublicSystem {
		c.JSON(http.StatusNotFound, gin.H{"error": "estratégia não encontrada"})
		return nil, false
	}
	return s, true
}
