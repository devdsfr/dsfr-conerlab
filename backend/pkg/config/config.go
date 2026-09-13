package config

import (
	"fmt"
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

	// SyncTimeoutMinutes limita o ciclo disparado pelo botão "Sincronizar agora".
	// Precisa acompanhar o tamanho do atraso a recuperar: um ciclo rotineiro
	// termina em minutos, mas recuperar semanas de acúmulo pode levar bem mais
	// (ver o comentário de syncTimeout em handlers/sync_handler.go).
	SyncTimeoutMinutes int

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
		SyncTimeoutMinutes:         getEnvInt("SYNC_TIMEOUT_MINUTES", 60),
	}
}

// Validate recusa uma configuração que só funcionaria em desenvolvimento.
//
// POR QUE ISTO EXISTE — caso real, 13/09/2026, primeira execução do Cron Job:
//
//	{"level":"WARN","msg":"postgres ainda não respondeu, tentando de novo",
//	 "erro":"failed to connect to `user=cornerlab database=cornerlab`:
//	         127.0.0.1:5432 (localhost): connect: connection refused"}
//	❌ Your cronjob failed because of an error: Exited with status 1
//
// O worker não estava tentando falar com o Neon: estava tentando falar com um
// Postgres local que não existe naquele contêiner. DATABASE_URL chegou vazia e o
// getEnv abaixo devolveu silenciosamente o padrão de desenvolvimento.
//
// O erro resultante descreve um SINTOMA a três passos da causa — quem lê
// "connection refused em 127.0.0.1" investiga rede, firewall, Neon fora do ar.
// A causa era uma variável de ambiente faltando, e nada no log dizia isso.
//
// Um padrão de desenvolvimento é conveniência em desenvolvimento e armadilha em
// produção. Aqui ele deixa de ser aceito quando ENVIRONMENT=production: falta de
// configuração passa a falhar na inicialização, dizendo qual variável falta.
func (c Config) Validate() error {
	if c.Environment != "production" {
		return nil
	}

	var faltando []string
	if c.DatabaseURL == "" || strings.Contains(c.DatabaseURL, "localhost") || strings.Contains(c.DatabaseURL, "127.0.0.1") {
		faltando = append(faltando, "DATABASE_URL (vazia ou apontando para localhost — em produção o banco é remoto)")
	}
	if c.JWTSecret == "" || c.JWTSecret == "change-me-in-production" {
		faltando = append(faltando, "JWT_SECRET (vazio ou ainda no valor de exemplo)")
	}

	if len(faltando) > 0 {
		return fmt.Errorf("configuração inválida para ENVIRONMENT=production: %s",
			strings.Join(faltando, "; "))
	}
	return nil
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
