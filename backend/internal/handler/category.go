package handler

import (
	"net/http"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// CategoryHandler 分类处理器
type CategoryHandler struct {
	service *service.CategoryService
}

// NewCategoryHandler 创建分类处理器
func NewCategoryHandler() *CategoryHandler {
	return &CategoryHandler{
		service: service.NewCategoryService(),
	}
}

// CreateRequest 创建分类请求
type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=64"`
	ParentID *uint  `json:"parent_id"`
	Kind     string `json:"kind" binding:"required,oneof=income expense"`
	Icon     string `json:"icon" binding:"max=64"`
	Sort     int    `json:"sort"`
}

// UpdateRequest 更新分类请求
type UpdateCategoryRequest struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
	Icon string `json:"icon" binding:"max=64"`
	Sort int    `json:"sort"`
}

// Create 创建分类
func (h *CategoryHandler) Create(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	cat := &model.Category{
		Name:     req.Name,
		ParentID: req.ParentID,
		Kind:     req.Kind,
		Icon:     req.Icon,
		Sort:     req.Sort,
	}

	cat, err := h.service.Create(ledgerID, userID, cat)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cat})
}

// Update 更新分类
func (h *CategoryHandler) Update(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	next := &model.Category{
		Name: req.Name,
		Icon: req.Icon,
		Sort: req.Sort,
	}

	cat, err := h.service.Update(ledgerID, userID, id, next)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cat})
}

// Delete 删除分类
func (h *CategoryHandler) Delete(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	userID := middleware.UserID(c)
	id := getUintParam(c, "id")

	if err := h.service.Delete(ledgerID, userID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ListTree 获取分类树
func (h *CategoryHandler) ListTree(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	kind := c.Query("kind")

	cats, err := h.service.ListTree(ledgerID, kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cats})
}

// ListFlat 获取扁平分类列表
func (h *CategoryHandler) ListFlat(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)

	cats, err := h.service.ListFlat(ledgerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cats})
}
