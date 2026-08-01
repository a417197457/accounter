package handler

import (
	"context"
	"net/http"
	"strings"

	"accounter/backend/internal/ai"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// AIHandler AI 处理器
type AIHandler struct {
	aiService *ai.Service
}

// NewAIHandler 创建 AI 处理器
func NewAIHandler(aiService *ai.Service) *AIHandler {
	return &AIHandler{aiService: aiService}
}

// ChatRequest 对话请求
type ChatRequest struct {
	Message string `json:"message" binding:"required"`
}

// Chat AI 对话记账
func (h *AIHandler) Chat(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	result, err := h.aiService.Chat(context.Background(), ledgerID, userID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// TranscribeRequest 语音识别请求
type TranscribeRequest struct {
	Audio  string `json:"audio" binding:"required"` // base64 编码（不含 data: 前缀）
	Format string `json:"format" binding:"required"` // mp3/wav/webm/m4a/aac/ogg
}

// Transcribe 语音识别
func (h *AIHandler) Transcribe(c *gin.Context) {
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

	text, err := h.aiService.Transcribe(context.Background(), audio, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"text": text}})
}

// ConfirmRequest 确认记账请求
type ConfirmRequest struct {
	Draft *ai.Draft `json:"draft" binding:"required"`
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
	if req.Draft == nil {
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

	tx, err := h.aiService.ConfirmDraft(ledgerID, userID, req.Draft)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tx})
}

// Settings AI 配置（脱敏返回）
func (h *AIHandler) Settings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"configured": h.aiService.IsConfigured(),
		},
	})
}
