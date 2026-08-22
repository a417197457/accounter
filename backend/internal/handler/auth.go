package handler

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"accounter/backend/internal/config"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/observability"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	service *service.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		service: service.NewAuthService(),
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Nickname string `json:"nickname" binding:"max=32"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type WechatLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

type WechatPhoneLoginRequest struct {
	Code      string `json:"code" binding:"required"`
	PhoneCode string `json:"phone_code" binding:"required"`
}

type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"required"`
}

// Register 注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	user, token, err := h.service.Register(req.Username, req.Password, req.Nickname)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_registered", "authenticated_user_id", user.ID, "login_type", "password")

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	user, token, err := h.service.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_login", "authenticated_user_id", user.ID, "login_type", "password")

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}

// WechatLogin 微信小程序 openid 登录，不要求绑定手机号。
func (h *AuthHandler) WechatLogin(c *gin.Context) {
	var req WechatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少微信登录凭证"})
		return
	}
	user, token, err := h.service.WechatLogin(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_login", "authenticated_user_id", user.ID, "login_type", "wechat")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token, "user": user}})
}

// WechatPhoneLogin 微信小程序授权手机号登录。
func (h *AuthHandler) WechatPhoneLogin(c *gin.Context) {
	var req WechatPhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少微信登录凭证或手机号授权凭证"})
		return
	}
	user, token, err := h.service.WechatPhoneLogin(c.Request.Context(), req.Code, req.PhoneCode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_login", "authenticated_user_id", user.ID, "login_type", "wechat_phone")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token, "user": user}})
}

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := middleware.UserID(c)
	user, err := h.service.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// UpdateProfile 更新当前用户公开昵称。
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写昵称"})
		return
	}
	user, err := h.service.UpdateNickname(middleware.UserID(c), req.Nickname)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_profile_updated", "field", "nickname")
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// UploadAvatar 上传当前用户头像。只接收经过解码验证且不超过 2MB 的 JPG/PNG。
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	const maxAvatarBytes = 2 << 20
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarBytes+(256<<10))
	header, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择头像"})
		return
	}
	if header.Size <= 0 || header.Size > maxAvatarBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像大小不能超过 2MB"})
		return
	}
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取头像失败"})
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil || len(content) > maxAvatarBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取头像失败或头像过大"})
		return
	}
	imageConfig, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || (format != "jpeg" && format != "png") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像只支持 JPG 或 PNG"})
		return
	}
	if imageConfig.Width <= 0 || imageConfig.Height <= 0 || imageConfig.Width > 4096 || imageConfig.Height > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像尺寸无效或过大"})
		return
	}
	extension := ".jpg"
	if format == "png" {
		extension = ".png"
	}
	user, err := h.service.UpdateAvatar(middleware.UserID(c), config.Get().Storage.UploadDir, content, extension)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	observability.Event(c, "user_profile_updated",
		"field", "avatar",
		"avatar_bytes", len(content),
		"avatar_format", format,
	)
	c.JSON(http.StatusOK, gin.H{"data": user})
}
