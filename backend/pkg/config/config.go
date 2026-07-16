package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          string
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	JWTSecret     string
	JWTExpiry     time.Duration
	Environment   string

	// Provedor de dados esportivos usado pelo comando de sincronização (cmd/sync).
	// "api_football" | "sportmonks" | "fallback" (tenta o primário e cai para o secundário)
	SportsDataProvider string
	APIFootballKey     string
	SportMonksKey      string

	// Usado pelo módulo de Inteligência Estatística para gerar explicações em texto
	// via OpenAI (Chat Completions API). Se vazio, o endpoint de explicação retorna
	// erro claro em vez de quebrar.
	OpenAIAPIKey string

	// Provedor usado pelo cmd/worker (Módulo de Sincronização de Dados, em background
	// contínuo — diferente de SportsDataProvider acima, que é usado pelo cmd/sync
	// manual). "api_football" (padrão, já tem chave real configurada) | "sofascore"
	// (interface pronta, integração real ainda pendente — ver
	// internal/integration/statsprovider/sofascore).
	StatisticsProvider string

	// TTL padrão do cache de cálculos do módulo de Inteligência Estatística.
	// Regra do documento de requisitos: "atualização automática diária".
	IntelligenceCacheTTL time.Duration

	// Assinatura Premium (Stripe Checkout hospedado + Billing Portal + webhooks).
	// Sem STRIPE_SECRET_KEY configurada, os endpoints /billing/* respondem 503 com
	// mensagem clara em vez de quebrar o restante da aplicação — o resto do app
	// (incluindo os módulos gratuitos) continua funcionando normalmente.
	StripeSecretKey    string
	StripeWebhookSecret string
	StripePriceID      string
	StripeTrialDays    int
	// URL pública do frontend, usada para montar as URLs de sucesso/cancelamento
	// do Checkout e de retorno do Billing Portal.
	FrontendURL string
}

func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://cornerlab:cornerlab@localhost:5432/cornerlab?sslmode=disable"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:     24 * time.Hour,
		Environment:   getEnv("ENVIRONMENT", "development"),

		SportsDataProvider: getEnv("SPORTS_DATA_PROVIDER", "fallback"),
		APIFootballKey:     getEnv("API_FOOTBALL_KEY", ""),
		SportMonksKey:      getEnv("SPORTMONKS_KEY", ""),

		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),

		StatisticsProvider: getEnv("STATISTICS_PROVIDER", "api_football"),

		IntelligenceCacheTTL: 24 * time.Hour,

		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceID:       getEnv("STRIPE_PRICE_ID", ""),
		StripeTrialDays:     getEnvInt("STRIPE_TRIAL_DAYS", 7),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:4200"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
