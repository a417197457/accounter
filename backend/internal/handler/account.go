package handler

import (
	"net/http"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AccountHandler 账户处理器
type AccountHandler struct {
	service *service.AccountService
}

// NewAccountHandler 创建账户处理器
func NewAccountHandler() *AccountHandler {
	return &AccountHandler{
		service: service.NewAccountService(),
	}
}

// CreateRequest 创建账户请求
type CreateAccountRequest struct {
	Name             string `json:"name" binding:"required,min=1,max=64"`
	Type             string `json:"type" binding:"required,max=32"`
	Sort             int    `json:"sort"`
	StatementDay     *int   `json:"statement_day"`
	DueDay           *int   `json:"due_day"`
	LinkedAccountIDs []uint `json:"linked_account_ids"`
}

// UpdateRequest 更新账户请求
type UpdateAccountRequest struct {
	Name             string `json:"name" binding:"required,min=1,max=64"`
	Type             string `json:"type" binding:"max=32"`
	Sort             int    `json:"sort"`
	IsArchived       bool   `json:"is_archived"`
	StatementDay     *int   `json:"statement_day"`
	DueDay           *int   `json:"due_day"`
	LinkedAccountIDs []uint `json:"linked_account_ids"`
}

// Create 创建账户
func (h *AccountHandler) Create(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	acc := &model.Account{
		Name:         req.Name,
		Type:         req.Type,
		Sort:         req.Sort,
		Balance:      decimal.Zero,
		StatementDay: req.StatementDay,
		DueDay:       req.DueDay,
	}

	acc, err := h.service.Create(ledgerID, userID, acc, req.LinkedAccountIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": acc})
}

// Update 更新账户
func (h *AccountHandler) Update(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	next := &model.Account{
		Name:         req.Name,
		Type:         req.Type,
		Sort:         req.Sort,
		IsArchived:   req.IsArchived,
		StatementDay: req.StatementDay,
		DueDay:       req.DueDay,
	}

	acc, err := h.service.Update(ledgerID, userID, id, next, req.LinkedAccountIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": acc})
}

// Delete 删除账户
func (h *AccountHandler) Delete(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	if err := h.service.Delete(ledgerID, userID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// List 获取账户列表
func (h *AccountHandler) List(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)

	accounts, err := h.service.List(ledgerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": accounts})
}
