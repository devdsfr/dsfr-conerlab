package http

import (
	"net/http"

	"github.com/devdsfr/cornerlab/internal/delivery/http/handlers"
	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth            *handlers.AuthHandler
	Catalog         *handlers.CatalogHandler
	Dashboard       *handlers.DashboardHandler
	Comparator      *handlers.ComparatorHandler
	Filter          *handlers.FilterHandler
	Bet             *handlers.BetHandler
	Intelligence    *handlers.IntelligenceHandler
	Alert           *handlers.AlertHandler
	StrategyHistory *handlers.StrategyHistoryHandler
	Export          *handlers.ExportHandler
	Diagnostics     *handlers.DiagnosticsHandler
	Bankroll        *handlers.BankrollHandler
	Billing         *handlers.BillingHandler
}

func NewRouter(h Handlers, jwtSecret string, users repository.UserRepository) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "cornerlab-api"})
	})

	r.GET("/docs/*any", func(c *gin.Context) {
		if c.Param("any") == "/openapi.yaml" {
			c.File("./docs/openapi.yaml")
			return
		}
		swaggerUIHandler()(c)
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		auth.POST("/register", h.Auth.Register)
		auth.POST("/login", h.Auth.Login)
		auth.POST("/forgot-password", h.Auth.ForgotPassword)
		auth.POST("/reset-password", h.Auth.ResetPassword)

		api.GET("/leagues", h.Catalog.ListLeagues)
		api.GET("/leagues/:id/seasons", h.Catalog.ListSeasons)
		api.GET("/teams", h.Catalog.ListTeams)

		api.GET("/dashboard", h.Dashboard.GetDashboard)
		api.GET("/comparator", h.Comparator.Compare)
		api.POST("/filters/run", h.Filter.Run)

		// Módulo de Inteligência Estatística — leitura pública (mesma política do
		// Dashboard/Comparador/Backtest: não exige login para consultar estatísticas).
		intel := api.Group("/intelligence")
		intel.GET("/consistency", h.Intelligence.Consistency)
		intel.GET("/trend", h.Intelligence.Trend)
		intel.GET("/stability", h.Intelligence.Stability)
		intel.GET("/score", h.Intelligence.Score)
		intel.GET("/compatibility", h.Intelligence.Compatibility)
		intel.GET("/opponent", h.Intelligence.Opponent)
		intel.GET("/ranking", h.Intelligence.Ranking)
		intel.GET("/executive-dashboard", h.Intelligence.ExecutiveDashboard)
		intel.POST("/explain", h.Intelligence.Explain)

		// Painel "Integrações" — status/consumo das chaves de API externas
		// (OpenAI, API-Football, SportMonks) e botão "Testar agora" por provedor.
		diag := api.Group("/diagnostics")
		diag.GET("/usage", h.Diagnostics.Usage)
		diag.GET("/recent", h.Diagnostics.Recent)
		diag.POST("/test/:provider", h.Diagnostics.TestConnection)

		// Assinatura Premium (Stripe). /webhook é a única rota pública do grupo — é
		// chamada pelo Stripe, não pelo navegador do usuário, então não carrega o JWT
		// da aplicação (a autenticidade é garantida pela assinatura HMAC do Stripe).
		billingGroup := api.Group("/billing")
		billingGroup.POST("/webhook", h.Billing.Webhook)

		authGroup := api.Group("")
		authGroup.Use(middleware.AuthRequired(jwtSecret))
		{
			authGroup.POST("/filters", h.Filter.Save)
			authGroup.GET("/filters", h.Filter.List)
			authGroup.POST("/filters/:id/duplicate", h.Filter.Duplicate)
			authGroup.DELETE("/filters/:id", h.Filter.Delete)

			authGroup.POST("/bets", h.Bet.Create)
			authGroup.GET("/bets", h.Bet.List)
			authGroup.GET("/bets/dashboard", h.Bet.Dashboard)
			authGroup.DELETE("/bets/:id", h.Bet.Delete)

			authGroup.GET("/strategy-history", h.StrategyHistory.List)

			authGroup.GET("/billing/status", h.Billing.Status)
			authGroup.POST("/billing/checkout", h.Billing.Checkout)
			authGroup.POST("/billing/portal", h.Billing.Portal)

			premiumGroup := authGroup.Group("")
			premiumGroup.Use(middleware.RequirePremium(users))
			{
				// Alertas personalizados — recurso premium (ver ESTRATEGIA-MONETIZACAO.md).
				premiumGroup.POST("/alerts", h.Alert.Create)
				premiumGroup.GET("/alerts", h.Alert.List)
				premiumGroup.POST("/alerts/:id/evaluate", h.Alert.Evaluate)
				premiumGroup.DELETE("/alerts/:id", h.Alert.Delete)

				// Módulo de Gestão Evolutiva de Banca — usa as apostas já registradas pelo
				// usuário (acima) como base real dos cálculos de evolução de fase.
				bankroll := premiumGroup.Group("/bankroll")
				bankroll.GET("/status", h.Bankroll.Status)
				bankroll.GET("/phases", h.Bankroll.ListPhases)
				bankroll.PUT("/phases", h.Bankroll.SetPhases)
				bankroll.GET("/criteria", h.Bankroll.GetCriteria)
				bankroll.PUT("/criteria", h.Bankroll.SetCriteria)
				bankroll.POST("/promote", h.Bankroll.Promote)
				bankroll.POST("/demote", h.Bankroll.Demote)
				bankroll.GET("/history", h.Bankroll.History)
				bankroll.POST("/rounds", h.Bankroll.ConfirmRound)
				bankroll.GET("/rounds", h.Bankroll.ListRounds)

				// Exportação de dados (CSV/XLSX) — antes 100% pública sem autenticação;
				// agora exige login + assinatura premium.
				exports := premiumGroup.Group("/exports")
				exports.GET("/dashboard", h.Export.DashboardCSV)
				exports.GET("/comparator", h.Export.ComparatorCSV)
				exports.GET("/filters/run", h.Export.FilterRunCSV)
				exports.GET("/ranking", h.Export.RankingCSV)
			}
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// swaggerUIHandler serve uma página estática de Swagger UI (via CDN) apontando para
// o nosso openapi.yaml, evitando dependência de geração de código em build time.
func swaggerUIHandler() gin.HandlerFunc {
	page := []byte(`<!DOCTYPE html>
<html>
<head>
  <title>CornerLab API Docs</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui.min.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui-bundle.min.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/docs/openapi.yaml',
        dom_id: '#swagger-ui',
      });
    };
  </script>
</body>
</html>`)
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	}
}
