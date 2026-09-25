// Package adminaccess define quem é ADMINISTRADOR da plataforma.
//
// REV-P4 (B4): o botão "Procurar agora" do Discovery disparava a mineração
// completa — que publica e desativa estratégias PÚBLICAS — para qualquer
// usuário autenticado. A decisão de produto é: só administrador dispara.
// Usuários comuns continuam consumindo os resultados.
//
// O sistema não tinha papel de administrador. O mecanismo mínimo e auditável é
// uma lista de e-mails configurada por variável de ambiente (ADMIN_EMAILS),
// no mesmo padrão de pkg/devaccess. A checagem é feita NO BACKEND
// (middleware.RequireAdmin); esconder o botão no frontend é conveniência, não
// controle de segurança.
//
// Falha fechada: lista vazia = NINGUÉM é administrador.
package adminaccess

import "strings"

var admins = map[string]bool{}

// Configure define a lista de e-mails administradores. Chamado uma vez na
// inicialização da API (cmd/api/main.go), a partir de cfg.AdminEmails.
func Configure(emails []string) {
	admins = make(map[string]bool, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			admins[e] = true
		}
	}
}

// IsAdmin indica se o e-mail está na lista de administradores. Comparação sem
// diferenciar maiúsculas e ignorando espaços, como no cadastro.
func IsAdmin(email string) bool {
	return admins[strings.ToLower(strings.TrimSpace(email))]
}

// Count devolve quantos administradores estão configurados — usado no log de
// inicialização para deixar visível quando a lista está vazia.
func Count() int { return len(admins) }
