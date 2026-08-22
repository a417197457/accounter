package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
	"accounter/backend/internal/wechat"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db     *gorm.DB
	wechat *wechat.Client
}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	cfg := config.Get()
	return &AuthService{
		db:     repository.GetDB(),
		wechat: wechat.NewClient(cfg.WeChat.AppID, cfg.WeChat.AppSecret, cfg.WeChat.APIBase),
	}
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

	var user model.User
	// 用户、个人账本、成员关系和默认数据必须同时成功，避免产生无法使用的半成品账号。
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		user = model.User{
			Username:     username,
			PasswordHash: string(hash),
			Nickname:     nickname,
		}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("创建用户失败: %w", err)
		}

		ledger := model.Ledger{
			Name:       user.Nickname + "的个人账本",
			OwnerID:    user.ID,
			IsPersonal: true,
			InviteCode: generateRandomString(6),
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return fmt.Errorf("创建个人账本失败: %w", err)
		}
		if err := tx.Create(&model.Membership{LedgerID: ledger.ID, UserID: user.ID, Role: "owner"}).Error; err != nil {
			return fmt.Errorf("创建成员关系失败: %w", err)
		}
		if err := SeedDefaultData(tx, ledger.ID, user.ID); err != nil {
			return fmt.Errorf("创建默认账户和分类失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, "", err
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

// WechatLogin 使用 wx.login code 登录；首次登录时按 openid 自动注册。
// 个人主体小程序可使用该流程，不依赖手机号快速验证组件。
func (s *AuthService) WechatLogin(ctx context.Context, loginCode string) (*model.User, string, error) {
	openID, err := s.wechat.Code2Session(ctx, loginCode)
	if err != nil {
		return nil, "", err
	}

	var user model.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("wechat_open_id = ?", openID).First(&user).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return createWechatUser(tx, &user, openID, nil)
	})
	if err != nil {
		return nil, "", err
	}
	return issueUserToken(&user)
}

// WechatPhoneLogin 使用微信登录 code 和手机号授权 code 登录；首次授权时自动注册。
func (s *AuthService) WechatPhoneLogin(ctx context.Context, loginCode, phoneCode string) (*model.User, string, error) {
	openID, err := s.wechat.Code2Session(ctx, loginCode)
	if err != nil {
		return nil, "", err
	}
	phone, err := s.wechat.GetPhoneNumber(ctx, phoneCode)
	if err != nil {
		return nil, "", err
	}

	var user model.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 同一微信身份再次登录：校验手机号没有被其他用户占用，并刷新绑定号码。
		if err := tx.Where("wechat_open_id = ?", openID).First(&user).Error; err == nil {
			var owner model.User
			if findErr := tx.Where("phone = ? AND id <> ?", phone, user.ID).First(&owner).Error; findErr == nil {
				return errors.New("该手机号已绑定其他微信账号")
			} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			user.Phone = &phone
			return tx.Model(&user).Update("phone", phone).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 手机号此前通过其他方式注册过时，直接绑定微信身份，避免生成两个账户。
		if err := tx.Where("phone = ?", phone).First(&user).Error; err == nil {
			if user.WechatOpenID != nil && *user.WechatOpenID != openID {
				return errors.New("该手机号已绑定其他微信账号")
			}
			user.WechatOpenID = &openID
			return tx.Model(&user).Update("wechat_open_id", openID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		return createWechatUser(tx, &user, openID, &phone)
	})
	if err != nil {
		return nil, "", err
	}

	return issueUserToken(&user)
}

func createWechatUser(tx *gorm.DB, user *model.User, openID string, phone *string) error {
	openidHash := sha256.Sum256([]byte(openID))
	username := "wx_" + hex.EncodeToString(openidHash[:])[:20]
	// bcrypt 拒绝超过 72 字节的输入。微信用户不会使用密码登录，先将随机种子
	// 压缩为固定 32 字节，再生成不可用的内部密码凭证。
	credentialSeed := sha256.Sum256([]byte(openID + time.Now().String()))
	randomPassword, err := bcrypt.GenerateFromPassword(credentialSeed[:], bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("创建微信用户凭证失败: %w", err)
	}
	*user = model.User{
		Username: username, PasswordHash: string(randomPassword), Nickname: "微信用户",
		Phone: phone, WechatOpenID: &openID,
	}
	if err := tx.Create(user).Error; err != nil {
		return fmt.Errorf("创建微信用户失败: %w", err)
	}
	ledger := model.Ledger{
		Name: "我的个人账本", OwnerID: user.ID, IsPersonal: true, InviteCode: model.GenInviteCode(),
	}
	if err := tx.Create(&ledger).Error; err != nil {
		return fmt.Errorf("创建个人账本失败: %w", err)
	}
	if err := tx.Create(&model.Membership{LedgerID: ledger.ID, UserID: user.ID, Role: "owner"}).Error; err != nil {
		return fmt.Errorf("创建成员关系失败: %w", err)
	}
	return SeedDefaultData(tx, ledger.ID, user.ID)
}

func issueUserToken(user *model.User) (*model.User, string, error) {
	cfg := config.Get()
	token, err := middleware.NewToken(user.ID, cfg.JWT.Secret, cfg.JWT.Expire)
	if err != nil {
		return nil, "", fmt.Errorf("签发令牌失败: %w", err)
	}
	return user, token, nil
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
