package service

import (
	"errors"
	"fmt"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxOwnedSharedLedgers int64 = 10

var ErrLedgerCreateLimit = errors.New("每个用户最多创建 10 个账本")

// LedgerService 账本服务
type LedgerService struct {
	db *gorm.DB
}

// NewLedgerService 创建账本服务
func NewLedgerService() *LedgerService {
	return &LedgerService{db: repository.GetDB()}
}

// LedgerWithRole 账本信息（包含用户在该账本的角色）
type LedgerWithRole struct {
	model.Ledger
	Role        string `json:"role"`
	MemberCount int64  `json:"member_count"`
}

// ListForUser 获取用户加入的所有账本
func (s *LedgerService) ListForUser(userID uint) ([]LedgerWithRole, error) {
	var results []LedgerWithRole

	// 查询用户的所有账本及角色
	err := s.db.Table("ledgers").
		Select("ledgers.*, memberships.role, (SELECT COUNT(*) FROM memberships m WHERE m.ledger_id = ledgers.id) as member_count").
		Joins("JOIN memberships ON memberships.ledger_id = ledgers.id").
		Where("memberships.user_id = ? AND ledgers.deleted_at IS NULL", userID).
		Order("ledgers.is_personal DESC, ledgers.created_at DESC").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("查询账本列表失败: %w", err)
	}

	return results, nil
}

// Create 创建家庭、旅游或礼账账本。个人账本由注册流程自动创建。
func (s *LedgerService) Create(userID uint, name, ledgerType string) (*model.Ledger, string, error) {
	if ledgerType == "" {
		ledgerType = model.LedgerTypeFamily
	}
	if !model.ValidLedgerType(ledgerType) || ledgerType == model.LedgerTypePersonal {
		return nil, "", errors.New("账本类型只支持家庭、旅游或礼账")
	}
	var ledger model.Ledger
	var inviteCode string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 锁住当前用户行，确保并发创建也不能突破 10 个上限。
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&user, userID).Error; err != nil {
			return errors.New("用户不存在")
		}
		var ownedCount int64
		if err := tx.Model(&model.Ledger{}).
			Where("owner_id = ? AND is_personal = ? AND deleted_at IS NULL", userID, false).
			Count(&ownedCount).Error; err != nil {
			return fmt.Errorf("查询账本数量失败: %w", err)
		}
		if ownedCount >= MaxOwnedSharedLedgers {
			return ErrLedgerCreateLimit
		}

		inviteCode = generateRandomString(6)
		for {
			var count int64
			if err := tx.Unscoped().Model(&model.Ledger{}).Where("invite_code = ?", inviteCode).Count(&count).Error; err != nil {
				return fmt.Errorf("检查邀请码失败: %w", err)
			}
			if count == 0 {
				break
			}
			inviteCode = generateRandomString(6)
		}

		ledger = model.Ledger{
			Name: name, OwnerID: userID, IsPersonal: false, Type: ledgerType, InviteCode: inviteCode,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return fmt.Errorf("创建账本失败: %w", err)
		}
		if err := tx.Create(&model.Membership{LedgerID: ledger.ID, UserID: userID, Role: "owner"}).Error; err != nil {
			return fmt.Errorf("创建成员关系失败: %w", err)
		}
		if err := SeedDefaultData(tx, ledger.ID, userID); err != nil {
			return fmt.Errorf("初始化默认数据失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return &ledger, inviteCode, nil
}

// Delete 删除创建者拥有的非个人账本。账本里的同步副本会一起删除，
// 但个人账本中的源流水不受影响。
func (s *LedgerService) Delete(ledgerID, userID uint) (*model.Ledger, error) {
	var deletedLedger model.Ledger
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&deletedLedger, ledgerID).Error; err != nil {
			return errors.New("账本不存在")
		}
		if deletedLedger.IsPersonal {
			return errors.New("个人账本不能删除")
		}
		if deletedLedger.OwnerID != userID {
			return errors.New("只有创建者可以删除账本")
		}

		// 先让同步副本退出后续源流水的级联更新范围，再清理账本附属数据。
		if err := tx.Where("ledger_id = ?", ledgerID).Delete(&model.Transaction{}).Error; err != nil {
			return fmt.Errorf("删除账本流水失败: %w", err)
		}
		if err := tx.Where("ledger_id = ?", ledgerID).Delete(&model.Account{}).Error; err != nil {
			return fmt.Errorf("删除账本账户失败: %w", err)
		}
		if err := tx.Where("ledger_id = ?", ledgerID).Delete(&model.Category{}).Error; err != nil {
			return fmt.Errorf("删除账本分类失败: %w", err)
		}
		for _, item := range []interface{}{
			&model.AccountLink{}, &model.Budget{}, &model.BookkeepingPreference{}, &model.Membership{},
		} {
			if err := tx.Where("ledger_id = ?", ledgerID).Delete(item).Error; err != nil {
				return fmt.Errorf("清理账本关联数据失败: %w", err)
			}
		}
		if err := tx.Delete(&deletedLedger).Error; err != nil {
			return fmt.Errorf("删除账本失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &deletedLedger, nil
}

// Join 通过邀请码加入账本
func (s *LedgerService) Join(userID uint, inviteCode string) (*model.Ledger, error) {
	// 查找账本
	var ledger model.Ledger
	if err := s.db.Where("invite_code = ?", inviteCode).First(&ledger).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("邀请码无效")
		}
		return nil, fmt.Errorf("查询账本失败: %w", err)
	}

	// 检查是否已是成员
	var count int64
	s.db.Model(&model.Membership{}).
		Where("ledger_id = ? AND user_id = ?", ledger.ID, userID).
		Count(&count)
	if count > 0 {
		return &ledger, nil // 已是成员，幂等返回
	}

	// 创建成员关系
	membership := model.Membership{
		LedgerID: ledger.ID,
		UserID:   userID,
		Role:     "member",
	}
	if err := s.db.Create(&membership).Error; err != nil {
		return nil, fmt.Errorf("加入账本失败: %w", err)
	}

	return &ledger, nil
}

// MemberInfo 成员信息
type MemberInfo struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Alias     string `json:"alias"`
	Role      string `json:"role"`
}

// Members 获取账本成员列表
func (s *LedgerService) Members(ledgerID uint, requesterID uint) ([]MemberInfo, error) {
	// 检查是否是成员
	var count int64
	s.db.Model(&model.Membership{}).
		Where("ledger_id = ? AND user_id = ?", ledgerID, requesterID).
		Count(&count)
	if count == 0 {
		return nil, errors.New("你不是该账本成员")
	}

	var members []MemberInfo
	err := s.db.Table("memberships").
		Select("users.id as user_id, users.username, users.nickname, users.avatar_url, memberships.alias, memberships.role").
		Joins("JOIN users ON users.id = memberships.user_id").
		Where("memberships.ledger_id = ?", ledgerID).
		Scan(&members).Error

	if err != nil {
		return nil, fmt.Errorf("查询成员列表失败: %w", err)
	}

	return members, nil
}

// RemoveMember 移除成员（仅 owner 可操作）
func (s *LedgerService) RemoveMember(ledgerID, requesterID, targetUserID uint) error {
	// 检查请求者是否是 owner
	var requesterMembership model.Membership
	if err := s.db.Where("ledger_id = ? AND user_id = ?", ledgerID, requesterID).
		First(&requesterMembership).Error; err != nil {
		return errors.New("你不是该账本成员")
	}
	if requesterMembership.Role != "owner" {
		return errors.New("只有创建者可以移除成员")
	}

	// 不能移除 owner 自己
	if requesterID == targetUserID {
		return errors.New("不能移除创建者")
	}

	// 检查目标是否是成员
	var targetMembership model.Membership
	if err := s.db.Where("ledger_id = ? AND user_id = ?", ledgerID, targetUserID).
		First(&targetMembership).Error; err != nil {
		return errors.New("该用户不是账本成员")
	}

	// 删除成员关系
	if err := s.db.Delete(&targetMembership).Error; err != nil {
		return fmt.Errorf("移除成员失败: %w", err)
	}

	return nil
}

// Leave 成员退出账本
func (s *LedgerService) Leave(ledgerID, userID uint) error {
	// 查找成员关系
	var membership model.Membership
	if err := s.db.Where("ledger_id = ? AND user_id = ?", ledgerID, userID).
		First(&membership).Error; err != nil {
		return errors.New("你不是该账本成员")
	}

	// 检查是否是个人账本
	var ledger model.Ledger
	if err := s.db.First(&ledger, ledgerID).Error; err != nil {
		return fmt.Errorf("查询账本失败: %w", err)
	}
	if ledger.IsPersonal {
		return errors.New("不能退出个人账本")
	}

	// 检查是否是 owner
	if membership.Role == "owner" {
		return errors.New("创建者不能退出账本，请先转让或移除其他成员")
	}

	// 删除成员关系
	if err := s.db.Delete(&membership).Error; err != nil {
		return fmt.Errorf("退出账本失败: %w", err)
	}

	return nil
}

// Role 获取用户在账本中的角色
func (s *LedgerService) Role(ledgerID, userID uint) (string, error) {
	var membership model.Membership
	if err := s.db.Where("ledger_id = ? AND user_id = ?", ledgerID, userID).
		First(&membership).Error; err != nil {
		return "", errors.New("你不是该账本成员")
	}
	return membership.Role, nil
}

// IsPersonal 检查是否是个人账本
func (s *LedgerService) IsPersonal(ledgerID uint) (bool, error) {
	var ledger model.Ledger
	if err := s.db.First(&ledger, ledgerID).Error; err != nil {
		return false, err
	}
	return ledger.IsPersonal, nil
}

// PersonalLedgerID 获取用户的个人账本 ID
func (s *LedgerService) PersonalLedgerID(userID uint) (uint, error) {
	var ledger model.Ledger
	if err := s.db.Where("owner_id = ? AND is_personal = ?", userID, true).First(&ledger).Error; err != nil {
		return 0, err
	}
	return ledger.ID, nil
}
