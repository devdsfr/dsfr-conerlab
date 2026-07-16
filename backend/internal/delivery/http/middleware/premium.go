package middleware

import (
	"net/http"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/gin-gonic/gin"
)

// RequirePremium bloqueia o acesso a recursos da Assinatura Premium (Gestão de
// Banca evolutiva, Alertas, Exportações) para usuários sem assinatura ativa/em
// trial. Deve ser usado sempre depois de AuthRequired na cadeia de middlewares,
// já que depende do user_id já estar no contexto.
//
// Responde 402 Payment Required (não 403) para o frontend poder diferenciar "não
// autenticado" (401) de "autenticado mas sem plano" (402) e mostrar o paywall
// certo em vez de redirecionar para o login.
func RequirePremium(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := UserIDFromContext(c)
		if userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token ausente"})
			return
		}
		user, err := users.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "usuário não encontrado"})
			return
		}
		if !user.IsPremium() {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error": "recurso exclusivo da Assinatura Premium",
				"code":  "premium_required",
			})
			return
		}
		c.Next()
	}
}
