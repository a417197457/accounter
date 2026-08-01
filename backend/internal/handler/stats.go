package handler

import (
	"net/http"
	"strconv"
	"time"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// StatsHandler 统计处理器
type StatsHandler struct {
	service *service.StatsService
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler() *StatsHandler {
	return &StatsHandler{service: service.NewStatsService()}
}

// parseTimeParam 解析时间参数
func parseTimeParam(c *gin.Context, key string) *time.Time {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return nil
	}
	// end 补成当天 23:59:59
	if key == "end" {
		t = t.Add(24*time.Hour - time.Second)
	}
	return &t
}

// Overview 账本总览
func (h *StatsHandler) Overview(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	data, err := h.service.Overview(ledgerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Trend 收支趋势
func (h *StatsHandler) Trend(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	granularity := c.DefaultQuery("granularity", "day")
	if granularity != "day" && granularity != "month" {
		granularity = "day"
	}
	start := parseTimeParam(c, "start")
	end := parseTimeParam(c, "end")
	data, err := h.service.Trend(ledgerID, granularity, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Category 分类占比
func (h *StatsHandler) Category(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	kind := c.Query("kind")
	start := parseTimeParam(c, "start")
	end := parseTimeParam(c, "end")
	data, err := h.service.CategoryBreakdown(ledgerID, kind, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Monthly 月度对比
func (h *StatsHandler) Monthly(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	months, _ := strconv.Atoi(c.DefaultQuery("months", "6"))
	if months <= 0 || months > 24 {
		months = 6
	}
	data, err := h.service.Monthly(ledgerID, months)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
