package handler

import (
	"errors"
	"net/http"
	"strings"

	"accounter/backend/internal/ai"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/observability"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// AIHandler AI 处理器
type AIHandler struct {
	aiService          *ai.Service
	entitlementService *service.AIEntitlementService
}

// NewAIHandler 创建 AI 处理器
func NewAIHandler(aiService *ai.Service) *AIHandler {
	return &AIHandler{
		aiService:          aiService,
		entitlementService: service.NewAIEntitlementService(),
	}
}

func writeAIAccessError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrAIAccessExpired):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "AI_ACCESS_EXPIRED"})
		return true
	case errors.Is(err, service.ErrAIDailyLimitReached):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error(), "code": "AI_DAILY_LIMIT_REACHED"})
		return true
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查AI权益失败"})
		return true
	default:
		return false
	}
}

// ChatRequest 对话请求（history 不含本轮 message）
type ChatRequest struct {
	Message string              `json:"message" binding:"required"`
	History []ai.HistoryMessage `json:"history"`
}

// Chat AI 对话记账
func (h *AIHandler) Chat(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	if _, err := h.entitlementService.CheckAccess(userID); writeAIAccessError(c, err) {
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	result, err := h.aiService.Chat(c.Request.Context(), ledgerID, userID, req.Message, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.entitlementService.RecordSuccessfulUse(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录AI试用状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// TranscribeRequest 语音识别请求
type TranscribeRequest struct {
	Audio  string `json:"audio" binding:"required"`  // base64 编码（不含 data: 前缀）
	Format string `json:"format" binding:"required"` // mp3/wav/webm/m4a/aac/ogg
}

// Transcribe 语音识别
func (h *AIHandler) Transcribe(c *gin.Context) {
	userID := middleware.UserID(c)
	if _, err := h.entitlementService.CheckAccess(userID); writeAIAccessError(c, err) {
		return
	}
	var req TranscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 去掉可能的 data URI 前缀
	audio := req.Audio
	if idx := strings.Index(audio, "base64,"); idx >= 0 {
		audio = audio[idx+7:]
	}

	text, err := h.aiService.Transcribe(c.Request.Context(), audio, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.entitlementService.RecordSuccessfulUse(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录AI试用状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"text": text}})
}

// Entitlement 返回当前用户的 AI 试用和激活状态。
func (h *AIHandler) Entitlement(c *gin.Context) {
	view, err := h.entitlementService.Status(middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取AI权益失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

type ActivateAIRequest struct {
	Code string `json:"code" binding:"required,max=64"`
}

// Activate 使用一次性激活码永久开通当前账号的 AI 权益。
func (h *AIHandler) Activate(c *gin.Context) {
	var req ActivateAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写有效激活码", "code": "AI_ACTIVATION_CODE_INVALID"})
		return
	}
	view, err := h.entitlementService.Activate(middleware.UserID(c), req.Code, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrActivationRateLimited):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error(), "code": "AI_ACTIVATION_RATE_LIMITED"})
		case errors.Is(err, service.ErrActivationCodeExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "AI_ACTIVATION_CODE_EXPIRED"})
		case errors.Is(err, service.ErrActivationCodeInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "AI_ACTIVATION_CODE_INVALID"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "激活失败，请稍后重试"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// ConfirmRequest 确认记账请求（单笔 draft 或多笔 drafts）
type ConfirmRequest struct {
	Draft  *ai.Draft   `json:"draft"`
	Drafts []*ai.Draft `json:"drafts"`
}

// Confirm 确认草稿落库（仅个人账本）
func (h *AIHandler) Confirm(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req ConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	drafts := req.Drafts
	if len(drafts) == 0 && req.Draft != nil {
		drafts = []*ai.Draft{req.Draft}
	}
	if len(drafts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少草稿数据"})
		return
	}

	// 记账只在个人账本
	ledgerSvc := service.NewLedgerService()
	isPersonal, err := ledgerSvc.IsPersonal(ledgerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账本不存在"})
		return
	}
	if !isPersonal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "家庭账本不能直接记账，请在个人账本记账后同步过来"})
		return
	}

	txs, err := h.aiService.ConfirmDrafts(ledgerID, userID, drafts)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, tx := range txs {
		observability.Event(c, "transaction_created",
			"transaction_id", tx.ID,
			"source", "ai",
			"op_type", tx.OpType,
			"amount", tx.Amount.String(),
			"account_id", tx.AccountID,
			"occurred_at", tx.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
		)
	}

	// 单笔兼容旧前端：data 仍返回那一笔；多笔附带 items
	if len(txs) == 1 {
		c.JSON(http.StatusOK, gin.H{"data": txs[0], "items": txs})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": txs, "items": txs})
}

// Settings AI 配置（脱敏返回）
func (h *AIHandler) Settings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured": h.aiService.IsConfigured(),
		},
	})
}
