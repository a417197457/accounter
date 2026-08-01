package service

import (
	"fmt"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StatsService 统计服务
type StatsService struct {
	db *gorm.DB
}

// NewStatsService 创建统计服务
func NewStatsService() *StatsService {
	return &StatsService{db: repository.GetDB()}
}

// periodExpr 按方言返回日期格式化表达式
func (s *StatsService) periodExpr(field string, granularity string) string {
	isMySQL := config.Get().DB.Driver == "mysql"
	if granularity == "month" {
		if isMySQL {
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m')", field)
		}
		return fmt.Sprintf("strftime('%%Y-%%m', %s)", field)
	}
	// day
	if isMySQL {
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d')", field)
	}
	return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", field)
}

// Overview 账本总览
type Overview struct {
	Income      decimal.Decimal `json:"income"`
	Expense     decimal.Decimal `json:"expense"`
	Balance     decimal.Decimal `json:"balance"`
	TotalAssets decimal.Decimal `json:"total_assets"`
}

// Overview 账本总览（全部流水的收入/支出/结余 + 未归档账户余额合计）
func (s *StatsService) Overview(ledgerID uint) (*Overview, error) {
	var income, expense decimal.Decimal

	rows := []struct {
		OpType string
		Sum    decimal.Decimal
	}{}
	err := s.db.Model(&model.Transaction{}).
		Select("op_type, COALESCE(SUM(amount), 0) as sum").
		Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Group("op_type").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("查询总览失败: %w", err)
	}

	for _, r := range rows {
		switch r.OpType {
		case "income", "borrow_in":
			income = income.Add(r.Sum)
		case "expense", "borrow_out", "transfer":
			expense = expense.Add(r.Sum)
		}
	}

	// 总资产 = 未归档账户余额合计
	var totalAssets decimal.Decimal
	err = s.db.Model(&model.Account{}).
		Where("ledger_id = ? AND is_archived = ? AND deleted_at IS NULL", ledgerID, false).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&totalAssets).Error
	if err != nil {
		return nil, fmt.Errorf("查询总资产失败: %w", err)
	}

	return &Overview{
		Income:      income,
		Expense:     expense,
		Balance:     income.Sub(expense),
		TotalAssets: totalAssets,
	}, nil
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Period  string          `json:"period"`
	Income  decimal.Decimal `json:"income"`
	Expense decimal.Decimal `json:"expense"`
}

// Trend 收支趋势（按天或按月）
func (s *StatsService) Trend(ledgerID uint, granularity string, start, end *time.Time) ([]TrendPoint, error) {
	if granularity == "" {
		granularity = "day"
	}
	periodExpr := s.periodExpr("occurred_at", granularity)

	query := s.db.Model(&model.Transaction{}).
		Select(fmt.Sprintf("%s as period, op_type, COALESCE(SUM(amount), 0) as sum", periodExpr)).
		Where("ledger_id = ? AND deleted_at IS NULL", ledgerID)
	if start != nil {
		query = query.Where("occurred_at >= ?", start)
	}
	if end != nil {
		query = query.Where("occurred_at <= ?", end)
	}
	query = query.Group("period, op_type").Order("period asc")

	var rows []struct {
		Period string
		OpType string
		Sum    decimal.Decimal
	}
	err := query.Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("查询趋势失败: %w", err)
	}

	// 合并同 period 的 income/expense
	m := map[string]*TrendPoint{}
	order := []string{}
	for _, r := range rows {
		if _, ok := m[r.Period]; !ok {
			m[r.Period] = &TrendPoint{Period: r.Period}
			order = append(order, r.Period)
		}
		switch r.OpType {
		case "income", "borrow_in":
			m[r.Period].Income = m[r.Period].Income.Add(r.Sum)
		case "expense", "borrow_out", "transfer":
			m[r.Period].Expense = m[r.Period].Expense.Add(r.Sum)
		}
	}

	result := []TrendPoint{}
	for _, p := range order {
		result = append(result, *m[p])
	}
	return result, nil
}

// CategorySlice 分类统计切片
type CategorySlice struct {
	CategoryID   uint            `json:"category_id"`
	CategoryName string          `json:"category_name"`
	Icon         string          `json:"icon"`
	Amount       decimal.Decimal `json:"amount"`
}

// CategoryBreakdown 分类占比（把子类金额归并到一级分类）
func (s *StatsService) CategoryBreakdown(ledgerID uint, kind string, start, end *time.Time) ([]CategorySlice, error) {
	query := s.db.Model(&model.Transaction{}).
		Select(fmt.Sprintf(`COALESCE(parent.id, transactions.category_id) as category_id,
			COALESCE(parent.name, category.name) as category_name,
			COALESCE(parent.icon, category.icon) as icon,
			COALESCE(SUM(transactions.amount), 0) as amount`)).
		Joins("LEFT JOIN categories category ON category.id = transactions.category_id").
		Joins("LEFT JOIN categories parent ON parent.id = category.parent_id").
		Where("transactions.ledger_id = ? AND transactions.deleted_at IS NULL", ledgerID)
	if kind != "" {
		query = query.Where("category.kind = ?", kind)
	}
	if start != nil {
		query = query.Where("transactions.occurred_at >= ?", start)
	}
	if end != nil {
		query = query.Where("transactions.occurred_at <= ?", end)
	}
	query = query.Group("1, 2").
		Order("amount desc")

	var result []CategorySlice
	err := query.Scan(&result).Error
	if err != nil {
		return nil, fmt.Errorf("查询分类占比失败: %w", err)
	}
	// 无数据返回 [] 而非 nil（否则前端 .map 崩）
	if result == nil {
		result = []CategorySlice{}
	}
	return result, nil
}

// MonthlyPoint 月度对比点
type MonthlyPoint struct {
	Month   string          `json:"month"`
	Income  decimal.Decimal `json:"income"`
	Expense decimal.Decimal `json:"expense"`
}

// Monthly 近 N 月收支对比（补齐缺月为 0）
func (s *StatsService) Monthly(ledgerID uint, months int) ([]MonthlyPoint, error) {
	if months <= 0 {
		months = 6
	}
	now := time.Now()
	// 从 N 个月前的第一天开始
	startMonth := time.Date(now.Year(), now.Month()-time.Month(months-1), 1, 0, 0, 0, 0, now.Location())

	periodExpr := s.periodExpr("occurred_at", "month")
	var rows []struct {
		Period string
		OpType string
		Sum    decimal.Decimal
	}
	err := s.db.Model(&model.Transaction{}).
		Select(fmt.Sprintf("%s as period, op_type, COALESCE(SUM(amount), 0) as sum", periodExpr)).
		Where("ledger_id = ? AND deleted_at IS NULL AND occurred_at >= ?", ledgerID, startMonth).
		Group("period, op_type").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("查询月度对比失败: %w", err)
	}

	m := map[string]*MonthlyPoint{}
	for _, r := range rows {
		if _, ok := m[r.Period]; !ok {
			m[r.Period] = &MonthlyPoint{Month: r.Period}
		}
		switch r.OpType {
		case "income", "borrow_in":
			m[r.Period].Income = m[r.Period].Income.Add(r.Sum)
		case "expense", "borrow_out", "transfer":
			m[r.Period].Expense = m[r.Period].Expense.Add(r.Sum)
		}
	}

	// 补齐缺月
	result := []MonthlyPoint{}
	for i := 0; i < months; i++ {
		t := time.Date(now.Year(), now.Month()-time.Month(months-1-i), 1, 0, 0, 0, 0, now.Location())
		key := t.Format("2006-01")
		if p, ok := m[key]; ok {
			result = append(result, *p)
		} else {
			result = append(result, MonthlyPoint{Month: key})
		}
	}
	return result, nil
}
