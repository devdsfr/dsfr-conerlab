package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/delivery/http/middleware"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// MyDataHandler exporta TUDO que o usuário criou dentro do CornerLab, num único
// arquivo JSON.
//
// É diferente dos outros exports (dashboard, comparador, ranking), que exportam
// ANÁLISE — números calculados pelo sistema sobre dados públicos de futebol.
// Aqui o conteúdo é do usuário: os filtros que ele montou, a banca que ele
// configurou, as apostas que ele registrou.
//
// POR QUE NÃO É RECURSO PREMIUM. O resto de /exports está atrás de assinatura
// porque exporta trabalho do sistema. Levar embora o que é seu não é um
// benefício que se vende — é o mínimo. Prender o dado do usuário dentro do
// produto como forma de retenção é hostil, e uma assinatura que vence não
// deveria significar perder o que você montou.
//
// JSON e não CSV/XLSX de propósito: são estruturas aninhadas e heterogêneas
// (banca tem fases, critérios, estado e histórico). Achatá-las em planilha
// perderia informação; JSON preserva e é reimportável.
type MyDataHandler struct {
	filters         repository.FilterRepository
	bets            repository.BetRepository
	bankroll        repository.BankrollRepository
	alerts          repository.AlertRuleRepository
	strategyHistory repository.StrategyHistoryRepository
}

func NewMyDataHandler(
	filters repository.FilterRepository,
	bets repository.BetRepository,
	bankroll repository.BankrollRepository,
	alerts repository.AlertRuleRepository,
	strategyHistory repository.StrategyHistoryRepository,
) *MyDataHandler {
	return &MyDataHandler{
		filters: filters, bets: bets, bankroll: bankroll,
		alerts: alerts, strategyHistory: strategyHistory,
	}
}

// bankrollExport agrupa as quatro peças do módulo de banca, que vivem em
// tabelas separadas mas só fazem sentido juntas.
type bankrollExport struct {
	Phases   []domain.BankrollPhase        `json:"fases"`
	Criteria domain.BankrollCriteria       `json:"criterios"`
	State    *domain.BankrollState         `json:"estado"`
	History  []domain.BankrollHistoryEntry `json:"historico"`
}

type myDataExport struct {
	ExportadoEm string `json:"exportado_em"`
	Aviso       string `json:"aviso"`

	FiltrosSalvos       []domain.SavedFilter          `json:"filtros_salvos"`
	Apostas             []domain.Bet                  `json:"apostas"`
	Banca               bankrollExport                `json:"gestao_de_banca"`
	Alertas             []domain.AlertRule            `json:"alertas"`
	HistoricoEstrategia []domain.StrategyHistoryEntry `json:"historico_de_estrategias"`
}

// Download godoc
// @Summary Baixar meus dados (JSON)
// @Description Exporta filtros salvos, apostas, gestão de banca, alertas e histórico de estratégias do usuário autenticado.
// @Tags export
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/exports/meus-dados [get]
func (h *MyDataHandler) Download(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	ctx := c.Request.Context()

	out := myDataExport{
		ExportadoEm: time.Now().Format(time.RFC3339),
		Aviso: "Dados pessoais exportados do CornerLab. Os números de análise aqui dentro " +
			"descrevem desempenho histórico e não constituem recomendação de aposta.",
	}

	// Uma seção que falha não pode derrubar o arquivo inteiro: quem pede os
	// próprios dados prefere recebê-los parciais a receber um erro 500. Cada
	// falha vira erro explícito no lugar do silêncio.
	var problemas []string

	filtros, err := h.filters.List(ctx, userID)
	if err != nil {
		problemas = append(problemas, "filtros_salvos: "+err.Error())
	}
	out.FiltrosSalvos = filtros

	apostas, err := h.bets.List(ctx, userID)
	if err != nil {
		problemas = append(problemas, "apostas: "+err.Error())
	}
	out.Apostas = apostas

	fases, err := h.bankroll.ListPhases(ctx, userID)
	if err != nil {
		problemas = append(problemas, "banca/fases: "+err.Error())
	}
	out.Banca.Phases = fases

	criterios, err := h.bankroll.GetCriteria(ctx, userID)
	if err != nil {
		problemas = append(problemas, "banca/criterios: "+err.Error())
	}
	out.Banca.Criteria = criterios

	estado, err := h.bankroll.GetState(ctx, userID)
	if err != nil {
		problemas = append(problemas, "banca/estado: "+err.Error())
	}
	out.Banca.State = estado

	historicoBanca, err := h.bankroll.ListHistory(ctx, userID)
	if err != nil {
		problemas = append(problemas, "banca/historico: "+err.Error())
	}
	out.Banca.History = historicoBanca

	alertas, err := h.alerts.List(ctx, userID)
	if err != nil {
		problemas = append(problemas, "alertas: "+err.Error())
	}
	out.Alertas = alertas

	historicoEstrategias, err := h.strategyHistory.List(ctx, userID)
	if err != nil {
		problemas = append(problemas, "historico_de_estrategias: "+err.Error())
	}
	out.HistoricoEstrategia = historicoEstrategias

	if len(problemas) > 0 {
		out.Aviso += fmt.Sprintf(" ATENÇÃO: %d seção(ões) não puderam ser lidas: %v.",
			len(problemas), problemas)
	}

	nome := fmt.Sprintf("cornerlab-meus-dados-%s.json", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", "attachment; filename="+nome)
	c.JSON(http.StatusOK, out)
}
