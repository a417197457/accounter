package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// TransactionHandler 流水处理器
type TransactionHandler struct {
	service *service.TransactionService
}

// NewTransactionHandler 创建流水处理器
func NewTransactionHandler() *TransactionHandler {
	return &TransactionHandler{
		service: service.NewTransactionService(),
	}
}

// CreateRequest 创建流水请求
type CreateTransactionRequest struct {
	AccountID   uint            `json:"account_id" binding:"required"`
	ToAccountID *uint           `json:"to_account_id,omitempty"`
	CategoryID  *uint           `json:"category_id,omitempty"`
	OpType      string          `json:"op_type" binding:"required,oneof=expense income borrow_in borrow_out transfer"`
	Amount      decimal.Decimal `json:"amount" binding:"required"`
	Remark      string          `json:"remark" binding:"max=255"`
	OccurredAt  string          `json:"occurred_at" binding:"required"`
}

// UpdateRequest 更新流水请求
type UpdateTransactionRequest struct {
	AccountID   uint            `json:"account_id" binding:"required"`
	ToAccountID *uint           `json:"to_account_id,omitempty"`
	CategoryID  *uint           `json:"category_id,omitempty"`
	OpType      string          `json:"op_type" binding:"required,oneof=expense income borrow_in borrow_out transfer"`
	Amount      decimal.Decimal `json:"amount" binding:"required"`
	Remark      string          `json:"remark" binding:"max=255"`
	OccurredAt  string          `json:"occurred_at" binding:"required"`
}

// parseTime 解析时间字符串
func parseTime(s string) (time.Time, error) {
	// 尝试多种格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("时间格式错误，请使用 2006-01-02 或 2006-01-02 15:04:05")
}

// Create 创建流水
func (h *TransactionHandler) Create(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	occurredAt, err := parseTime(req.OccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 纯日期补成 12:00（避免时区问题）
	if req.OccurredAt == occurredAt.Format("2006-01-02") {
		occurredAt = time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 12, 0, 0, 0, time.Local)
	}

	t := &model.Transaction{
		AccountID:   req.AccountID,
		ToAccountID: req.ToAccountID,
		CategoryID:  req.CategoryID,
		OpType:      req.OpType,
		Amount:      req.Amount,
		Remark:      req.Remark,
		OccurredAt:  occurredAt,
	}

	t, err = h.service.Create(ledgerID, userID, t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": t})
}

// Update 更新流水
func (h *TransactionHandler) Update(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	var req UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	occurredAt, err := parseTime(req.OccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 纯日期补成 12:00
	if req.OccurredAt == occurredAt.Format("2006-01-02") {
		occurredAt = time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 12, 0, 0, 0, time.Local)
	}

	next := &model.Transaction{
		AccountID:   req.AccountID,
		ToAccountID: req.ToAccountID,
		CategoryID:  req.CategoryID,
		OpType:      req.OpType,
		Amount:      req.Amount,
		Remark:      req.Remark,
		OccurredAt:  occurredAt,
	}

	t, err := h.service.Update(ledgerID, userID, id, next)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": t})
}

// Delete 删除流水
func (h *TransactionHandler) Delete(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	if err := h.service.Delete(ledgerID, userID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// List 获取流水列表
// 注意：不要用 ShouldBindQuery 绑定 start/end 到 time.Time，
// Gin 默认按 RFC3339 解析，YYYY-MM-DD 会直接报错。
func (h *TransactionHandler) List(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	req := service.ListRequest{
		OpType:   c.Query("op_type"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}

	if start, err := queryDate(c, "start", false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start 格式错误，请使用 YYYY-MM-DD"})
		return
	} else {
		req.Start = start
	}
	if end, err := queryDate(c, "end", true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end 格式错误，请使用 YYYY-MM-DD"})
		return
	} else {
		req.End = end
	}
	if s := c.Query("month"); s != "" {
		t, err := time.ParseInLocation("2006-01", s, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month 格式错误，请使用 YYYY-MM"})
			return
		}
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		req.Start = &start
		req.End = &end
	}

	if s := c.Query("account_id"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account_id 格式错误"})
			return
		}
		vUint := uint(v)
		req.AccountID = &vUint
	}
	if s := c.Query("category_id"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id 格式错误"})
			return
		}
		vUint := uint(v)
		req.CategoryID = &vUint
	}
	if s := c.Query("member_id"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "member_id 格式错误"})
			return
		}
		vUint := uint(v)
		req.MemberID = &vUint
	}

	transactions, total, err := h.service.List(ledgerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"list":      transactions,
			"total":     total,
			"page":      req.Page,
			"page_size": req.PageSize,
		},
	})
}

// Get 获取单条流水
func (h *TransactionHandler) Get(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	id := getUintParam(c, "id")

	t, err := h.service.Get(ledgerID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": t})
}
