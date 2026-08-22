package model

import (
	"crypto/rand"
	"fmt"
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
	Phone        *string        `gorm:"uniqueIndex;size:32" json:"phone,omitempty"`
	WechatOpenID *string        `gorm:"column:wechat_open_id;uniqueIndex;size:128" json:"-"`
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
	Type       string         `gorm:"size:16;not null;default:family;index" json:"type"`
	InviteCode string         `gorm:"size:16;uniqueIndex" json:"invite_code,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

const (
	LedgerTypePersonal = "personal"
	LedgerTypeFamily   = "family"
	LedgerTypeTravel   = "travel"
	LedgerTypeGift     = "gift"
)

// ValidLedgerType 检查账本类型是否为系统支持的类型。
func ValidLedgerType(ledgerType string) bool {
	switch ledgerType {
	case LedgerTypePersonal, LedgerTypeFamily, LedgerTypeTravel, LedgerTypeGift:
		return true
	default:
		return false
	}
}

// BeforeSave 兼容旧数据和旧调用方，并确保 type 与 is_personal 始终一致。
func (l *Ledger) BeforeSave(_ *gorm.DB) error {
	if l.Type == "" {
		if l.IsPersonal {
			l.Type = LedgerTypePersonal
		} else {
			l.Type = LedgerTypeFamily
		}
	}
	if !ValidLedgerType(l.Type) {
		return fmt.Errorf("不支持的账本类型: %s", l.Type)
	}
	l.IsPersonal = l.Type == LedgerTypePersonal
	return nil
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
	ID         uint            `gorm:"primarykey" json:"id"`
	LedgerID   uint            `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID     uint            `gorm:"not null" json:"user_id"` // 创建者
	Name       string          `gorm:"size:64;not null" json:"name"`
	Type       string          `gorm:"size:32" json:"type"` // alipay | wechat | cash | savings | credit
	Balance    decimal.Decimal `gorm:"type:decimal(18,2);not null;default:0" json:"balance"`
	Sort       int             `gorm:"default:0" json:"sort"`
	IsArchived bool            `gorm:"default:false" json:"is_archived"`
	// 仅信用卡：账单日（使用周期闭合日）、还款日，取值 1–28
	StatementDay *int           `gorm:"column:statement_day" json:"statement_day,omitempty"`
	DueDay       *int           `gorm:"column:due_day" json:"due_day,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 支付宝/微信作为支付渠道时，可关联储蓄卡或信用卡。
	// 该字段由账户列表服务填充，不直接落在 accounts 表。
	LinkedAccounts []Account `gorm:"-" json:"linked_accounts,omitempty"`
}

// AccountLink 支付渠道与实际资金账户的关联。
// PaymentAccount 只能是 alipay/wechat，FundingAccount 只能是 savings/credit。
type AccountLink struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	LedgerID         uint      `gorm:"not null;index;default:0;uniqueIndex:idx_account_link" json:"ledger_id"`
	PaymentAccountID uint      `gorm:"not null;index;uniqueIndex:idx_account_link" json:"payment_account_id"`
	FundingAccountID uint      `gorm:"not null;index;uniqueIndex:idx_account_link" json:"funding_account_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// Category 分类（二级树）
type Category struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	LedgerID  uint           `gorm:"not null;index;default:0" json:"ledger_id"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	Name      string         `gorm:"size:64;not null" json:"name"`
	ParentID  *uint          `gorm:"index" json:"parent_id"`       // nil 为一级分类
	Kind      string         `gorm:"size:16;not null" json:"kind"` // income | expense
	Icon      string         `gorm:"size:64" json:"icon"`
	Sort      int            `gorm:"default:0" json:"sort"`
	Children  []Category     `gorm:"-" json:"children,omitempty"` // 内存组装，不落库
	Parent    *Category      `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Transaction 流水
type Transaction struct {
	ID               uint            `gorm:"primarykey" json:"id"`
	LedgerID         uint            `gorm:"not null;index:idx_tx_ledger;index:idx_ledger_occurred,priority:1;index:idx_ledger_op,priority:1;default:0" json:"ledger_id"`
	UserID           uint            `gorm:"not null;index" json:"user_id"` // 记账人/归属成员
	AccountID        uint            `gorm:"not null;index" json:"account_id"`
	PaymentAccountID *uint           `gorm:"index" json:"payment_account_id,omitempty"`                      // 支付宝/微信渠道；余额支付时为空
	ToAccountID      *uint           `gorm:"index" json:"to_account_id,omitempty"`                           // 转账转入账户
	CategoryID       *uint           `gorm:"index" json:"category_id,omitempty"`                             // 转账时为 nil
	OpType           string          `gorm:"size:16;not null;index:idx_ledger_op,priority:2" json:"op_type"` // expense|income|borrow_in|borrow_out|transfer
	Amount           decimal.Decimal `gorm:"type:decimal(18,2);not null" json:"amount"`
	Remark           string          `gorm:"size:255" json:"remark"`
	OccurredAt       time.Time       `gorm:"not null;index:idx_ledger_occurred,priority:2" json:"occurred_at"`
	SourceID         *uint           `gorm:"index" json:"source_id,omitempty"` // 非空表示从别的账本同步来的副本
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"-"`

	// 关联（Preload 用）
	Account        *Account  `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	PaymentAccount *Account  `gorm:"foreignKey:PaymentAccountID" json:"payment_account,omitempty"`
	ToAccount      *Account  `gorm:"foreignKey:ToAccountID" json:"to_account,omitempty"`
	Category       *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Member         *User     `gorm:"foreignKey:UserID" json:"member,omitempty"`
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

// BookkeepingPreference 控制 AI 在用户未说明支付账户时的处理方式。
// default：使用指定默认支付路径；strict：必须由用户明确余额或具体卡。
type BookkeepingPreference struct {
	ID                      uint      `gorm:"primarykey" json:"id"`
	LedgerID                uint      `gorm:"not null;default:0;uniqueIndex" json:"ledger_id"`
	UserID                  uint      `gorm:"not null;index" json:"user_id"`
	AccountMode             string    `gorm:"size:16;not null;default:default" json:"account_mode"`
	DefaultAccountID        *uint     `gorm:"index" json:"default_account_id,omitempty"`
	DefaultPaymentAccountID *uint     `gorm:"index" json:"default_payment_account_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Setting 全局 KV 配置（AI 配置持久化用）
type Setting struct {
	Key       string    `gorm:"primarykey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AIEntitlement 保存用户级 AI 试用和永久激活权益。
// 状态由时间字段推导，避免额外 status 字段与实际时间不一致。
type AIEntitlement struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	UserID           uint       `gorm:"not null;uniqueIndex" json:"user_id"`
	TrialStartedAt   *time.Time `json:"trial_started_at,omitempty"`
	TrialExpiresAt   *time.Time `gorm:"index" json:"trial_expires_at,omitempty"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty"`
	ActivationCodeID *uint      `gorm:"index" json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ActivationCode 只保存激活码 HMAC，不保存可兑换的明文。
type ActivationCode struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	CodeHash         string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	DisplaySuffix    string     `gorm:"size:8;not null;index" json:"display_suffix"`
	BatchName        string     `gorm:"size:64;index" json:"batch_name"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at,omitempty"`
	AssignedTo       string     `gorm:"size:128" json:"assigned_to,omitempty"`
	RedeemedByUserID *uint      `gorm:"index" json:"redeemed_by_user_id,omitempty"`
	RedeemedAt       *time.Time `gorm:"index" json:"redeemed_at,omitempty"`
	RevokedAt        *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	Note             string     `gorm:"size:255" json:"note,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// AIDailyUsage 为用户按自然日累计成功调用次数，限制异常消耗。
type AIDailyUsage struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_ai_usage_user_date" json:"user_id"`
	UsageDate    string    `gorm:"size:10;not null;uniqueIndex:idx_ai_usage_user_date" json:"usage_date"`
	RequestCount int       `gorm:"not null;default:0" json:"request_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AIActivationAttempt 记录激活尝试，用于按用户和来源 IP 限流及审计。
type AIActivationAttempt struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	IPHash      string    `gorm:"size:64;not null;index" json:"-"`
	CodeSuffix  string    `gorm:"size:8" json:"code_suffix,omitempty"`
	Success     bool      `gorm:"not null;default:false;index" json:"success"`
	FailureCode string    `gorm:"size:32" json:"failure_code,omitempty"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

// AllModels 返回所有模型（用于 AutoMigrate）
func AllModels() []interface{} {
	return []interface{}{
		&User{},
		&Ledger{},
		&Membership{},
		&Account{},
		&AccountLink{},
		&Category{},
		&Transaction{},
		&Budget{},
		&BookkeepingPreference{},
		&Setting{},
		&AIEntitlement{},
		&ActivationCode{},
		&AIDailyUsage{},
		&AIActivationAttempt{},
	}
}

// ValidAccountMode 校验 AI 记账账户策略。
func ValidAccountMode(mode string) bool {
	return mode == "default" || mode == "strict"
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

// ValidAccountType 账户只保留五种明确类型。
func ValidAccountType(accountType string) bool {
	switch accountType {
	case "alipay", "wechat", "cash", "savings", "credit":
		return true
	}
	return false
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
