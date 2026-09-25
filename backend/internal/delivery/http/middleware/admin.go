package middleware

import (
	"net/http"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/gin-gonic/gin"
)

// RequireAdmin restringe a rota a administradores (REV-P4, B4).
//
// Deve vir depois de AuthRequired. O usuário é carregado DO BANCO a partir do
// user_id do token — o mesmo padrão de RequirePremium — em vez de confiar no
// e-mail gravado no token, que pode estar desatualizado se o cadastro mudar.
//
// Respostas:
//   - 401 sem token ou usuário inexistente;
//   - 403 autenticado mas não administrador.
func RequireAdmin(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := UserIDFromContext(c)
		if userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token ausente"})
			return
		}
		user, err := users.GetByID(c.Request.Context(), userID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "usuário não encontrado"})
			return
		}
		if !user.IsAdmin() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "ação restrita a administradores",
				"code":  "admin_required",
			})
			return
		}
		c.Next()
	}
}
