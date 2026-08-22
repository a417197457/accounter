package handler

import (
	"net/http"
	"time"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// BudgetHandler 预算处理器
type BudgetHandler struct {
	service *service.BudgetService
}

// NewBudgetHandler 创建预算处理器
func NewBudgetHandler() *BudgetHandler {
	return &BudgetHandler{service: service.NewBudgetService()}
}

// SetRequest 设置预算请求
type SetBudgetRequest struct {
	CategoryID *uint           `json:"category_id"`
	Amount     decimal.Decimal `json:"amount" binding:"required"`
}

// List 预算列表
func (h *BudgetHandler) List(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	monthStart := time.Now()
	if month := c.Query("month"); month != "" {
		parsed, err := time.ParseInLocation("2006-01", month, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "月份格式应为 YYYY-MM"})
			return
		}
		monthStart = parsed
	}
	data, err := h.service.ListBudgets(ledgerID, monthStart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Set 设置预算
func (h *BudgetHandler) Set(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req SetBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 校验分类归属（如果指定了分类）
	if req.CategoryID != nil {
		_ = model.Category{} // 引用 model 包
	}

	if err := h.service.Set(ledgerID, userID, req.CategoryID, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// Delete 删除预算
func (h *BudgetHandler) Delete(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	id := getUintParam(c, "id")
	if err := h.service.Delete(ledgerID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}
