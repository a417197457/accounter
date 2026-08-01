package service

import (
	"errors"
	"fmt"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"gorm.io/gorm"
)

// AccountService 账户服务
type AccountService struct {
	db *gorm.DB
}

// NewAccountService 创建账户服务
func NewAccountService() *AccountService {
	return &AccountService{db: repository.GetDB()}
}

// Create 创建账户
func (s *AccountService) Create(ledgerID, userID uint, acc *model.Account) (*model.Account, error) {
	acc.LedgerID = ledgerID
	acc.UserID = userID
	if acc.Balance.IsZero() == false {
		return nil, errors.New("新建账户余额必须为 0")
	}
	if err := s.db.Create(acc).Error; err != nil {
		return nil, fmt.Errorf("创建账户失败: %w", err)
	}
	return acc, nil
}

// Update 更新账户
func (s *AccountService) Update(ledgerID, userID, id uint, next *model.Account) (*model.Account, error) {
	var acc model.Account
	if err := s.db.First(&acc, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("账户不存在")
	}

	// 只允许更新 Name、Sort、IsArchived
	acc.Name = next.Name
	acc.Sort = next.Sort
	acc.IsArchived = next.IsArchived

	if err := s.db.Save(&acc).Error; err != nil {
		return nil, fmt.Errorf("更新账户失败: %w", err)
	}
	return &acc, nil
}

// Delete 删除账户（有流水则拒绝）
func (s *AccountService) Delete(ledgerID, userID, id uint) error {
	// 检查是否有流水使用该账户（作为 source 或 target）
	var count int64
	s.db.Model(&model.Transaction{}).
		Where("ledger_id = ? AND (account_id = ? OR to_account_id = ?) AND deleted_at IS NULL",
			ledgerID, id, id).
		Count(&count)
	if count > 0 {
		return errors.New("该账户下有流水，无法删除（可归档）")
	}

	if err := s.db.Delete(&model.Account{}, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return fmt.Errorf("删除账户失败: %w", err)
	}
	return nil
}

// List 获取账户列表
func (s *AccountService) List(ledgerID uint) ([]model.Account, error) {
	var accounts []model.Account
	err := s.db.Where("ledger_id = ?", ledgerID).
		Order("sort asc, id asc").
		Find(&accounts).Error
	if err != nil {
		return nil, fmt.Errorf("查询账户列表失败: %w", err)
	}
	return accounts, nil
}

// Get 获取单个账户
func (s *AccountService) Get(ledgerID, id uint) (*model.Account, error) {
	var acc model.Account
	if err := s.db.First(&acc, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("账户不存在")
	}
	return &acc, nil
}
