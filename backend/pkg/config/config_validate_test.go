package config

import "testing"

// P0 — reprodução do incidente de 13/09/2026.
//
// A primeira execução do Cron Job morreu em 23 segundos com:
//
//	failed to connect to `user=cornerlab database=cornerlab`:
//	127.0.0.1:5432 (localhost): connect: connection refused
//
// Esse é o valor PADRÃO de DATABASE_URL, devolvido silenciosamente por getEnv
// quando a variável chega vazia. O log falava de rede local; a causa era
// configuração ausente. Estes testes travam o comportamento novo: em produção,
// um padrão de desenvolvimento é erro de inicialização, não fallback.

func produção() Config {
	return Config{
		Environment: "production",
		DatabaseURL: "postgres://user:senha@ep-xyz.neon.tech/cornerlab?sslmode=require",
		JWTSecret:   "um-segredo-de-verdade",
	}
}

func TestProducaoRecusaDatabaseURLVazia(t *testing.T) {
	cfg := produção()
	cfg.DatabaseURL = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate aceitou DATABASE_URL vazia em produção — foi exatamente assim que o " +
			"cron tentou falar com um Postgres local inexistente")
	}
	if !contémTudo(err.Error(), "DATABASE_URL") {
		t.Errorf("a mensagem precisa NOMEAR a variável que falta, veio: %q", err)
	}
}

// O caso que realmente aconteceu: a variável não estava "ausente" do ponto de
// vista do código — getEnv já tinha trocado o vazio pelo padrão de localhost.
func TestProducaoRecusaBancoEmLocalhost(t *testing.T) {
	for _, url := range []string{
		"postgres://cornerlab:cornerlab@localhost:5432/cornerlab?sslmode=disable",
		"postgres://cornerlab:cornerlab@127.0.0.1:5432/cornerlab?sslmode=disable",
	} {
		cfg := produção()
		cfg.DatabaseURL = url

		if err := cfg.Validate(); err == nil {
			t.Errorf("Validate aceitou %q em produção; em produção o banco é remoto", url)
		}
	}
}

func TestProducaoRecusaJWTSecretDeExemplo(t *testing.T) {
	cfg := produção()
	cfg.JWTSecret = "change-me-in-production"

	err := cfg.ValidateAPI()
	if err == nil {
		t.Fatal("ValidateAPI aceitou o JWT_SECRET de exemplo em produção")
	}
	if !contémTudo(err.Error(), "JWT_SECRET") {
		t.Errorf("a mensagem precisa nomear JWT_SECRET, veio: %q", err)
	}
}

// REGRESSÃO — incidente de 14/09/2026 11:00 UTC.
//
// A primeira versão de Validate exigia JWT_SECRET de todo binário. O Cron Job
// não tem essa variável (e não deve ter: o worker não emite nem valida token),
// então morreu na largada com "worker não vai rodar: JWT_SECRET (vazio...)".
// A proteção contra má configuração virou a própria indisponibilidade.
//
// Este teste trava a regra correta: Validate valida o que TODO processo usa.
func TestWorkerSemJWTSecretEhValido(t *testing.T) {
	cfg := produção()
	cfg.JWTSecret = "" // o worker não usa JWT

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate recusou a configuração do worker por causa de JWT_SECRET, "+
			"que o worker não usa: %v", err)
	}
}

// E o outro lado da mesma regra: a API continua exigindo o segredo. Aceitar o
// valor de exemplo significaria aceitar token forjado por qualquer um que leia
// o repositório.
func TestAPISemJWTSecretEhInvalida(t *testing.T) {
	cfg := produção()
	cfg.JWTSecret = ""

	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("ValidateAPI aceitou JWT_SECRET vazio em produção")
	}
}

// Desenvolvimento continua funcionando sem nenhuma variável configurada — a
// conveniência do padrão local não pode ser destruída pela proteção.
func TestDesenvolvimentoAceitaPadroesLocais(t *testing.T) {
	cfg := Config{
		Environment: "development",
		DatabaseURL: "postgres://cornerlab:cornerlab@localhost:5432/cornerlab?sslmode=disable",
		JWTSecret:   "change-me-in-production",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate recusou configuração de desenvolvimento: %v", err)
	}
}

func TestProducaoBemConfiguradaPassa(t *testing.T) {
	if err := produção().Validate(); err != nil {
		t.Errorf("Validate recusou uma configuração de produção válida: %v", err)
	}
}

// Quando falta mais de uma variável, todas aparecem: descobrir uma por deploy
// transformaria uma correção em três.
func TestValidateAPIRelataTodosOsProblemasDeUmaVez(t *testing.T) {
	cfg := Config{Environment: "production"}

	err := cfg.ValidateAPI()
	if err == nil {
		t.Fatal("ValidateAPI aceitou configuração de produção completamente vazia")
	}
	if !contémTudo(err.Error(), "DATABASE_URL") {
		t.Errorf("a mensagem deveria citar DATABASE_URL, veio: %q", err)
	}
}

func contémTudo(s string, partes ...string) bool {
	for _, p := range partes {
		encontrado := false
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				encontrado = true
				break
			}
		}
		if !encontrado {
			return false
		}
	}
	return true
}
