package service

import (
	"errors"
	"fmt"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db *gorm.DB
}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	return &AuthService{db: repository.GetDB()}
}

// Register 注册
func (s *AuthService) Register(username, password, nickname string) (*model.User, string, error) {
	// 检查用户名是否已存在
	var count int64
	s.db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, "", errors.New("用户名已存在")
	}

	// 密码加密
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, "", fmt.Errorf("创建用户失败: %w", err)
	}

	// 创建个人账本（个人账本也生成 invite_code，避免空字符串触发 UNIQUE 约束冲突）
	ledger := model.Ledger{
		Name:       user.Nickname + "的个人账本",
		OwnerID:    user.ID,
		IsPersonal: true,
		InviteCode: generateRandomString(6),
	}
	if err := s.db.Create(&ledger).Error; err != nil {
		return nil, "", fmt.Errorf("创建个人账本失败: %w", err)
	}

	// 创建 owner 成员关系
	membership := model.Membership{
		LedgerID: ledger.ID,
		UserID:   user.ID,
		Role:     "owner",
	}
	if err := s.db.Create(&membership).Error; err != nil {
		return nil, "", fmt.Errorf("创建成员关系失败: %w", err)
	}

	// 播种默认账户和分类
	if err := SeedDefaultData(s.db, ledger.ID, user.ID); err != nil {
		// 播种失败不影响注册，只记录日志
		fmt.Printf("警告: 播种默认数据失败: %v\n", err)
	}

	// 签发 Token
	cfg := config.Get()
	token, err := middleware.NewToken(user.ID, cfg.JWT.Secret, cfg.JWT.Expire)
	if err != nil {
		return nil, "", fmt.Errorf("签发令牌失败: %w", err)
	}

	return &user, token, nil
}

// Login 登录
func (s *AuthService) Login(username, password string) (*model.User, string, error) {
	// 查找用户
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", errors.New("用户名或密码错误")
		}
		return nil, "", fmt.Errorf("查询用户失败: %w", err)
	}

	// 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("用户名或密码错误")
	}

	// 签发 Token
	cfg := config.Get()
	token, err := middleware.NewToken(user.ID, cfg.JWT.Secret, cfg.JWT.Expire)
	if err != nil {
		return nil, "", fmt.Errorf("签发令牌失败: %w", err)
	}

	return &user, token, nil
}

// GetUser 获取用户信息
func (s *AuthService) GetUser(userID uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// generateRandomString 生成随机邀请码（复用 model.GenInviteCode，保持 6 位去易混字符）
func generateRandomString(length int) string {
	return model.GenInviteCode()
}

// formatTime 格式化时间
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
