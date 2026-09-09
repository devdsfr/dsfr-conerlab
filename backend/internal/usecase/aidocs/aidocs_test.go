package aidocs

import (
	"strings"
	"testing"
)

func TestMarkdownGeraDocumentoValido(t *testing.T) {
	md := Markdown([]Route{{Method: "GET", Path: "/api/v1/leagues"}})

	// Verbo de formatação não substituído indica erro na ordem dos argumentos.
	for _, ruim := range []string{"%!", "%s", "%d", "%.0f", "(MISSING)", "EXTRA"} {
		if strings.Contains(md, ruim) {
			t.Fatalf("documento contem marcador de formatacao nao resolvido: %q", ruim)
		}
	}
	for _, esperado := range []string{
		"# CornerLab", "Regras inegociáveis", "DSFR Score", "/api/v1/leagues",
		"1 endpoints publicados", "Nunca recomende aposta",
	} {
		if !strings.Contains(md, esperado) {
			t.Errorf("faltou no documento: %q", esperado)
		}
	}
}
