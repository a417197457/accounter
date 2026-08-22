package handler

import (
	"net/http"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type BookkeepingPreferenceHandler struct {
	service *service.BookkeepingPreferenceService
}

func NewBookkeepingPreferenceHandler() *BookkeepingPreferenceHandler {
	return &BookkeepingPreferenceHandler{service: service.NewBookkeepingPreferenceService()}
}

func (h *BookkeepingPreferenceHandler) Get(c *gin.Context) {
	view, err := h.service.Get(middleware.LedgerID(c), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

type UpdateBookkeepingPreferenceRequest struct {
	AccountMode             string `json:"account_mode" binding:"required"`
	DefaultAccountID        uint   `json:"default_account_id" binding:"required"`
	DefaultPaymentAccountID *uint  `json:"default_payment_account_id"`
}

func (h *BookkeepingPreferenceHandler) Update(c *gin.Context) {
	var req UpdateBookkeepingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	view, err := h.service.Update(
		middleware.LedgerID(c), middleware.UserID(c), req.AccountMode,
		req.DefaultAccountID, req.DefaultPaymentAccountID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}
