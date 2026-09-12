package config

import (
	"os"
	"strconv"
	"strings"
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

	// --- Dimensionamento do ciclo de sincronização -------------------------
	//
	// Estes dois números precisam ser lidos JUNTOS, e junto com a frequência do
	// cron. O ciclo roda uma vez por dia (ver render.yaml), então tudo o que não
	// couber num ciclo só é adiado por 24 horas.
	//
	// APIFootballRateLimitPerMin é o teto de requisições POR MINUTO do plano
	// contratado. O cliente deriva daqui o espaçamento entre chamadas. Padrão 10
	// = plano Free, que é o valor seguro: subir isso sem ter o plano correspondente
	// faz a API responder 429 e, se o excesso for grande, o firewall dela pode
	// bloquear a chave.
	//
	//	Free 10 · Pro 300 · Ultra 450 · Mega 900
	//
	// SyncMaxPerCycle é quantas partidas o Worker de Atualização finaliza por
	// ciclo. Cada partida custa até 2 requisições (lookup + estatísticas).
	//
	// O padrão 50 vem de quando o ciclo rodava a cada 15 minutos — 50 por ciclo
	// dava ~4.800 por dia. Com UM ciclo diário, 50 vira o teto do DIA inteiro, e
	// isso não cobre um fim de semana cheio (12 ligas somam bem mais que 50 jogos
	// num sábado). Quem roda uma vez por dia precisa subir este número, senão a
	// fila de partidas sem resultado só cresce.
	APIFootballRateLimitPerMin int
	SyncMaxPerCycle            int

	// TTL padrão do cache de cálculos do módulo de Inteligência Estatística.
	// Regra do documento de requisitos: "atualização automática diária".
	IntelligenceCacheTTL time.Duration

	// Assinatura Premium (Stripe Checkout hospedado + Billing Portal + webhooks).
	// Sem STRIPE_SECRET_KEY configurada, os endpoints /billing/* respondem 503 com
	// mensagem clara em vez de quebrar o restante da aplicação — o resto do app
	// (incluindo os módulos gratuitos) continua funcionando normalmente.
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceID       string
	StripeTrialDays     int
	// URL pública do frontend, usada para montar as URLs de sucesso/cancelamento
	// do Checkout e de retorno do Billing Portal.
	FrontendURL string

	// DevPremiumEmails libera acesso Premium manualmente para e-mails específicos,
	// sem depender do Stripe — uso interno de dev/QA (ver pkg/devaccess). Lista
	// separada por vírgula em DEV_PREMIUM_EMAILS. Vazio por padrão.
	DevPremiumEmails []string

	// Envio de e-mail (Resend — ver pkg/email) usado pelo fluxo "esqueci minha
	// senha". Sem RESEND_API_KEY configurada, POST /auth/forgot-password responde
	// 503 com mensagem clara em vez de quebrar o restante da aplicação.
	ResendAPIKey string
	// EmailFrom deve estar no formato "Nome <email@dominio>" aceito pela Resend.
	// Por padrão usa o remetente de testes da própria Resend, que só entrega para o
	// e-mail da conta Resend enquanto nenhum domínio próprio for verificado.
	EmailFrom string
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
		DevPremiumEmails:    getEnvList("DEV_PREMIUM_EMAILS"),

		ResendAPIKey: getEnv("RESEND_API_KEY", ""),
		EmailFrom:    getEnv("EMAIL_FROM", "CornerLab <onboarding@resend.dev>"),

		// Padrões conservadores de propósito: são os do plano Free. Quem tem plano
		// pago sobe os dois no ambiente (ver render.yaml).
		APIFootballRateLimitPerMin: getEnvInt("API_FOOTBALL_RATE_LIMIT_PER_MIN", 10),
		SyncMaxPerCycle:            getEnvInt("SYNC_MAX_PER_CYCLE", 50),
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

func getEnvList(key string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
