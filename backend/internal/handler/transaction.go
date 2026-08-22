package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/observability"
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
	AccountID        uint            `json:"account_id" binding:"required"`
	PaymentAccountID *uint           `json:"payment_account_id,omitempty"`
	ToAccountID      *uint           `json:"to_account_id,omitempty"`
	CategoryID       *uint           `json:"category_id,omitempty"`
	OpType           string          `json:"op_type" binding:"required,oneof=expense income borrow_in borrow_out transfer"`
	Amount           decimal.Decimal `json:"amount" binding:"required"`
	Remark           string          `json:"remark" binding:"max=255"`
	OccurredAt       string          `json:"occurred_at" binding:"required"`
}

// UpdateRequest 更新流水请求
type UpdateTransactionRequest struct {
	AccountID        uint            `json:"account_id" binding:"required"`
	PaymentAccountID *uint           `json:"payment_account_id,omitempty"`
	ToAccountID      *uint           `json:"to_account_id,omitempty"`
	CategoryID       *uint           `json:"category_id,omitempty"`
	OpType           string          `json:"op_type" binding:"required,oneof=expense income borrow_in borrow_out transfer"`
	Amount           decimal.Decimal `json:"amount" binding:"required"`
	Remark           string          `json:"remark" binding:"max=255"`
	OccurredAt       string          `json:"occurred_at" binding:"required"`
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
		AccountID:        req.AccountID,
		PaymentAccountID: req.PaymentAccountID,
		ToAccountID:      req.ToAccountID,
		CategoryID:       req.CategoryID,
		OpType:           req.OpType,
		Amount:           req.Amount,
		Remark:           req.Remark,
		OccurredAt:       occurredAt,
	}

	t, err = h.service.Create(ledgerID, userID, t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transaction_created",
		"transaction_id", t.ID,
		"source", "manual",
		"op_type", t.OpType,
		"amount", t.Amount.String(),
		"account_id", t.AccountID,
		"occurred_at", t.OccurredAt.Format(time.RFC3339),
	)

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
		AccountID:        req.AccountID,
		PaymentAccountID: req.PaymentAccountID,
		ToAccountID:      req.ToAccountID,
		CategoryID:       req.CategoryID,
		OpType:           req.OpType,
		Amount:           req.Amount,
		Remark:           req.Remark,
		OccurredAt:       occurredAt,
	}

	t, err := h.service.Update(ledgerID, userID, id, next)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transaction_updated",
		"transaction_id", t.ID,
		"op_type", t.OpType,
		"amount", t.Amount.String(),
		"account_id", t.AccountID,
		"occurred_at", t.OccurredAt.Format(time.RFC3339),
	)

	c.JSON(http.StatusOK, gin.H{"data": t})
}

// Delete 删除流水
func (h *TransactionHandler) Delete(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")
	existing, _ := h.service.Get(ledgerID, id)

	if err := h.service.Delete(ledgerID, userID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isSyncedCopy := existing != nil && existing.SourceID != nil
	sourceTransactionID := uint(0)
	if isSyncedCopy {
		sourceTransactionID = *existing.SourceID
	}
	observability.Event(c, "transaction_deleted",
		"transaction_id", id,
		"is_synced_copy", isSyncedCopy,
		"source_transaction_id", sourceTransactionID,
	)

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// List 获取流水列表
func (h *TransactionHandler) List(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)

	var req service.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	// 解析时间参数
	if s := c.Query("start"); s != "" {
		t, err := parseTime(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start 格式错误"})
			return
		}
		req.Start = &t
	}
	if s := c.Query("end"); s != "" {
		t, err := parseTime(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end 格式错误"})
			return
		}
		// 纯日期补到当天结束，避免漏掉 end 当天流水
		if len(s) == 10 {
			t = t.Add(24*time.Hour - time.Second)
		}
		req.End = &t
	}

	// 解析数字参数
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

// ListMonths 获取按月折叠摘要，月份内容由 GET /transactions 按需加载。
func (h *TransactionHandler) ListMonths(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	months, err := h.service.ListMonths(ledgerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": months})
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

// PushRequest 单笔推送请求
type PushRequest struct {
	TargetLedgerID uint `json:"target_ledger_id" binding:"required"`
}

type BatchTransactionRequest struct {
	TransactionIDs []uint `json:"transaction_ids" binding:"required"`
}

type BatchPushRequest struct {
	TransactionIDs []uint `json:"transaction_ids" binding:"required"`
	TargetLedgerID uint   `json:"target_ledger_id" binding:"required"`
}

// DeleteBatch 批量删除当前账本内选中的流水。
func (h *TransactionHandler) DeleteBatch(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	var req BatchTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	deleted, err := h.service.DeleteBatch(ledgerID, userID, req.TransactionIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transactions_batch_deleted", "selected", len(req.TransactionIDs), "deleted", deleted)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": deleted}})
}

// PushBatch 批量同步选中的个人流水到目标账本。
func (h *TransactionHandler) PushBatch(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	var req BatchPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	result, err := h.service.PushBatch(ledgerID, req.TransactionIDs, req.TargetLedgerID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transactions_batch_pushed",
		"target_ledger_id", req.TargetLedgerID,
		"selected", len(req.TransactionIDs),
		"pushed", result.Pushed,
		"skipped", result.Skipped,
		"failed", result.Failed,
	)
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Push 单笔同步到其他类型账本
func (h *TransactionHandler) Push(c *gin.Context) {
	srcLedgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	t, err := h.service.PushToLedger(srcLedgerID, id, req.TargetLedgerID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transaction_pushed",
		"source_transaction_id", id,
		"target_transaction_id", t.ID,
		"source_ledger_id", srcLedgerID,
		"target_ledger_id", req.TargetLedgerID,
	)
	c.JSON(http.StatusOK, gin.H{"data": t})
}

// PushMonthRequest 整月推送请求
type PushMonthRequest struct {
	Month          string `json:"month" binding:"required"`
	TargetLedgerID uint   `json:"target_ledger_id" binding:"required"`
}

// PushMonth 整月同步到其他类型账本
func (h *TransactionHandler) PushMonth(c *gin.Context) {
	srcLedgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req PushMonthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	result, err := h.service.PushMonth(srcLedgerID, req.Month, req.TargetLedgerID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "transaction_month_pushed",
		"source_ledger_id", srcLedgerID,
		"target_ledger_id", req.TargetLedgerID,
		"month", req.Month,
		"pushed", result.Pushed,
		"skipped", result.Skipped,
	)
	c.JSON(http.StatusOK, gin.H{"data": result})
}
