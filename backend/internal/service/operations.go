package service

import (
	"errors"
	"fmt"
	"time"

	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OperationsService 为 ECS 上的只读运维 CLI 提供脱敏查询。
type OperationsService struct {
	db *gorm.DB
}

func NewOperationsService() *OperationsService {
	return &OperationsService{db: repository.GetDB()}
}

type OpsUserSummary struct {
	UserID         uint            `gorm:"column:user_id"`
	Username       string          `gorm:"column:username"`
	Nickname       string          `gorm:"column:nickname"`
	LoginType      string          `gorm:"column:login_type"`
	AccountCount   int64           `gorm:"column:account_count"`
	BillCount      int64           `gorm:"column:bill_count"`
	ExpenseTotal   decimal.Decimal `gorm:"column:expense_total"`
	IncomeTotal    decimal.Decimal `gorm:"column:income_total"`
	LastOccurredAt *time.Time      `gorm:"column:last_occurred_at"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
}

// ListUserSummaries 返回注册用户总数以及当前页的个人账本汇总。
// 同步副本不计入，避免同一笔个人流水在共享账本中重复统计。
func (s *OperationsService) ListUserSummaries(page, pageSize int) ([]OpsUserSummary, int64, error) {
	page, pageSize, err := normalizeOpsPage(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.db.Table("users").Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询用户总数失败: %w", err)
	}

	var result []OpsUserSummary
	err = s.db.Table("users u").
		Select(`u.id AS user_id, u.username, u.nickname,
			CASE WHEN u.wechat_open_id IS NULL THEN 'password' ELSE 'wechat' END AS login_type,
			(SELECT COUNT(*) FROM accounts a
			 JOIN ledgers al ON al.id = a.ledger_id AND al.deleted_at IS NULL
			 WHERE al.owner_id = u.id AND al.is_personal = ? AND a.deleted_at IS NULL) AS account_count,
			COUNT(t.id) AS bill_count,
			COALESCE(SUM(CASE WHEN t.op_type IN ('expense', 'borrow_out') THEN t.amount ELSE 0 END), 0) AS expense_total,
			COALESCE(SUM(CASE WHEN t.op_type IN ('income', 'borrow_in') THEN t.amount ELSE 0 END), 0) AS income_total,
			u.created_at`, true).
		Joins("LEFT JOIN ledgers l ON l.owner_id = u.id AND l.is_personal = ? AND l.deleted_at IS NULL", true).
		Joins("LEFT JOIN transactions t ON t.ledger_id = l.id AND t.source_id IS NULL AND t.deleted_at IS NULL").
		Where("u.deleted_at IS NULL").
		Group("u.id, u.username, u.nickname, u.wechat_open_id, u.created_at").
		Order("u.id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&result).Error
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户账单汇总失败: %w", err)
	}
	// SQLite 对 MAX(datetime) 返回字符串，而 MySQL 返回时间类型。逐用户查询最新一笔
	// 既能兼容两种数据库，也只会对当前页（最多 200 个用户）产生有限查询。
	for i := range result {
		var latest []struct {
			OccurredAt time.Time `gorm:"column:occurred_at"`
		}
		if err := s.db.Table("transactions t").
			Select("t.occurred_at").
			Joins("JOIN ledgers l ON l.id = t.ledger_id AND l.owner_id = ? AND l.is_personal = ? AND l.deleted_at IS NULL", result[i].UserID, true).
			Where("t.deleted_at IS NULL AND t.source_id IS NULL").
			Order("t.occurred_at DESC, t.id DESC").
			Limit(1).
			Scan(&latest).Error; err != nil {
			return nil, 0, fmt.Errorf("查询用户最近账单失败: %w", err)
		}
		if len(latest) > 0 {
			result[i].LastOccurredAt = &latest[0].OccurredAt
		}
	}
	return result, total, nil
}

type OpsBillFilter struct {
	UserID   uint
	Month    string
	Page     int
	PageSize int
}

type OpsBill struct {
	ID            uint            `gorm:"column:id"`
	OwnerUserID   uint            `gorm:"column:owner_user_id"`
	OwnerUsername string          `gorm:"column:owner_username"`
	OwnerNickname string          `gorm:"column:owner_nickname"`
	OccurredAt    time.Time       `gorm:"column:occurred_at"`
	OpType        string          `gorm:"column:op_type"`
	Amount        decimal.Decimal `gorm:"column:amount"`
	AccountName   string          `gorm:"column:account_name"`
	CategoryName  string          `gorm:"column:category_name"`
	Remark        string          `gorm:"column:remark"`
}

// ListBills 分页查询个人账本原始流水；UserID=0 表示全部用户。
func (s *OperationsService) ListBills(filter OpsBillFilter) ([]OpsBill, int64, error) {
	page, pageSize, err := normalizeOpsPage(filter.Page, filter.PageSize)
	if err != nil {
		return nil, 0, err
	}
	base := s.db.Table("transactions t").
		Joins("JOIN ledgers l ON l.id = t.ledger_id AND l.is_personal = ? AND l.deleted_at IS NULL", true).
		Joins("JOIN users u ON u.id = l.owner_id AND u.deleted_at IS NULL").
		Where("t.deleted_at IS NULL AND t.source_id IS NULL")
	if filter.UserID > 0 {
		base = base.Where("u.id = ?", filter.UserID)
	}
	if filter.Month != "" {
		month, parseErr := time.ParseInLocation("2006-01", filter.Month, time.Local)
		if parseErr != nil {
			return nil, 0, errors.New("月份格式必须为 YYYY-MM")
		}
		base = base.Where("t.occurred_at >= ? AND t.occurred_at < ?", month, month.AddDate(0, 1, 0))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询账单总数失败: %w", err)
	}
	var result []OpsBill
	err = base.
		Select(`t.id, u.id AS owner_user_id, u.username AS owner_username, u.nickname AS owner_nickname,
			t.occurred_at, t.op_type, t.amount, COALESCE(a.name, '') AS account_name,
			COALESCE(c.name, '') AS category_name, t.remark`).
		Joins("LEFT JOIN accounts a ON a.id = t.account_id").
		Joins("LEFT JOIN categories c ON c.id = t.category_id").
		Order("t.occurred_at DESC, t.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&result).Error
	if err != nil {
		return nil, 0, fmt.Errorf("查询账单记录失败: %w", err)
	}
	return result, total, nil
}

func normalizeOpsPage(page, pageSize int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		return 0, 0, errors.New("page-size 不能超过 200")
	}
	return page, pageSize, nil
}
