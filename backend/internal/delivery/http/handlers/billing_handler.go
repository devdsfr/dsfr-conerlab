package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/usecase/billing"
)

type BillingHandler struct {
	billing *billing.Usecase
}

func NewBillingHandler(b *billing.Usecase) *BillingHandler {
	return &BillingHandler{billing: b}
}

// Status godoc
// @Summary Consultar status da assinatura do usuário logado
// @Tags billing
// @Produce json
// @Success 200 {object} billing.Status
// @Router /api/v1/billing/status [get]
func (h *BillingHandler) Status(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	status, err := h.billing.Status(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// Checkout godoc
// @Summary Criar sessão de checkout do Stripe (assinatura premium, com trial)
// @Tags billing
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/v1/billing/checkout [post]
func (h *BillingHandler) Checkout(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	url, err := h.billing.CreateCheckoutSession(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, billing.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// Portal godoc
// @Summary Criar sessão do Billing Portal (gerenciar assinatura: cartão, cancelar)
// @Tags billing
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/v1/billing/portal [post]
func (h *BillingHandler) Portal(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	url, err := h.billing.CreatePortalSession(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, billing.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// Webhook godoc
// @Summary Receber eventos de assinatura do Stripe
// @Tags billing
// @Accept json
// @Success 200 {object} map[string]any
// @Router /api/v1/billing/webhook [post]
//
// Precisa do corpo bruto da requisição (não passar por c.ShouldBindJSON antes) —
// a verificação de assinatura HMAC do Stripe é feita sobre os bytes exatos
// recebidos, e qualquer parsing/reserialização anterior invalidaria a assinatura.
func (h *BillingHandler) Webhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo da requisição inválido"})
		return
	}
	signature := c.GetHeader("Stripe-Signature")
	if err := h.billing.HandleWebhook(c.Request.Context(), payload, signature); err != nil {
		if errors.Is(err, billing.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}
