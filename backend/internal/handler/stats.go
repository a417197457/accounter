package handler

import (
	"net/http"
	"strconv"

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

// Overview 账本总览（可选 start/end，YYYY-MM-DD；禁止 Gin 绑 time.Time）
func (h *StatsHandler) Overview(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	start, err := queryDate(c, "start", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start 格式错误，请使用 YYYY-MM-DD"})
		return
	}
	end, err := queryDate(c, "end", true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end 格式错误，请使用 YYYY-MM-DD"})
		return
	}
	data, err := h.service.Overview(ledgerID, start, end)
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
	start, err := queryDate(c, "start", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start 格式错误，请使用 YYYY-MM-DD"})
		return
	}
	end, err := queryDate(c, "end", true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end 格式错误，请使用 YYYY-MM-DD"})
		return
	}
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
	start, err := queryDate(c, "start", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start 格式错误，请使用 YYYY-MM-DD"})
		return
	}
	end, err := queryDate(c, "end", true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end 格式错误，请使用 YYYY-MM-DD"})
		return
	}
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
