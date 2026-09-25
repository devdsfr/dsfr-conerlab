package handlers

import "testing"

// REV-P3 / correção 5 — proveniência ao salvar do Simulador.
//
// O que está sendo protegido: uma estratégia salva do Simulador é um recorte que
// o usuário montou à mão, SEM holdout temporal e SEM correção de múltiplas
// comparações. Se ela for gravada como "discovery", a lista de Estratégias passa
// a exibi-la como se tivesse passado por essa validação. Salvar não valida.
func TestOrigemPermitida(t *testing.T) {
	casos := []struct {
		entrada string
		quer    string
		porque  string
	}{
		{"simulator", "simulator", "o Simulador declara a própria procedência"},
		{"", "user", "sem declaração, é criação direta do usuário"},
		{"user", "user", "criação direta continua sendo user"},
		{"discovery", "user", "NINGUÉM reivindica 'discovery' por requisição: " +
			"esse valor significa 'passou pelo Discovery Engine' e só o motor o grava"},
		{"validated", "user", "valor desconhecido não pode virar promoção"},
		{"approved", "user", "idem — salvar nunca aprova"},
	}
	for _, c := range casos {
		if got := origemPermitida(c.entrada); got != c.quer {
			t.Errorf("origemPermitida(%q) = %q, esperado %q — %s", c.entrada, got, c.quer, c.porque)
		}
	}
}
