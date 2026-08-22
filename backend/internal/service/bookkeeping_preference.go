package service

import (
	"errors"
	"fmt"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"gorm.io/gorm"
)

// BookkeepingPreferenceView 是前端和 AI 共用的已校验账户策略。
type BookkeepingPreferenceView struct {
	AccountMode               string `json:"account_mode"`
	DefaultAccountID          uint   `json:"default_account_id"`
	DefaultAccountName        string `json:"default_account_name"`
	DefaultPaymentAccountID   *uint  `json:"default_payment_account_id,omitempty"`
	DefaultPaymentAccountName string `json:"default_payment_account_name,omitempty"`
	DefaultAccountLabel       string `json:"default_account_label"`
}

type BookkeepingPreferenceService struct {
	db *gorm.DB
}

func NewBookkeepingPreferenceService() *BookkeepingPreferenceService {
	return &BookkeepingPreferenceService{db: repository.GetDB()}
}

// Get 获取策略；旧用户首次读取时自动创建“便捷模式 + 支付宝余额”。
// 如果默认账户后来被归档，会自动切换到当前第一个可用账户。
func (s *BookkeepingPreferenceService) Get(ledgerID, userID uint) (*BookkeepingPreferenceView, error) {
	var pref model.BookkeepingPreference
	err := s.db.Where("ledger_id = ?", ledgerID).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		account, pickErr := s.pickFallbackAccount(ledgerID)
		if pickErr != nil {
			return nil, pickErr
		}
		pref = model.BookkeepingPreference{
			LedgerID:         ledgerID,
			UserID:           userID,
			AccountMode:      "default",
			DefaultAccountID: &account.ID,
		}
		if err := s.db.Create(&pref).Error; err != nil {
			// 并发首次读取时以数据库中已经创建的配置为准。
			if retryErr := s.db.Where("ledger_id = ?", ledgerID).First(&pref).Error; retryErr != nil {
				return nil, fmt.Errorf("创建记账偏好失败: %w", err)
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("查询记账偏好失败: %w", err)
	}

	if !model.ValidAccountMode(pref.AccountMode) {
		pref.AccountMode = "default"
	}
	view, err := s.resolve(&pref)
	if err == nil {
		return view, nil
	}

	// 默认路径失效时自愈，避免账户归档后 AI 无法记账。
	account, pickErr := s.pickFallbackAccount(ledgerID)
	if pickErr != nil {
		return nil, pickErr
	}
	pref.DefaultAccountID = &account.ID
	pref.DefaultPaymentAccountID = nil
	if err := s.db.Model(&pref).Updates(map[string]any{
		"account_mode":               pref.AccountMode,
		"default_account_id":         account.ID,
		"default_payment_account_id": nil,
	}).Error; err != nil {
		return nil, fmt.Errorf("修复默认账户失败: %w", err)
	}
	return s.resolve(&pref)
}

func (s *BookkeepingPreferenceService) Update(
	ledgerID, userID uint,
	mode string,
	defaultAccountID uint,
	defaultPaymentAccountID *uint,
) (*BookkeepingPreferenceView, error) {
	if !model.ValidAccountMode(mode) {
		return nil, errors.New("记账方式只能是便捷模式或精确模式")
	}
	if defaultAccountID == 0 {
		return nil, errors.New("请选择默认账户")
	}

	pref := model.BookkeepingPreference{
		LedgerID:                ledgerID,
		UserID:                  userID,
		AccountMode:             mode,
		DefaultAccountID:        &defaultAccountID,
		DefaultPaymentAccountID: defaultPaymentAccountID,
	}
	view, err := s.resolve(&pref)
	if err != nil {
		return nil, err
	}

	var existing model.BookkeepingPreference
	err = s.db.Where("ledger_id = ?", ledgerID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.db.Create(&pref).Error; err != nil {
			return nil, fmt.Errorf("保存记账偏好失败: %w", err)
		}
		return view, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询记账偏好失败: %w", err)
	}
	if err := s.db.Model(&existing).Updates(map[string]any{
		"user_id":                    userID,
		"account_mode":               mode,
		"default_account_id":         defaultAccountID,
		"default_payment_account_id": defaultPaymentAccountID,
	}).Error; err != nil {
		return nil, fmt.Errorf("保存记账偏好失败: %w", err)
	}
	return view, nil
}

func (s *BookkeepingPreferenceService) pickFallbackAccount(ledgerID uint) (model.Account, error) {
	var account model.Account
	err := s.db.Where("ledger_id = ? AND type = ? AND is_archived = ?", ledgerID, "alipay", false).
		Order("sort asc, id asc").First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = s.db.Where("ledger_id = ? AND is_archived = ?", ledgerID, false).
			Order("sort asc, id asc").First(&account).Error
	}
	if err != nil {
		return model.Account{}, errors.New("个人账本下没有可用账户，请先新增账户")
	}
	return account, nil
}

func (s *BookkeepingPreferenceService) resolve(pref *model.BookkeepingPreference) (*BookkeepingPreferenceView, error) {
	if pref == nil || pref.DefaultAccountID == nil || *pref.DefaultAccountID == 0 {
		return nil, errors.New("请选择默认账户")
	}
	var account model.Account
	if err := s.db.First(&account,
		"id = ? AND ledger_id = ? AND is_archived = ?", *pref.DefaultAccountID, pref.LedgerID, false,
	).Error; err != nil {
		return nil, errors.New("默认账户不存在或已归档")
	}

	view := &BookkeepingPreferenceView{
		AccountMode:         pref.AccountMode,
		DefaultAccountID:    account.ID,
		DefaultAccountName:  account.Name,
		DefaultAccountLabel: account.Name,
	}
	if account.Type == "alipay" || account.Type == "wechat" {
		view.DefaultAccountLabel = account.Name + "余额"
	}

	if pref.DefaultPaymentAccountID == nil {
		return view, nil
	}
	var payment model.Account
	if err := s.db.First(&payment,
		"id = ? AND ledger_id = ? AND is_archived = ?", *pref.DefaultPaymentAccountID, pref.LedgerID, false,
	).Error; err != nil || (payment.Type != "alipay" && payment.Type != "wechat") {
		return nil, errors.New("默认支付渠道不存在或已归档")
	}
	var count int64
	s.db.Model(&model.AccountLink{}).Where(
		"ledger_id = ? AND payment_account_id = ? AND funding_account_id = ?",
		pref.LedgerID, payment.ID, account.ID,
	).Count(&count)
	if count == 0 {
		return nil, errors.New("默认支付渠道没有关联所选银行卡")
	}
	view.DefaultPaymentAccountID = &payment.ID
	view.DefaultPaymentAccountName = payment.Name
	view.DefaultAccountLabel = payment.Name + " · " + account.Name
	return view, nil
}
