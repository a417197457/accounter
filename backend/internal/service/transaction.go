package service

import (
	"errors"
	"fmt"
	"time"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TransactionService 流水服务
type TransactionService struct {
	db *gorm.DB
}

// NewTransactionService 创建流水服务
func NewTransactionService() *TransactionService {
	return &TransactionService{db: repository.GetDB()}
}

// validate 校验流水数据
func (s *TransactionService) validate(ledgerID uint, t *model.Transaction) error {
	// 金额必须 > 0
	if t.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("金额必须大于 0")
	}

	// 操作类型校验
	if !model.ValidOpType(t.OpType) {
		return errors.New("无效的操作类型")
	}

	// 账户归属校验
	var acc model.Account
	if err := s.db.First(&acc, "id = ? AND ledger_id = ?", t.AccountID, ledgerID).Error; err != nil {
		return errors.New("账户不存在或不属于该账本")
	}

	// 转账需要转入账户
	if t.OpType == "transfer" {
		if t.ToAccountID == nil {
			return errors.New("转账需要转入账户")
		}
		if *t.ToAccountID == t.AccountID {
			return errors.New("转入账户不能与转出账户相同")
		}
		var toAcc model.Account
		if err := s.db.First(&toAcc, "id = ? AND ledger_id = ?", *t.ToAccountID, ledgerID).Error; err != nil {
			return errors.New("转入账户不存在或不属于该账本")
		}
	}

	// 非转账需要分类
	if t.OpType != "transfer" && t.CategoryID == nil {
		return errors.New("非转账需要分类")
	}

	// 分类归属校验
	if t.CategoryID != nil {
		var cat model.Category
		if err := s.db.First(&cat, "id = ? AND ledger_id = ?", *t.CategoryID, ledgerID).Error; err != nil {
			return errors.New("分类不存在或不属于该账本")
		}
	}

	return nil
}

// applyBalance 调整账户余额（在事务内调用）
func (s *TransactionService) applyBalance(tx *gorm.DB, t *model.Transaction, sign int) error {
	delta := model.BalanceDelta(t.OpType, t.Amount)
	if sign < 0 {
		delta = delta.Neg()
	}

	// 调整转出账户余额
	if err := tx.Model(&model.Account{}).
		Where("id = ? AND ledger_id = ?", t.AccountID, t.LedgerID).
		Update("balance", gorm.Expr("balance + ?", delta)).Error; err != nil {
		return fmt.Errorf("调整账户余额失败: %w", err)
	}

	// 转账额外调整转入账户余额
	if t.OpType == "transfer" && t.ToAccountID != nil {
		if err := tx.Model(&model.Account{}).
			Where("id = ? AND ledger_id = ?", *t.ToAccountID, t.LedgerID).
			Update("balance", gorm.Expr("balance + ?", delta.Neg())).Error; err != nil {
			return fmt.Errorf("调整转入账户余额失败: %w", err)
		}
	}

	return nil
}

// Create 创建流水
func (s *TransactionService) Create(ledgerID, userID uint, t *model.Transaction) (*model.Transaction, error) {
	t.LedgerID = ledgerID
	t.UserID = userID

	// 校验
	if err := s.validate(ledgerID, t); err != nil {
		return nil, err
	}

	// 事务内创建 + 调整余额
	var result *model.Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("创建流水失败: %w", err)
		}
		if err := s.applyBalance(tx, t, 1); err != nil {
			return err
		}
		result = t
		return nil
	})

	return result, err
}

// Update 更新流水
func (s *TransactionService) Update(ledgerID, userID, id uint, next *model.Transaction) (*model.Transaction, error) {
	// 查询原流水
	var old model.Transaction
	if err := s.db.First(&old, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("流水不存在")
	}

	// 校验新数据
	next.LedgerID = ledgerID
	next.UserID = old.UserID // 保留原记账人
	if err := s.validate(ledgerID, next); err != nil {
		return nil, err
	}

	var result *model.Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 撤销原余额
		if err := s.applyBalance(tx, &old, -1); err != nil {
			return err
		}

		// 更新流水（保留 CreatedAt）
		next.ID = old.ID
		next.UserID = old.UserID
		next.CreatedAt = old.CreatedAt
		if err := tx.Save(next).Error; err != nil {
			return fmt.Errorf("更新流水失败: %w", err)
		}

		// 应用新余额
		if err := s.applyBalance(tx, next, 1); err != nil {
			return err
		}

		result = next
		return nil
	})

	return result, err
}

// Delete 删除流水
func (s *TransactionService) Delete(ledgerID, userID, id uint) error {
	var t model.Transaction
	if err := s.db.First(&t, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return errors.New("流水不存在")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 撤销余额
		if err := s.applyBalance(tx, &t, -1); err != nil {
			return err
		}
		// 删除流水
		if err := tx.Delete(&t).Error; err != nil {
			return fmt.Errorf("删除流水失败: %w", err)
		}
		return nil
	})
}

// ListRequest 流水列表请求参数
// start/end/account_id 等由 handler 手动解析，避免 Gin 把 YYYY-MM-DD 按 RFC3339 绑定失败
type ListRequest struct {
	Start      *time.Time `form:"-"`
	End        *time.Time `form:"-"`
	AccountID  *uint      `form:"-"`
	CategoryID *uint      `form:"-"`
	MemberID   *uint      `form:"-"`
	OpType     string     `form:"op_type"`
	Keyword    string     `form:"keyword"`
	Page       int        `form:"page"`
	PageSize   int        `form:"page_size"`
}

// List 获取流水列表
func (s *TransactionService) List(ledgerID uint, req *ListRequest) ([]model.Transaction, int64, error) {
	query := s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID)

	// 时间筛选
	if req.Start != nil {
		query = query.Where("occurred_at >= ?", req.Start)
	}
	if req.End != nil {
		query = query.Where("occurred_at <= ?", req.End)
	}

	// 账户筛选：本账户支出/收入，或转账转入本账户
	if req.AccountID != nil {
		id := *req.AccountID
		query = query.Where("account_id = ? OR to_account_id = ?", id, id)
	}

	// 分类筛选
	if req.CategoryID != nil {
		query = query.Where("category_id = ?", *req.CategoryID)
	}

	// 成员筛选
	if req.MemberID != nil {
		query = query.Where("user_id = ?", *req.MemberID)
	}

	// 操作类型筛选
	if req.OpType != "" {
		query = query.Where("op_type = ?", req.OpType)
	}

	// 关键词搜索（备注）
	if req.Keyword != "" {
		query = query.Where("remark LIKE ?", "%"+req.Keyword+"%")
	}

	// 总数
	var total int64
	query.Model(&model.Transaction{}).Count(&total)

	// 分页
	offset := (req.Page - 1) * req.PageSize
	var transactions []model.Transaction
	err := query.Preload("Account").Preload("Category").Preload("Member").
		Order("occurred_at desc, id desc").
		Offset(offset).Limit(req.PageSize).
		Find(&transactions).Error

	if err != nil {
		return nil, 0, fmt.Errorf("查询流水列表失败: %w", err)
	}

	return transactions, total, nil
}

// Get 获取单条流水
func (s *TransactionService) Get(ledgerID, id uint) (*model.Transaction, error) {
	var t model.Transaction
	err := s.db.Preload("Account").Preload("Category").Preload("Member").
		First(&t, "id = ? AND ledger_id = ?", id, ledgerID).Error
	if err != nil {
		return nil, errors.New("流水不存在")
	}
	return &t, nil
}
