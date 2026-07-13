package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/intelligence"
	"github.com/devdsfr/cornerlab/pkg/export"
)

type ExportHandler struct {
	dashboard  *usecase.DashboardUsecase
	comparator *usecase.ComparatorUsecase
	filters    *usecase.FilterUsecase
	ranking    *intelligence.RankingUsecase
}

func NewExportHandler(dashboard *usecase.DashboardUsecase, comparator *usecase.ComparatorUsecase, filters *usecase.FilterUsecase, ranking *intelligence.RankingUsecase) *ExportHandler {
	return &ExportHandler{dashboard: dashboard, comparator: comparator, filters: filters, ranking: ranking}
}

// writeTable escreve a tabela no formato solicitado (?format=csv|xlsx, padrão csv).
func writeTable(c *gin.Context, filename string, t export.Table) {
	format := c.DefaultQuery("format", "csv")
	switch format {
	case "xlsx":
		data, err := export.ToXLSX(t)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename+".xlsx")
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
	default:
		data, err := export.ToCSV(t)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename+".csv")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
	}
}

// DashboardCSV godoc
// @Summary Exportar Dashboard (CSV/Excel)
// @Tags export
// @Param team_id query int true "ID da equipe"
// @Param league_id query int false "ID do campeonato"
// @Param season_id query int false "ID da temporada"
// @Param limit query int false "Quantidade de jogos"
// @Param format query string false "csv (padrão) ou xlsx"
// @Router /api/v1/exports/dashboard [get]
func (h *ExportHandler) DashboardCSV(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório"})
		return
	}
	var leagueID, seasonID *int64
	if v := c.Query("league_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		leagueID = &id
	}
	if v := c.Query("season_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		seasonID = &id
	}
	limit := 10
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			limit = l
		}
	}

	result, err := h.dashboard.GetDashboard(c.Request.Context(), teamID, leagueID, seasonID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	rows := make([][]string, 0, len(result.RecentMatches))
	for _, m := range result.RecentMatches {
		mando := "Fora"
		if m.IsHome {
			mando = "Casa"
		}
		rows = append(rows, []string{
			m.MatchDate.Format("2006-01-02"),
			m.Opponent.Name,
			mando,
			strconv.Itoa(m.CornersFor),
			strconv.Itoa(m.CornersAgainst),
			strconv.Itoa(m.TotalCorners),
		})
	}

	writeTable(c, "dashboard_"+sanitizeFilename(result.Team.Name), export.Table{
		SheetName: "Dashboard",
		Headers:   []string{"Data", "Adversário", "Mando", "Escanteios a Favor", "Escanteios Sofridos", "Total"},
		Rows:      rows,
	})
}

// ComparatorCSV godoc
// @Summary Exportar Comparador (CSV/Excel)
// @Tags export
// @Router /api/v1/exports/comparator [get]
func (h *ExportHandler) ComparatorCSV(c *gin.Context) {
	teamA, err := strconv.ParseInt(c.Query("team_a"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_a é obrigatório"})
		return
	}
	teamB, err := strconv.ParseInt(c.Query("team_b"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_b é obrigatório"})
		return
	}
	var leagueID *int64
	if v := c.Query("league_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		leagueID = &id
	}
	limit := 10
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			limit = l
		}
	}

	result, err := h.comparator.Compare(c.Request.Context(), teamA, teamB, leagueID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	rows := [][]string{
		{result.TeamA.Team.Name, "Total (média)", floatStr(result.TeamA.TotalCorners.Mean)},
		{result.TeamA.Team.Name, "A favor (média)", floatStr(result.TeamA.CornersFor.Mean)},
		{result.TeamA.Team.Name, "Sofridos (média)", floatStr(result.TeamA.CornersAgainst.Mean)},
		{result.TeamB.Team.Name, "Total (média)", floatStr(result.TeamB.TotalCorners.Mean)},
		{result.TeamB.Team.Name, "A favor (média)", floatStr(result.TeamB.CornersFor.Mean)},
		{result.TeamB.Team.Name, "Sofridos (média)", floatStr(result.TeamB.CornersAgainst.Mean)},
	}

	writeTable(c, "comparador_"+sanitizeFilename(result.TeamA.Team.Name)+"_vs_"+sanitizeFilename(result.TeamB.Team.Name), export.Table{
		SheetName: "Comparador",
		Headers:   []string{"Equipe", "Métrica", "Valor"},
		Rows:      rows,
	})
}

// FilterRunCSV godoc
// @Summary Exportar resultado do Simulador de Filtros (CSV/Excel)
// @Tags export
// @Router /api/v1/exports/filters/run [get]
func (h *ExportHandler) FilterRunCSV(c *gin.Context) {
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	var seasonIDs []int64
	if v := c.Query("season_ids"); v != "" {
		for _, s := range strings.Split(v, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err == nil {
				seasonIDs = append(seasonIDs, id)
			}
		}
	}
	cornersThreshold, err := strconv.Atoi(c.Query("corners_threshold"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corners_threshold é obrigatório"})
		return
	}

	criteria := usecase.FilterCriteria{
		HomeAway:         c.Query("home_away"),
		CornersThreshold: cornersThreshold,
		OpponentTier:     c.Query("opponent_tier"),
	}
	if v := c.Query("team_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		criteria.TeamID = &id
	}
	if v := c.Query("last_n_games"); v != "" {
		n, _ := strconv.Atoi(v)
		criteria.LastNGames = n
	}
	if v := c.Query("max_odds"); v != "" {
		odds, _ := strconv.ParseFloat(v, 64)
		criteria.MaxOdds = odds
	}
	if v := c.Query("stake"); v != "" {
		stake, _ := strconv.ParseFloat(v, 64)
		criteria.Stake = stake
	}

	result, err := h.filters.RunBacktest(c.Request.Context(), leagueID, seasonIDs, criteria)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows := make([][]string, 0, len(result.Entries))
	for _, e := range result.Entries {
		mando := "Fora"
		if e.IsHome {
			mando = "Casa"
		}
		hit := "Erro"
		if e.Hit {
			hit = "Acerto"
		}
		rows = append(rows, []string{
			e.MatchDate, e.Team, e.Opponent, mando,
			strconv.Itoa(e.TotalCorners), hit, floatStr(e.Odd), floatStr(e.ProfitLoss),
		})
	}

	writeTable(c, "backtest", export.Table{
		SheetName: "Backtest",
		Headers:   []string{"Data", "Equipe", "Adversário", "Mando", "Escanteios", "Resultado", "Odd", "Resultado (stake)"},
		Rows:      rows,
	})
}

// RankingCSV godoc
// @Summary Exportar Ranking (CSV/Excel)
// @Tags export
// @Router /api/v1/exports/ranking [get]
func (h *ExportHandler) RankingCSV(c *gin.Context) {
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	metric := intelligence.RankingMetric(c.DefaultQuery("metric", string(intelligence.MetricAverageCorners)))
	limit := 10
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			limit = l
		}
	}
	topN := 10
	if v := c.Query("top"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			topN = n
		}
	}

	result, err := h.ranking.Compute(c.Request.Context(), leagueID, nil, metric, limit, topN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows := make([][]string, 0, len(result.Entries))
	for _, e := range result.Entries {
		rows = append(rows, []string{strconv.Itoa(e.Rank), e.TeamName, floatStr(e.Value)})
	}

	writeTable(c, "ranking_"+string(metric), export.Table{
		SheetName: "Ranking",
		Headers:   []string{"Posição", "Equipe", "Valor"},
		Rows:      rows,
	})
}

func floatStr(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "-")
	return replacer.Replace(s)
}
