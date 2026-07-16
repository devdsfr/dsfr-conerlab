// Package devaccess implementa uma liberação manual e temporária de acesso Premium
// para e-mails específicos, sem depender do Stripe — útil para o próprio time
// testar/demonstrar as telas premium antes de a assinatura real estar configurada
// (ver DEV_PREMIUM_EMAILS em .env.example). Não deve ser usado para conceder acesso
// a clientes reais; é puramente uma ferramenta de desenvolvimento/QA.
package devaccess

import "strings"

var allowlist = map[string]bool{}

// Configure define a lista de e-mails com acesso Premium liberado manualmente.
// Chamado uma única vez, na inicialização (cmd/api/main.go), a partir de
// cfg.DevPremiumEmails.
func Configure(emails []string) {
	allowlist = make(map[string]bool, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			allowlist[e] = true
		}
	}
}

// IsPremium indica se o e-mail está na lista de liberação manual.
func IsPremium(email string) bool {
	return allowlist[strings.ToLower(strings.TrimSpace(email))]
}
