package model

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// User 用户
type User struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Nickname     string         `gorm:"size:64" json:"nickname"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Ledger 账本
type Ledger struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	Name       string         `gorm:"size:64;not null" json:"name"`
	OwnerID    uint           `gorm:"not null;index" json:"owner_id"`
	IsPersonal bool           `gorm:"default:false" json:"is_personal"`
	InviteCode string         `gorm:"size:16;uniqueIndex" json:"invite_code,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Membership 账本成员关系
type Membership struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	LedgerID  uint      `gorm:"not null;uniqueIndex:idx_ledger_user" json:"ledger_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_ledger_user" json:"user_id"`
	Role      string    `gorm:"size:16;not null" json:"role"` // owner | member
	Alias     string    `gorm:"size:64" json:"alias"`         // 账本内昵称
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Account 账户
type Account struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	LedgerID   uint           `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID     uint           `gorm:"not null" json:"user_id"` // 创建者
	Name       string         `gorm:"size:64;not null" json:"name"`
	Type       string         `gorm:"size:32" json:"type"` // cash | bank | alipay | wechat | credit | ...
	Balance    decimal.Decimal `gorm:"type:decimal(18,2);not null;default:0" json:"balance"`
	Sort       int            `gorm:"default:0" json:"sort"`
	IsArchived bool           `gorm:"default:false" json:"is_archived"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Category 分类（二级树）
type Category struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	LedgerID   uint           `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID     uint           `gorm:"not null" json:"user_id"`
	Name       string         `gorm:"size:64;not null" json:"name"`
	ParentID   *uint          `gorm:"index" json:"parent_id"` // nil 为一级分类
	Kind       string         `gorm:"size:16;not null" json:"kind"` // income | expense
	Icon       string         `gorm:"size:64" json:"icon"`
	Sort       int            `gorm:"default:0" json:"sort"`
	Children   []Category     `gorm:"-" json:"children,omitempty"` // 内存组装，不落库
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Transaction 流水
type Transaction struct {
	ID          uint            `gorm:"primarykey" json:"id"`
	LedgerID    uint            `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID      uint            `gorm:"not null;index" json:"user_id"` // 记账人/归属成员
	AccountID   uint            `gorm:"not null;index" json:"account_id"`
	ToAccountID *uint           `gorm:"index" json:"to_account_id,omitempty"` // 转账转入账户
	CategoryID  *uint           `gorm:"index" json:"category_id,omitempty"`   // 转账时为 nil
	OpType      string          `gorm:"size:16;not null" json:"op_type"`      // expense|income|borrow_in|borrow_out|transfer
	Amount      decimal.Decimal `gorm:"type:decimal(18,2);not null" json:"amount"`
	Remark      string          `gorm:"size:255" json:"remark"`
	OccurredAt  time.Time       `gorm:"not null;index" json:"occurred_at"`
	SourceID    *uint           `gorm:"index" json:"source_id,omitempty"` // 非空表示从别的账本同步来的副本
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`

	// 关联（Preload 用）
	Account  *Account    `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	Category *Category   `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Member   *User       `gorm:"foreignKey:UserID" json:"member,omitempty"`
}

// Budget 月度循环预算（不绑定具体月份）
type Budget struct {
	ID         uint            `gorm:"primarykey" json:"id"`
	LedgerID   uint            `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID     uint            `gorm:"not null" json:"user_id"`
	CategoryID *uint           `gorm:"index" json:"category_id,omitempty"` // nil 表示总预算
	Amount     decimal.Decimal `gorm:"type:decimal(18,2);not null" json:"amount"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// Setting 全局 KV 配置（AI 配置持久化用）
type Setting struct {
	Key       string    `gorm:"primarykey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AllModels 返回所有模型（用于 AutoMigrate）
func AllModels() []interface{} {
	return []interface{}{
		&User{},
		&Ledger{},
		&Membership{},
		&Account{},
		&Category{},
		&Transaction{},
		&Budget{},
		&Setting{},
	}
}

// ValidOpType 校验操作类型
func ValidOpType(op string) bool {
	switch op {
	case "expense", "income", "borrow_in", "borrow_out", "transfer":
		return true
	}
	return false
}

// ValidCategoryKind 校验分类类型
func ValidCategoryKind(kind string) bool {
	return kind == "income" || kind == "expense"
}

// ValidRole 校验角色
func ValidRole(role string) bool {
	return role == "owner" || role == "member"
}

// BalanceDelta 计算余额变化
// income/borrow_in 为 +amount，expense/borrow_out/transfer 为 -amount
func BalanceDelta(opType string, amount decimal.Decimal) decimal.Decimal {
	switch opType {
	case "income", "borrow_in":
		return amount
	case "expense", "borrow_out", "transfer":
		return amount.Neg()
	}
	return decimal.Zero
}

// GenInviteCode 生成 6 位邀请码（去掉易混字符 0/1/I/O），避免空字符串触发 UNIQUE 冲突
func GenInviteCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	result := make([]byte, 6)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}
