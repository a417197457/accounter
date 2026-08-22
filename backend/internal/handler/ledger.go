package handler

import (
	"fmt"
	"net/http"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/observability"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// LedgerHandler 账本处理器
type LedgerHandler struct {
	service *service.LedgerService
}

// NewLedgerHandler 创建账本处理器
func NewLedgerHandler() *LedgerHandler {
	return &LedgerHandler{
		service: service.NewLedgerService(),
	}
}

// CreateRequest 创建账本请求
type CreateLedgerRequest struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
	Type string `json:"type" binding:"omitempty,oneof=family travel gift"`
}

// JoinRequest 加入账本请求
type JoinLedgerRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// List 获取账本列表
func (h *LedgerHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)
	ledgers, err := h.service.ListForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ledgers})
}

// Create 创建账本
func (h *LedgerHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req CreateLedgerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ledger, inviteCode, err := h.service.Create(userID, req.Name, req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "ledger_created",
		"created_ledger_id", ledger.ID,
		"ledger_type", ledger.Type,
		"ledger_name", ledger.Name,
	)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"ledger":      ledger,
			"invite_code": inviteCode,
		},
	})
}

// Delete 删除创建者拥有的非个人账本。
func (h *LedgerHandler) Delete(c *gin.Context) {
	userID := middleware.UserID(c)
	ledgerID := getUintParam(c, "id")
	ledger, err := h.service.Delete(ledgerID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "ledger_deleted",
		"deleted_ledger_id", ledger.ID,
		"ledger_type", ledger.Type,
		"ledger_name", ledger.Name,
	)
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// Join 加入账本
func (h *LedgerHandler) Join(c *gin.Context) {
	userID := middleware.UserID(c)

	var req JoinLedgerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ledger, err := h.service.Join(userID, req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "ledger_joined",
		"joined_ledger_id", ledger.ID,
		"ledger_type", ledger.Type,
	)

	c.JSON(http.StatusOK, gin.H{"data": ledger})
}

// Members 获取成员列表
func (h *LedgerHandler) Members(c *gin.Context) {
	userID := middleware.UserID(c)
	ledgerID := getUintParam(c, "id")

	members, err := h.service.Members(ledgerID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": members})
}

// RemoveMember 移除成员
func (h *LedgerHandler) RemoveMember(c *gin.Context) {
	userID := middleware.UserID(c)
	ledgerID := getUintParam(c, "id")
	targetUserID := getUintParam(c, "uid")

	if err := h.service.RemoveMember(ledgerID, userID, targetUserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "ledger_member_removed", "target_user_id", targetUserID)

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// Leave 退出账本
func (h *LedgerHandler) Leave(c *gin.Context) {
	userID := middleware.UserID(c)
	ledgerID := getUintParam(c, "id")

	if err := h.service.Leave(ledgerID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "ledger_left")

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// getUintParam 辅助函数：获取 URL 参数并转换为 uint
func getUintParam(c *gin.Context, key string) uint {
	val := c.Param(key)
	var result uint
	if _, err := fmt.Sscanf(val, "%d", &result); err != nil {
		return 0
	}
	return result
}
