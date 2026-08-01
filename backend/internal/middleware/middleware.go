package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// JWT Claims 结构
type Claims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

// NewToken 签发 JWT Token
func NewToken(userID uint, secret string, expire time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 解析 JWT Token，返回 userID
func ParseToken(tokenStr, secret string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return 0, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims.UserID, nil
	}
	return 0, jwt.ErrTokenInvalidClaims
}

// Auth JWT 鉴权中间件
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌格式错误"})
			c.Abort()
			return
		}

		userID, err := ParseToken(parts[1], secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌无效或已过期"})
			c.Abort()
			return
		}

		// 注入 userID 到上下文
		c.Set("userID", userID)
		c.Next()
	}
}

// RequireLedger 账本上下文中间件
func RequireLedger() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}
		uid := userID.(uint)

		db := repository.GetDB()

		// 读 X-Ledger-Id 头
		ledgerIDStr := c.GetHeader("X-Ledger-Id")
		var ledgerID uint

		if ledgerIDStr == "" {
			// 为空则用该用户的个人账本；没有则兜底自动创建（个人账本默认就有，不应报错）
			ledgerID = getOrCreatePersonalLedger(db, uid)
		} else {
			// 非空则校验是否为该账本成员
			var lid uint
			if _, err := fmt.Sscanf(ledgerIDStr, "%d", &lid); err != nil || lid == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "账本 ID 格式错误"})
				c.Abort()
				return
			}
			ledgerID = lid

			var membership model.Membership
			if err := db.Where("ledger_id = ? AND user_id = ?", ledgerID, uid).First(&membership).Error; err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "你不是该账本成员"})
				c.Abort()
				return
			}
		}

		// 注入 ledgerID 到上下文
		c.Set("ledgerID", ledgerID)
		c.Next()
	}
}

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Ledger-Id")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// UserID 从上下文获取 userID
func UserID(c *gin.Context) uint {
	val, exists := c.Get("userID")
	if !exists {
		return 0
	}
	return val.(uint)
}

// getOrCreatePersonalLedger 获取用户的个人账本，没有则兜底创建（含 owner 成员关系）
// 个人账本默认就该有，不该因缺失而报错
func getOrCreatePersonalLedger(db *gorm.DB, uid uint) uint {
	var ledger model.Ledger
	_ = db.Where("owner_id = ? AND is_personal = ?", uid, true).First(&ledger).Error
	if ledger.ID != 0 {
		return ledger.ID
	}
	// 兜底创建
	ledger = model.Ledger{
		Name:       "我的个人账本",
		OwnerID:    uid,
		IsPersonal: true,
		InviteCode: model.GenInviteCode(),
	}
	if err := db.Create(&ledger).Error; err != nil {
		return 0
	}
	db.Create(&model.Membership{
		LedgerID: ledger.ID,
		UserID:   uid,
		Role:     "owner",
	})
	return ledger.ID
}

// LedgerID 从上下文获取 ledgerID
func LedgerID(c *gin.Context) uint {
	val, exists := c.Get("ledgerID")
	if !exists {
		return 0
	}
	return val.(uint)
}
