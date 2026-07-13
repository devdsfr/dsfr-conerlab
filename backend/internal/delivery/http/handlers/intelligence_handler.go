package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/devdsfr/cornerlab/internal/usecase/intelligence"
	"github.com/devdsfr/cornerlab/pkg/cache"
)

type IntelligenceHandler struct {
	consistency *intelligence.ConsistencyUsecase
	trend       *intelligence.TrendUsecase
	stability   *intelligence.StabilityUsecase
	score       *intelligence.ScoreUsecase
	compat      *intelligence.CompatibilityUsecase
	opponent    *intelligence.OpponentUsecase
	ranking     *intelligence.RankingUsecase
	executive   *intelligence.ExecutiveDashboardUsecase
	explain     *intelligence.ExplainUsecase

	redis    *redis.Client
	cacheTTL time.Duration
}

func NewIntelligenceHandler(
	consistency *intelligence.ConsistencyUsecase,
	trend *intelligence.TrendUsecase,
	stability *intelligence.StabilityUsecase,
	score *intelligence.ScoreUsecase,
	compat *intelligence.CompatibilityUsecase,
	opponent *intelligence.OpponentUsecase,
	ranking *intelligence.RankingUsecase,
	executive *intelligence.ExecutiveDashboardUsecase,
	explain *intelligence.ExplainUsecase,
	redisClient *redis.Client,
	cacheTTL time.Duration,
) *IntelligenceHandler {
	return &IntelligenceHandler{
		consistency: consistency, trend: trend, stability: stability, score: score,
		compat: compat, opponent: opponent, ranking: ranking, executive: executive, explain: explain,
		redis: redisClient, cacheTTL: cacheTTL,
	}
}

// withCache tenta servir `out` do Redis; em caso de miss, executa `compute`, grava no
// cache e responde. `markCached` é chamado para marcar Meta.Cached=true no objeto
// deserializado do cache (cada usecase expõe seu próprio Meta).
func withCache[T any](c *gin.Context, h *IntelligenceHandler, key string, markCached func(*T), compute func(ctx context.Context) (*T, error)) {
	ctx := c.Request.Context()

	if h.redis != nil {
		var cached T
		if hit, err := cache.GetJSON(ctx, h.redis, key, &cached); err == nil && hit {
			if markCached != nil {
				markCached(&cached)
			}
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	result, err := compute(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.redis != nil {
		_ = cache.SetJSON(ctx, h.redis, key, h.cacheTTL, result)
	}
	c.JSON(http.StatusOK, result)
}

func (h *IntelligenceHandler) Consistency(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório"})
		return
	}
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 10)
	var seasonID *int64
	if v := c.Query("season_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		seasonID = &id
	}

	key := fmt.Sprintf("intel:consistency:%d:%d:%v:%d", teamID, leagueID, seasonID, limit)
	withCache(c, h, key, func(r *intelligence.ConsistencyReport) { r.Meta.Cached = true }, func(ctx context.Context) (*intelligence.ConsistencyReport, error) {
		return h.consistency.Compute(ctx, teamID, leagueID, seasonID, limit)
	})
}

func (h *IntelligenceHandler) Trend(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório"})
		return
	}
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	short := queryInt(c, "short_window", 5)
	long := queryInt(c, "long_window", 10)

	key := fmt.Sprintf("intel:trend:%d:%d:%d:%d", teamID, leagueID, short, long)
	withCache(c, h, key, func(r *intelligence.TrendReport) { r.Meta.Cached = true }, func(ctx context.Context) (*intelligence.TrendReport, error) {
		return h.trend.Compute(ctx, teamID, leagueID, short, long)
	})
}

func (h *IntelligenceHandler) Stability(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório"})
		return
	}
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 10)

	key := fmt.Sprintf("intel:stability:%d:%d:%d", teamID, leagueID, limit)
	withCache(c, h, key, func(r *intelligence.StabilityReport) { r.Meta.Cached = true }, func(ctx context.Context) (*intelligence.StabilityReport, error) {
		return h.stability.Compute(ctx, teamID, leagueID, limit)
	})
}

func (h *IntelligenceHandler) Score(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("team_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id é obrigatório"})
		return
	}
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 10)
	var opponentID *int64
	if v := c.Query("opponent_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		opponentID = &id
	}

	key := fmt.Sprintf("intel:score:%d:%d:%v:%d", teamID, leagueID, opponentID, limit)
	withCache(c, h, key, func(r *intelligence.ScoreReport) {}, func(ctx context.Context) (*intelligence.ScoreReport, error) {
		return h.score.Compute(ctx, teamID, leagueID, opponentID, limit)
	})
}

func (h *IntelligenceHandler) Compatibility(c *gin.Context) {
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
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 10)

	key := fmt.Sprintf("intel:compat:%d:%d:%d:%d", teamA, teamB, leagueID, limit)
	withCache(c, h, key, func(r *intelligence.CompatibilityReport) {}, func(ctx context.Context) (*intelligence.CompatibilityReport, error) {
		return h.compat.Compute(ctx, teamA, teamB, leagueID, limit)
	})
}

func (h *IntelligenceHandler) Opponent(c *gin.Context) {
	opponentID, err := strconv.ParseInt(c.Query("opponent_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "opponent_id é obrigatório"})
		return
	}
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 0)

	key := fmt.Sprintf("intel:opponent:%d:%d:%d", opponentID, leagueID, limit)
	withCache(c, h, key, func(r *intelligence.OpponentReport) {}, func(ctx context.Context) (*intelligence.OpponentReport, error) {
		return h.opponent.Compute(ctx, opponentID, leagueID, nil, limit)
	})
}

func (h *IntelligenceHandler) Ranking(c *gin.Context) {
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	metric := intelligence.RankingMetric(c.DefaultQuery("metric", string(intelligence.MetricAverageCorners)))
	limit := queryInt(c, "limit", 10)
	topN := queryInt(c, "top", 10)

	key := fmt.Sprintf("intel:ranking:%d:%s:%d:%d", leagueID, metric, limit, topN)
	withCache(c, h, key, func(r *intelligence.RankingResult) {}, func(ctx context.Context) (*intelligence.RankingResult, error) {
		return h.ranking.Compute(ctx, leagueID, nil, metric, limit, topN)
	})
}

func (h *IntelligenceHandler) ExecutiveDashboard(c *gin.Context) {
	leagueID, err := strconv.ParseInt(c.Query("league_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "league_id é obrigatório"})
		return
	}
	limit := queryInt(c, "limit", 10)

	key := fmt.Sprintf("intel:executive:%d:%d", leagueID, limit)
	withCache(c, h, key, func(r *intelligence.ExecutiveDashboard) {}, func(ctx context.Context) (*intelligence.ExecutiveDashboard, error) {
		return h.executive.Compute(ctx, leagueID, nil, limit)
	})
}

type explainRequest struct {
	TeamID     int64  `json:"team_id" binding:"required"`
	LeagueID   int64  `json:"league_id" binding:"required"`
	OpponentID *int64 `json:"opponent_id"`
	Question   string `json:"question"`
	Limit      int    `json:"limit"`
}

// Explain godoc
// @Summary Explicação em linguagem analítica (IA restrita a dados armazenados)
// @Tags intelligence
// @Router /api/v1/intelligence/explain [post]
func (h *IntelligenceHandler) Explain(c *gin.Context) {
	var req explainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.explain.Explain(c.Request.Context(), req.TeamID, req.LeagueID, req.OpponentID, req.Question, req.Limit)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func queryInt(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
