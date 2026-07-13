package config

import (
	"os"
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

	// Usado pelo módulo de Inteligência Estatística para gerar explicações em texto.
	// Se vazio, o endpoint de explicação retorna erro claro em vez de quebrar.
	AnthropicAPIKey string

	// TTL padrão do cache de cálculos do módulo de Inteligência Estatística.
	// Regra do documento de requisitos: "atualização automática diária".
	IntelligenceCacheTTL time.Duration
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

		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),

		IntelligenceCacheTTL: 24 * time.Hour,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
