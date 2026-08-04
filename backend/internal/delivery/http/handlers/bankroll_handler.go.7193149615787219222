package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/usecase/bankroll"
)

type BankrollHandler struct {
	bankroll *bankroll.Usecase
}

func NewBankrollHandler(b *bankroll.Usecase) *BankrollHandler {
	return &BankrollHandler{bankroll: b}
}

// Status godoc
// @Summary Dashboard de evolução de banca: fase atual, checklist, Score de Maturidade
// @Tags bankroll
// @Router /api/v1/bankroll/status [get]
func (h *BankrollHandler) Status(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	status, err := h.bankroll.Status(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// ListPhases godoc
// @Summary Listar as fases de banca configuradas
// @Tags bankroll
// @Router /api/v1/bankroll/phases [get]
func (h *BankrollHandler) ListPhases(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	phases, err := h.bankroll.Phases(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"phases": phases})
}

type setPhasesRequest struct {
	Phases []struct {
		Sequence int     `json:"sequence" binding:"required"`
		Name     string  `json:"name" binding:"required"`
		Amount   float64 `json:"amount" binding:"required"`
	} `json:"phases" binding:"required"`
}

// SetPhases godoc
// @Summary Configurar a sequência completa de fases de banca
// @Tags bankroll
// @Router /api/v1/bankroll/phases [put]
func (h *BankrollHandler) SetPhases(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req setPhasesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	phases := make([]domain.BankrollPhase, len(req.Phases))
	for i, p := range req.Phases {
		phases[i] = domain.BankrollPhase{UserID: userID, Sequence: p.Sequence, Name: p.Name, Amount: p.Amount}
	}
	updated, err := h.bankroll.SetPhases(c.Request.Context(), userID, phases)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"phases": updated})
}

// GetCriteria godoc
// @Summary Consultar os critérios mínimos de evolução configurados
// @Tags bankroll
// @Router /api/v1/bankroll/criteria [get]
func (h *BankrollHandler) GetCriteria(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	criteria, err := h.bankroll.Criteria(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, criteria)
}

type setCriteriaRequest struct {
	MinDays                int     `json:"min_days"`
	MinBets                int     `json:"min_bets"`
	MinWinRate             float64 `json:"min_win_rate"`
	MinROI                 float64 `json:"min_roi"`
	MinYield               float64 `json:"min_yield"`
	RequirePositiveProfit  bool    `json:"require_positive_profit"`
	MinCompletedCycles     int     `json:"min_completed_cycles"`
	CycleWinStreak         int     `json:"cycle_win_streak"`
}

// SetCriteria godoc
// @Summary Configurar os critérios mínimos de evolução de banca
// @Tags bankroll
// @Router /api/v1/bankroll/criteria [put]
func (h *BankrollHandler) SetCriteria(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req setCriteriaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	criteria := domain.BankrollCriteria{
		UserID: userID, MinDays: req.MinDays, MinBets: req.MinBets, MinWinRate: req.MinWinRate,
		MinROI: req.MinROI, MinYield: req.MinYield, RequirePositiveProfit: req.RequirePositiveProfit,
		MinCompletedCycles: req.MinCompletedCycles, CycleWinStreak: req.CycleWinStreak,
	}
	if err := h.bankroll.SetCriteria(c.Request.Context(), criteria); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, criteria)
}

type promoteRequest struct {
	Notes string `json:"notes"`
}

// Promote godoc
// @Summary Confirmar manualmente a evolução para a próxima fase (exige todos os critérios atendidos)
// @Tags bankroll
// @Router /api/v1/bankroll/promote [post]
func (h *BankrollHandler) Promote(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req promoteRequest
	_ = c.ShouldBindJSON(&req)
	entry, err := h.bankroll.Promote(c.Request.Context(), userID, req.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

type demoteRequest struct {
	Reason string `json:"reason"`
	Notes  string `json:"notes"`
}

// Demote godoc
// @Summary Confirmar manualmente o rebaixamento para a fase anterior
// @Tags bankroll
// @Router /api/v1/bankroll/demote [post]
func (h *BankrollHandler) Demote(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req demoteRequest
	_ = c.ShouldBindJSON(&req)
	entry, err := h.bankroll.Demote(c.Request.Context(), userID, req.Reason, req.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// History godoc
// @Summary Histórico completo de promoções e rebaixamentos de banca
// @Tags bankroll
// @Router /api/v1/bankroll/history [get]
func (h *BankrollHandler) History(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	entries, err := h.bankroll.History(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": entries})
}

type confirmRoundRequest struct {
	PhaseSequence int     `json:"phase_sequence" binding:"required"`
	Result        float64 `json:"result"`
	Notes         string  `json:"notes"`
}

// ConfirmRound godoc
// @Summary Confirmar manualmente o resultado real de uma rodada e atualizar o saldo acumulado
// @Tags bankroll
// @Router /api/v1/bankroll/rounds [post]
func (h *BankrollHandler) ConfirmRound(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	var req confirmRoundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry, err := h.bankroll.ConfirmRound(c.Request.Context(), userID, req.PhaseSequence, req.Result, req.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// ListRounds godoc
// @Summary Listar as rodadas confirmadas (registro real de saldo acumulado)
// @Tags bankroll
// @Router /api/v1/bankroll/rounds [get]
func (h *BankrollHandler) ListRounds(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	rounds, err := h.bankroll.Rounds(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rounds": rounds})
}
