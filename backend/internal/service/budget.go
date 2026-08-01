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

// BudgetService 预算服务
type BudgetService struct {
	db *gorm.DB
}

// NewBudgetService 创建预算服务
func NewBudgetService() *BudgetService {
	return &BudgetService{db: repository.GetDB()}
}

// Set 设置预算（upsert，categoryID nil=总预算）
func (s *BudgetService) Set(ledgerID, userID uint, categoryID *uint, amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("预算金额必须大于 0")
	}

	// 查找是否已存在
	var budget model.Budget
	err := s.db.Where("ledger_id = ? AND user_id = ? AND (category_id = ? OR (category_id IS NULL AND ? IS NULL))",
		ledgerID, userID, categoryID, categoryID).First(&budget).Error

	if err == gorm.ErrRecordNotFound {
		// 新建
		budget = model.Budget{
			LedgerID:   ledgerID,
			UserID:     userID,
			CategoryID: categoryID,
			Amount:     amount,
		}
		return s.db.Create(&budget).Error
	}
	if err != nil {
		return fmt.Errorf("查询预算失败: %w", err)
	}

	// 更新
	budget.Amount = amount
	return s.db.Save(&budget).Error
}

// BudgetWithSpent 预算 + 本月已用
type BudgetWithSpent struct {
	model.Budget
	CategoryName string          `json:"category_name,omitempty"`
	Spent        decimal.Decimal `json:"spent"`
}

// ListBudgets 列出该账本的所有预算（附本月已用）
func (s *BudgetService) ListBudgets(ledgerID uint) ([]BudgetWithSpent, error) {
	var budgets []model.Budget
	err := s.db.Where("ledger_id = ?", ledgerID).Find(&budgets).Error
	if err != nil {
		return nil, fmt.Errorf("查询预算列表失败: %w", err)
	}

	// 本月范围
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := monthStart.AddDate(0, 1, 0)

	result := []BudgetWithSpent{}
	for _, b := range budgets {
		var spent decimal.Decimal

		if b.CategoryID == nil {
			// 总预算 = 本月总支出
			err = s.db.Model(&model.Transaction{}).
				Where("ledger_id = ? AND op_type = ? AND occurred_at >= ? AND occurred_at < ? AND deleted_at IS NULL",
					ledgerID, "expense", monthStart, nextMonth).
				Select("COALESCE(SUM(amount), 0)").
				Scan(&spent).Error
		} else {
			// 分类预算 = 本月该分类（含子分类）支出
			err = s.db.Model(&model.Transaction{}).
				Joins("LEFT JOIN categories c ON c.id = transactions.category_id").
				Where("transactions.ledger_id = ? AND transactions.op_type = ? AND transactions.occurred_at >= ? AND transactions.occurred_at < ? AND transactions.deleted_at IS NULL AND (c.id = ? OR c.parent_id = ?)",
					ledgerID, "expense", monthStart, nextMonth, b.CategoryID, b.CategoryID).
				Select("COALESCE(SUM(transactions.amount), 0)").
				Scan(&spent).Error
		}
		if err != nil {
			return nil, fmt.Errorf("查询预算已用失败: %w", err)
		}

		item := BudgetWithSpent{
			Budget: b,
			Spent:  spent,
		}
		// 填分类名
		if b.CategoryID != nil {
			var cat model.Category
			s.db.First(&cat, *b.CategoryID)
			item.CategoryName = cat.Name
		} else {
			item.CategoryName = "总预算"
		}
		result = append(result, item)
	}
	if result == nil {
		result = []BudgetWithSpent{}
	}
	return result, nil
}

// Delete 删除预算
func (s *BudgetService) Delete(ledgerID, id uint) error {
	return s.db.Where("id = ? AND ledger_id = ?", id, ledgerID).Delete(&model.Budget{}).Error
}
