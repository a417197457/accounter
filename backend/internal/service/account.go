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
func (s *AccountService) Create(ledgerID, userID uint, acc *model.Account, linkedAccountIDs []uint) (*model.Account, error) {
	acc.LedgerID = ledgerID
	acc.UserID = userID
	if !model.ValidAccountType(acc.Type) {
		return nil, errors.New("账户类型只能是支付宝、微信、现金、储蓄卡或信用卡")
	}
	if acc.Balance.IsZero() == false {
		return nil, errors.New("新建账户余额必须为 0")
	}
	if err := normalizeCreditFields(acc); err != nil {
		return nil, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(acc).Error; err != nil {
			return fmt.Errorf("创建账户失败: %w", err)
		}
		return s.replaceLinks(tx, ledgerID, acc, linkedAccountIDs)
	}); err != nil {
		return nil, err
	}
	acc.LinkedAccounts, _ = s.linkedAccounts(ledgerID, acc.ID)
	return acc, nil
}

func normalizeCreditFields(acc *model.Account) error {
	if acc.Type != "credit" {
		acc.StatementDay = nil
		acc.DueDay = nil
		return nil
	}
	if acc.StatementDay != nil && !model.ValidMonthDay(*acc.StatementDay) {
		return errors.New("账单日请填写 1–28")
	}
	if acc.DueDay != nil && !model.ValidMonthDay(*acc.DueDay) {
		return errors.New("还款日请填写 1–28")
	}
	return nil
}

// Update 更新账户
func (s *AccountService) Update(ledgerID, userID, id uint, next *model.Account, linkedAccountIDs []uint) (*model.Account, error) {
	var acc model.Account
	if err := s.db.First(&acc, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("账户不存在")
	}

	if next.Type != "" && next.Type != acc.Type {
		return nil, errors.New("账户类型创建后不可修改")
	}

	// 只允许更新 Name、Sort、IsArchived、信用卡日期字段和支付渠道关联
	acc.Name = next.Name
	acc.Sort = next.Sort
	acc.IsArchived = next.IsArchived
	acc.StatementDay = next.StatementDay
	acc.DueDay = next.DueDay
	if err := normalizeCreditFields(&acc); err != nil {
		return nil, err
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&acc).Error; err != nil {
			return fmt.Errorf("更新账户失败: %w", err)
		}
		if linkedAccountIDs != nil {
			return s.replaceLinks(tx, ledgerID, &acc, linkedAccountIDs)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	acc.LinkedAccounts, _ = s.linkedAccounts(ledgerID, acc.ID)
	return &acc, nil
}

func (s *AccountService) replaceLinks(tx *gorm.DB, ledgerID uint, payment *model.Account, linkedAccountIDs []uint) error {
	if payment.Type != "alipay" && payment.Type != "wechat" {
		if len(linkedAccountIDs) > 0 {
			return errors.New("只有支付宝和微信可以关联储蓄卡或信用卡")
		}
		return tx.Where("ledger_id = ? AND payment_account_id = ?", ledgerID, payment.ID).Delete(&model.AccountLink{}).Error
	}

	if err := tx.Where("ledger_id = ? AND payment_account_id = ?", ledgerID, payment.ID).Delete(&model.AccountLink{}).Error; err != nil {
		return err
	}
	seen := map[uint]bool{}
	for _, fundingID := range linkedAccountIDs {
		if fundingID == 0 || seen[fundingID] {
			continue
		}
		seen[fundingID] = true
		var funding model.Account
		if err := tx.First(&funding, "id = ? AND ledger_id = ? AND is_archived = ?", fundingID, ledgerID, false).Error; err != nil {
			return errors.New("关联账户不存在或已归档")
		}
		if funding.Type != "savings" && funding.Type != "credit" {
			return errors.New("支付宝和微信只能关联储蓄卡或信用卡")
		}
		if err := tx.Create(&model.AccountLink{LedgerID: ledgerID, PaymentAccountID: payment.ID, FundingAccountID: funding.ID}).Error; err != nil {
			return fmt.Errorf("保存账户关联失败: %w", err)
		}
	}
	return nil
}

func (s *AccountService) linkedAccounts(ledgerID, paymentID uint) ([]model.Account, error) {
	result := []model.Account{}
	err := s.db.Model(&model.Account{}).
		Joins("JOIN account_links ON account_links.funding_account_id = accounts.id").
		Where("account_links.ledger_id = ? AND account_links.payment_account_id = ? AND accounts.deleted_at IS NULL", ledgerID, paymentID).
		Order("accounts.sort asc, accounts.id asc").Find(&result).Error
	return result, err
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
	var linkCount int64
	s.db.Model(&model.AccountLink{}).
		Where("ledger_id = ? AND (payment_account_id = ? OR funding_account_id = ?)", ledgerID, id, id).
		Count(&linkCount)
	if linkCount > 0 {
		return errors.New("该账户仍被支付宝或微信关联，请先解除关联")
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
	for i := range accounts {
		if accounts[i].Type == "alipay" || accounts[i].Type == "wechat" {
			accounts[i].LinkedAccounts, _ = s.linkedAccounts(ledgerID, accounts[i].ID)
		}
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
