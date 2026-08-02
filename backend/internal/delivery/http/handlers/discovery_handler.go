package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase/discovery"
)

// DiscoveryHandler expõe o Strategy Discovery Engine (Remodelagem F6, doc 08).
//
// Divisão de responsabilidade igual à do resto do pipeline (regra do doc 15:
// nada pesado dentro de requisição HTTP):
//
//   - GET /discovery/strategies — leitura do ranking já calculado. Público, como
//     Dashboard/Comparador/Intelligence: consultar estatística não exige login.
//   - POST /discovery/run — dispara uma varredura completa AGORA. Centenas de
//     backtests por liga, por isso fica atrás de login (ver router). O caminho
//     normal é o Discovery Worker noturno (cmd/worker); este endpoint existe para
//     não obrigar o usuário a esperar o próximo ciclo depois de sincronizar dados.
type DiscoveryHandler struct {
	strategies repository.StrategyRepository
	engine     *discovery.Engine
}

func NewDiscoveryHandler(strategies repository.StrategyRepository, engine *discovery.Engine) *DiscoveryHandler {
	return &DiscoveryHandler{strategies: strategies, engine: engine}
}

// discoveredItem é a linha do ranking exibida no doc 08 ("Dashboard": Score, ROI,
// Yield, EV, Jogos, Lucro, Drawdown, Confiabilidade), já achatada para o frontend
// não precisar navegar por três objetos aninhados nem repetir regra de negócio.
type discoveredItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Definition  any    `json:"definition"`

	Games    int     `json:"games"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	WinRate  float64 `json:"win_rate"`
	ROI      float64 `json:"roi"`
	Yield    float64 `json:"yield"`
	EV       float64 `json:"ev"`
	Profit   float64 `json:"profit"`
	Drawdown float64 `json:"drawdown"`

	DSFRScore      float64 `json:"dsfr_score"`
	Classification string  `json:"classification"`
	Confidence     float64 `json:"confidence"`
	Robustness     float64 `json:"robustness"`
	Risk           float64 `json:"risk"`
	HealthScore    float64 `json:"health_score"`
	LifecycleStage string  `json:"lifecycle_stage"`

	UpdatedAt string `json:"updated_at"`
}

// ListStrategies godoc
// @Summary Ranking de estratégias descobertas automaticamente
// @Tags discovery
// @Produce json
// @Param league_id query int false "Filtrar por campeonato"
// @Param limit query int false "Máximo de itens (padrão 50)"
// @Success 200 {object} object{strategies=[]discoveredItem,count=int}
// @Router /api/v1/discovery/strategies [get]
func (h *DiscoveryHandler) ListStrategies(c *gin.Context) {
	var leagueID *int64
	if v := c.Query("league_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "league_id inválido"})
			return
		}
		leagueID = &id
	}

	list, err := h.strategies.ListDiscovered(c.Request.Context(), leagueID, queryInt(c, "limit", 50))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]discoveredItem, 0, len(list))
	for _, d := range list {
		items = append(items, toDiscoveredItem(d))
	}

	c.JSON(http.StatusOK, gin.H{
		"strategies": items,
		"count":      len(items),
		"disclaimer": "Padrões identificados por mineração de dados históricos. " +
			"Não constituem recomendação de aposta nem previsão de resultados futuros.",
	})
}

func toDiscoveredItem(d repository.DiscoveredStrategy) discoveredItem {
	item := discoveredItem{
		ID:          d.Strategy.ID,
		Name:        d.Strategy.Name,
		Description: d.Strategy.Description,
		UpdatedAt:   d.Strategy.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// A definição é devolvida como objeto (não string) para o frontend poder
	// carregá-la direto no Simulador de Filtros e reexecutar a descoberta.
	var def any
	if err := json.Unmarshal([]byte(d.Strategy.Definition), &def); err == nil {
		item.Definition = def
	}

	if b := d.Backtest; b != nil {
		item.Games, item.Wins, item.Losses = b.Games, b.Wins, b.Losses
		if b.Games > 0 {
			item.WinRate = round1(100 * float64(b.Wins) / float64(b.Games))
		}
		item.ROI = derefFloat(b.ROI)
		item.Yield = derefFloat(b.Yield)
		item.EV = derefFloat(b.EV)
		item.Profit = derefFloat(b.Profit)
		item.Drawdown = derefFloat(b.Drawdown)
	}
	if s := d.Scores; s != nil {
		item.DSFRScore = s.DSFRScore
		item.Classification = string(discovery.Classify(s.DSFRScore))
		item.Confidence = derefFloat(s.Confidence)
		item.Robustness = derefFloat(s.Robustness)
		item.Risk = derefFloat(s.Risk)
		item.LifecycleStage = s.LifecycleStage
	}
	if hl := d.Health; hl != nil {
		item.HealthScore = hl.HealthScore
	}
	return item
}

type discoveryRunRequest struct {
	// LeagueID 0 = varrer todas as ligas cadastradas.
	LeagueID  int64   `json:"league_id"`
	SeasonIDs []int64 `json:"season_ids"`
}

// Run godoc
// @Summary Executar agora um ciclo de descoberta de estratégias
// @Tags discovery
// @Accept json
// @Produce json
// @Router /api/v1/discovery/run [post]
func (h *DiscoveryHandler) Run(c *gin.Context) {
	var req discoveryRunRequest
	// Corpo vazio é válido e significa "todas as ligas, todas as temporadas".
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	ctx := c.Request.Context()
	if req.LeagueID > 0 {
		result, err := h.engine.RunLeague(ctx, req.LeagueID, req.SeasonIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	result, err := h.engine.RunAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
