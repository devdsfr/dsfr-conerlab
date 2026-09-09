package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/usecase/aidocs"
)

// AIDocsHandler serve o documento de contexto do CornerLab em Markdown, baixado
// pelo botão do painel Integrações para alimentar outra IA.
//
// O documento é VIVO: nada aqui é texto fixo copiado à mão. Os limites e pesos
// saem das constantes que o motor usa em produção, e a lista de endpoints é lida
// do próprio router no momento da requisição. Consequência prática: mudar um peso
// do DSFR ou publicar uma rota nova já altera o documento no mesmo deploy, sem
// ninguém precisar lembrar de atualizar nada.
//
// Público de propósito: descreve como a plataforma funciona e quais regras qualquer
// consumidor deve respeitar. Não expõe chave, dado de usuário nem nada sensível.
type AIDocsHandler struct {
	// routes é resolvido a cada requisição (e não guardado) para refletir o
	// router realmente montado, inclusive rotas registradas depois deste handler.
	routes func() gin.RoutesInfo
}

func NewAIDocsHandler() *AIDocsHandler { return &AIDocsHandler{} }

// UseRouter liga o handler ao engine já montado. Chamado no fim de NewRouter,
// quando todas as rotas existem.
func (h *AIDocsHandler) UseRouter(r *gin.Engine) {
	h.routes = r.Routes
}

// Download godoc
// @Summary Documento de contexto do CornerLab em Markdown (para integrar com outra IA)
// @Tags docs
// @Produce text/markdown
// @Router /api/v1/docs/contexto.md [get]
func (h *AIDocsHandler) Download(c *gin.Context) {
	var rotas []aidocs.Route
	if h.routes != nil {
		for _, info := range h.routes() {
			rotas = append(rotas, aidocs.Route{Method: info.Method, Path: info.Path})
		}
	}

	md := aidocs.Markdown(rotas)

	// Content-Disposition attachment faz o navegador baixar em vez de renderizar,
	// já com o nome de arquivo certo.
	c.Header("Content-Disposition", `attachment; filename="`+aidocs.Filename+`"`)
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
}
