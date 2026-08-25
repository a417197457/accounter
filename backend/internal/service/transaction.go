package service

import (
	"errors"
	"fmt"
	"time"

	"accounter/backend/internal/config"
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

var (
	errSharedDirectCreate = errors.New("该账本不能直接记账，请在个人账本记账后同步过来")
	errCopyReadOnly       = errors.New("同步副本只读，请在个人账本修改源流水")
	errTransferNoPush     = errors.New("转账/借入借出不支持同步到其他账本")
	errAlreadyPushed      = errors.New("该流水已同步到目标账本")
	errNotPersonalSource  = errors.New("只能从个人账本推送流水")
	errTargetMustShared   = errors.New("只能同步到家庭、旅游或礼账账本")
	errNotMember          = errors.New("你不是该账本成员")
)

// validate 校验流水数据
func (s *TransactionService) validate(ledgerID uint, t *model.Transaction) error {
	if t.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("金额必须大于 0")
	}
	if !model.ValidOpType(t.OpType) {
		return errors.New("无效的操作类型")
	}

	var acc model.Account
	if err := s.db.First(&acc, "id = ? AND ledger_id = ?", t.AccountID, ledgerID).Error; err != nil {
		return errors.New("账户不存在或不属于该账本")
	}

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
		// 还款：储蓄/支付账户 → 信用卡；禁止信用卡 → 信用卡
		if acc.Type == "credit" && toAcc.Type == "credit" {
			return errors.New("信用卡之间不能互转，请从储蓄账户转入信用卡还款")
		}
	}

	if t.PaymentAccountID != nil {
		if t.OpType == "transfer" {
			return errors.New("转账和平账不使用支付渠道")
		}
		var payment model.Account
		if err := s.db.First(&payment, "id = ? AND ledger_id = ?", *t.PaymentAccountID, ledgerID).Error; err != nil || (payment.Type != "alipay" && payment.Type != "wechat") {
			return errors.New("支付渠道必须是本账本的支付宝或微信")
		}
		var linkCount int64
		s.db.Model(&model.AccountLink{}).Where(
			"ledger_id = ? AND payment_account_id = ? AND funding_account_id = ?",
			ledgerID, *t.PaymentAccountID, t.AccountID,
		).Count(&linkCount)
		if linkCount == 0 {
			return errors.New("所选储蓄卡或信用卡未关联到该支付渠道")
		}
	}

	if t.OpType != "transfer" && t.CategoryID == nil {
		return errors.New("非转账需要分类")
	}

	if t.CategoryID != nil {
		var cat model.Category
		if err := s.db.First(&cat, "id = ? AND ledger_id = ?", *t.CategoryID, ledgerID).Error; err != nil {
			return errors.New("分类不存在或不属于该账本")
		}
		if (t.OpType == "expense" && cat.Kind != "expense") || (t.OpType == "income" && cat.Kind != "income") {
			return errors.New("分类类型与收支类型不一致")
		}
	}

	return nil
}

func (s *TransactionService) applyBalance(tx *gorm.DB, t *model.Transaction, sign int) error {
	var source model.Account
	if err := tx.First(&source, "id = ? AND ledger_id = ?", t.AccountID, t.LedgerID).Error; err != nil {
		return errors.New("账户不存在")
	}

	// 普通账户 balance = 资产；信用卡 balance = 当前欠款。
	// 转账单独处理，避免「转入信用卡」被当成信用卡转出而增加欠款。
	if t.OpType == "transfer" && t.ToAccountID != nil {
		var target model.Account
		if err := tx.First(&target, "id = ? AND ledger_id = ?", *t.ToAccountID, t.LedgerID).Error; err != nil {
			return errors.New("转入账户不存在")
		}

		var sourceDelta, targetDelta decimal.Decimal
		if source.Type == "credit" {
			// 信用卡转出（取现）→ 欠款增加
			sourceDelta = t.Amount
		} else {
			// 普通账户转出 → 资产减少
			sourceDelta = t.Amount.Neg()
		}
		if target.Type == "credit" {
			// 转入信用卡 = 还款平账 → 欠款减少（绝不增加）
			targetDelta = t.Amount.Neg()
		} else {
			// 转入普通账户 → 资产增加
			targetDelta = t.Amount
		}
		if sign < 0 {
			sourceDelta = sourceDelta.Neg()
			targetDelta = targetDelta.Neg()
		}

		if err := tx.Model(&model.Account{}).
			Where("id = ? AND ledger_id = ?", t.AccountID, t.LedgerID).
			Update("balance", gorm.Expr("balance + ?", sourceDelta)).Error; err != nil {
			return fmt.Errorf("调整转出账户余额失败: %w", err)
		}
		if err := tx.Model(&model.Account{}).
			Where("id = ? AND ledger_id = ?", *t.ToAccountID, t.LedgerID).
			Update("balance", gorm.Expr("balance + ?", targetDelta)).Error; err != nil {
			return fmt.Errorf("调整转入账户余额失败: %w", err)
		}
		return nil
	}

	// 非转账：信用卡支出/借出 → 欠款+；收入/借入 → 欠款-
	delta := model.BalanceDelta(t.OpType, t.Amount)
	if source.Type == "credit" {
		switch t.OpType {
		case "expense", "borrow_out":
			delta = t.Amount
		case "income", "borrow_in":
			delta = t.Amount.Neg()
		}
	}
	if sign < 0 {
		delta = delta.Neg()
	}

	if err := tx.Model(&model.Account{}).
		Where("id = ? AND ledger_id = ?", t.AccountID, t.LedgerID).
		Update("balance", gorm.Expr("balance + ?", delta)).Error; err != nil {
		return fmt.Errorf("调整账户余额失败: %w", err)
	}
	return nil
}

func (s *TransactionService) assertPersonalLedger(ledgerID uint) error {
	var ledger model.Ledger
	if err := s.db.First(&ledger, ledgerID).Error; err != nil {
		return errors.New("账本不存在")
	}
	if !ledger.IsPersonal {
		return errSharedDirectCreate
	}
	return nil
}

func (s *TransactionService) isMember(ledgerID, userID uint) bool {
	var count int64
	s.db.Model(&model.Membership{}).Where("ledger_id = ? AND user_id = ?", ledgerID, userID).Count(&count)
	return count > 0
}

// Create 创建流水（仅个人账本）
func (s *TransactionService) Create(ledgerID, userID uint, t *model.Transaction) (*model.Transaction, error) {
	if err := s.assertPersonalLedger(ledgerID); err != nil {
		return nil, err
	}
	return s.createInternal(ledgerID, userID, t)
}

// createInternal 内部创建（推送同步用，跳过个人账本守卫）
func (s *TransactionService) createInternal(ledgerID, userID uint, t *model.Transaction) (*model.Transaction, error) {
	t.LedgerID = ledgerID
	t.UserID = userID

	if err := s.validate(ledgerID, t); err != nil {
		return nil, err
	}

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

// Update 更新流水（副本只读；更新后级联同步副本）
func (s *TransactionService) Update(ledgerID, userID, id uint, next *model.Transaction) (*model.Transaction, error) {
	var old model.Transaction
	if err := s.db.First(&old, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("流水不存在")
	}
	if old.SourceID != nil {
		return nil, errCopyReadOnly
	}
	if err := s.assertPersonalLedger(ledgerID); err != nil {
		return nil, err
	}

	next.LedgerID = ledgerID
	next.UserID = old.UserID
	if err := s.validate(ledgerID, next); err != nil {
		return nil, err
	}

	var result *model.Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.applyBalance(tx, &old, -1); err != nil {
			return err
		}

		next.ID = old.ID
		next.UserID = old.UserID
		next.CreatedAt = old.CreatedAt
		next.SourceID = old.SourceID
		if err := tx.Save(next).Error; err != nil {
			return fmt.Errorf("更新流水失败: %w", err)
		}

		if err := s.applyBalance(tx, next, 1); err != nil {
			return err
		}

		result = next
		return nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.syncCopies(result)
	return result, nil
}

// Delete 删除流水。删除同步副本只影响当前账本；删除源流水会级联删除所有副本。
func (s *TransactionService) Delete(ledgerID, userID, id uint) error {
	_, err := s.DeleteBatch(ledgerID, userID, []uint{id})
	return err
}

const maxBatchTransactions = 100

// DeleteBatch 在同一事务中批量删除流水。源流水的所有同步副本也在同一事务中删除，
// 在其他账本删除同步副本则只影响当前账本。
func (s *TransactionService) DeleteBatch(ledgerID, userID uint, ids []uint) (int, error) {
	transactionIDs, err := normalizeTransactionIDs(ids)
	if err != nil {
		return 0, err
	}
	if !s.isMember(ledgerID, userID) {
		return 0, errNotMember
	}

	var deleted int
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var ledger model.Ledger
		if err := tx.First(&ledger, ledgerID).Error; err != nil {
			return errors.New("账本不存在")
		}
		var transactions []model.Transaction
		if err := tx.Where("ledger_id = ? AND id IN ? AND deleted_at IS NULL", ledgerID, transactionIDs).
			Order("id ASC").Find(&transactions).Error; err != nil {
			return fmt.Errorf("查询流水失败: %w", err)
		}
		if len(transactions) != len(transactionIDs) {
			return errors.New("部分流水不存在或不属于当前账本，请刷新后重试")
		}
		for i := range transactions {
			item := &transactions[i]
			if item.SourceID == nil && !ledger.IsPersonal {
				return errSharedDirectCreate
			}
			if err := s.applyBalance(tx, item, -1); err != nil {
				return err
			}
			if item.SourceID == nil {
				if err := s.deleteCopiesInTx(tx, item.ID); err != nil {
					return err
				}
			}
			if err := tx.Delete(item).Error; err != nil {
				return fmt.Errorf("删除流水失败: %w", err)
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func normalizeTransactionIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, errors.New("请至少选择一笔流水")
	}
	if len(ids) > maxBatchTransactions {
		return nil, fmt.Errorf("单次最多操作 %d 笔流水", maxBatchTransactions)
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, errors.New("流水 ID 无效")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

// ListRequest 流水列表请求参数
type ListRequest struct {
	// start/end 由 handler 使用 parseTime 手动解析，不能交给 Gin 自动绑定。
	// 否则 YYYY-MM-DD 会被按 RFC3339 解析并在进入 handler 的日期逻辑前直接报错。
	Start      *time.Time `form:"-"`
	End        *time.Time `form:"-"`
	Month      string     `form:"month"` // YYYY-MM，与 ListMonths 同口径，避免时区导致明细为空
	AccountID  *uint      `form:"account_id"`
	CategoryID *uint      `form:"category_id"`
	MemberID   *uint      `form:"member_id"`
	OpType     string     `form:"op_type"`
	Keyword    string     `form:"keyword"`
	Page       int        `form:"page" binding:"omitempty,min=1"`
	PageSize   int        `form:"page_size" binding:"omitempty,min=1,max=100"`
}

func (s *TransactionService) monthExpr() string {
	if config.Get().DB.Driver == "mysql" {
		return "DATE_FORMAT(occurred_at, '%Y-%m')"
	}
	return "strftime('%Y-%m', occurred_at)"
}

// List 获取流水列表
func (s *TransactionService) List(ledgerID uint, req *ListRequest) ([]model.Transaction, int64, error) {
	query := s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID)

	if req.Month != "" {
		if _, err := time.ParseInLocation("2006-01", req.Month, time.Local); err != nil {
			return nil, 0, errors.New("月份格式错误，请使用 YYYY-MM")
		}
		query = query.Where(s.monthExpr()+" = ?", req.Month)
	} else {
		if req.Start != nil {
			query = query.Where("occurred_at >= ?", req.Start)
		}
		if req.End != nil {
			query = query.Where("occurred_at <= ?", req.End)
		}
	}
	if req.AccountID != nil {
		// 账户明细：支出/收入账户、转账转入、支付渠道关联均计入
		id := *req.AccountID
		query = query.Where("account_id = ? OR to_account_id = ? OR payment_account_id = ?", id, id, id)
	}
	if req.CategoryID != nil {
		query = query.Where("category_id = ?", *req.CategoryID)
	}
	if req.MemberID != nil {
		query = query.Where("user_id = ?", *req.MemberID)
	}
	if req.OpType != "" {
		query = query.Where("op_type = ?", req.OpType)
	}
	if req.Keyword != "" {
		query = query.Where("remark LIKE ?", "%"+req.Keyword+"%")
	}

	var total int64
	query.Model(&model.Transaction{}).Count(&total)

	offset := (req.Page - 1) * req.PageSize
	var transactions []model.Transaction
	err := query.Preload("Account").Preload("PaymentAccount").Preload("ToAccount").Preload("Category").Preload("Category.Parent").Preload("Member").
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
	err := s.db.Preload("Account").Preload("PaymentAccount").Preload("ToAccount").Preload("Category").Preload("Category.Parent").Preload("Member").
		First(&t, "id = ? AND ledger_id = ?", id, ledgerID).Error
	if err != nil {
		return nil, errors.New("流水不存在")
	}
	return &t, nil
}

// TransactionMonth 月份折叠摘要；转账/还款不计入收入和支出。
type TransactionMonth struct {
	Month   string          `json:"month"`
	Count   int64           `json:"count"`
	Income  decimal.Decimal `json:"income"`
	Expense decimal.Decimal `json:"expense"`
}

// ListMonths 返回有流水的全部月份，供前端按月懒加载。
func (s *TransactionService) ListMonths(ledgerID uint) ([]TransactionMonth, error) {
	period := s.monthExpr()
	result := []TransactionMonth{}
	err := s.db.Model(&model.Transaction{}).
		Select(period+` AS month,
			COUNT(*) AS count,
			COALESCE(SUM(CASE WHEN op_type IN ('income','borrow_in') THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN op_type IN ('expense','borrow_out') THEN amount ELSE 0 END), 0) AS expense`).
		Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Group("month").Order("month DESC").Scan(&result).Error
	if err != nil {
		return nil, fmt.Errorf("查询流水月份失败: %w", err)
	}
	return result, nil
}

// mapAccountToLedger 优先按名称和类型映射账户；同名不存在时退回同类型账户，
// 避免信用卡等资金账户被错误映射成排序靠前的支付宝或微信账户。
func (s *TransactionService) mapAccountToLedger(srcAccountID, targetLedgerID uint) (uint, error) {
	var src model.Account
	if err := s.db.First(&src, srcAccountID).Error; err != nil {
		return 0, errors.New("源账户不存在")
	}

	var target model.Account
	err := s.db.Where("ledger_id = ? AND name = ? AND type = ? AND is_archived = ? AND deleted_at IS NULL",
		targetLedgerID, src.Name, src.Type, false).
		Order("sort asc, id asc").First(&target).Error
	if err == nil {
		return target.ID, nil
	}

	err = s.db.Where("ledger_id = ? AND type = ? AND is_archived = ? AND deleted_at IS NULL",
		targetLedgerID, src.Type, false).
		Order("sort asc, id asc").First(&target).Error
	if err == nil {
		return target.ID, nil
	}

	err = s.db.Where("ledger_id = ? AND is_archived = ? AND deleted_at IS NULL", targetLedgerID, false).
		Order("sort asc, id asc").First(&target).Error
	if err != nil {
		return 0, errors.New("目标账本没有可用账户")
	}
	return target.ID, nil
}

// mapPaymentAccountToLedger 将支付宝/微信支付渠道独立映射到目标账本。
// 支付渠道不能使用普通账户兜底，否则展示出来的支付方式会与源流水不一致。
func (s *TransactionService) mapPaymentAccountToLedger(srcPaymentAccountID *uint, targetLedgerID uint) (*uint, error) {
	if srcPaymentAccountID == nil {
		return nil, nil
	}

	var src model.Account
	if err := s.db.First(&src, *srcPaymentAccountID).Error; err != nil {
		return nil, errors.New("源支付渠道不存在")
	}
	if src.Type != "alipay" && src.Type != "wechat" {
		return nil, errors.New("源支付渠道类型无效")
	}

	var target model.Account
	err := s.db.Where("ledger_id = ? AND name = ? AND type = ? AND is_archived = ? AND deleted_at IS NULL",
		targetLedgerID, src.Name, src.Type, false).
		Order("sort asc, id asc").First(&target).Error
	if err != nil {
		err = s.db.Where("ledger_id = ? AND type = ? AND is_archived = ? AND deleted_at IS NULL",
			targetLedgerID, src.Type, false).
			Order("sort asc, id asc").First(&target).Error
	}
	if err != nil {
		return nil, fmt.Errorf("目标账本没有可用的%s支付渠道", src.Name)
	}
	targetID := target.ID
	return &targetID, nil
}

// ensurePaymentRoute 确保目标账本存在“支付渠道 → 资金账户”的关联，
// 使同步副本能够通过与普通流水一致的数据校验。
func (s *TransactionService) ensurePaymentRoute(ledgerID, fundingAccountID uint, paymentAccountID *uint) error {
	if paymentAccountID == nil {
		return nil
	}
	var funding model.Account
	if err := s.db.First(&funding, "id = ? AND ledger_id = ?", fundingAccountID, ledgerID).Error; err != nil {
		return errors.New("目标资金账户不存在")
	}
	if funding.Type != "savings" && funding.Type != "credit" {
		return errors.New("支付渠道只能关联目标账本的储蓄卡或信用卡")
	}
	link := model.AccountLink{
		LedgerID: ledgerID, PaymentAccountID: *paymentAccountID, FundingAccountID: fundingAccountID,
	}
	if err := s.db.Where(
		"ledger_id = ? AND payment_account_id = ? AND funding_account_id = ?",
		ledgerID, *paymentAccountID, fundingAccountID,
	).FirstOrCreate(&link).Error; err != nil {
		return fmt.Errorf("同步支付渠道关联失败: %w", err)
	}
	return nil
}

func (s *TransactionService) mapTransactionAccountsToLedger(src *model.Transaction, targetLedgerID uint) (uint, *uint, error) {
	accountID, err := s.mapAccountToLedger(src.AccountID, targetLedgerID)
	if err != nil {
		return 0, nil, err
	}
	paymentAccountID, err := s.mapPaymentAccountToLedger(src.PaymentAccountID, targetLedgerID)
	if err != nil {
		return 0, nil, err
	}
	if err := s.ensurePaymentRoute(targetLedgerID, accountID, paymentAccountID); err != nil {
		return 0, nil, err
	}
	return accountID, paymentAccountID, nil
}

// mapCategoryToLedger 按名称映射分类；找不到则落到同 kind 的「其他」或第一个一级分类
func (s *TransactionService) mapCategoryToLedger(srcCategoryID *uint, targetLedgerID uint, opType string) (*uint, error) {
	if srcCategoryID == nil {
		return nil, nil
	}

	var src model.Category
	if err := s.db.First(&src, *srcCategoryID).Error; err != nil {
		return nil, errors.New("源分类不存在")
	}

	kind := src.Kind
	if kind == "" {
		if opType == "income" || opType == "borrow_in" {
			kind = "income"
		} else {
			kind = "expense"
		}
	}

	var target model.Category
	err := s.db.Where("ledger_id = ? AND name = ? AND kind = ? AND deleted_at IS NULL", targetLedgerID, src.Name, kind).
		Order("id asc").First(&target).Error
	if err == nil {
		return &target.ID, nil
	}

	// 尝试「其他」
	err = s.db.Where("ledger_id = ? AND name = ? AND kind = ? AND parent_id IS NULL AND deleted_at IS NULL",
		targetLedgerID, "其他", kind).First(&target).Error
	if err == nil {
		return &target.ID, nil
	}

	// 回落：同 kind 第一个一级分类
	err = s.db.Where("ledger_id = ? AND kind = ? AND parent_id IS NULL AND deleted_at IS NULL", targetLedgerID, kind).
		Order("sort asc, id asc").First(&target).Error
	if err != nil {
		return nil, errors.New("目标账本没有可用分类")
	}
	return &target.ID, nil
}

// PushToLedger 将个人账本流水同步到其他类型账本
func (s *TransactionService) PushToLedger(srcLedgerID, srcID, targetLedgerID, userID uint) (*model.Transaction, error) {
	if !s.isMember(srcLedgerID, userID) || !s.isMember(targetLedgerID, userID) {
		return nil, errNotMember
	}

	var srcLedger, targetLedger model.Ledger
	if err := s.db.First(&srcLedger, srcLedgerID).Error; err != nil {
		return nil, errors.New("源账本不存在")
	}
	if !srcLedger.IsPersonal {
		return nil, errNotPersonalSource
	}
	if err := s.db.First(&targetLedger, targetLedgerID).Error; err != nil {
		return nil, errors.New("目标账本不存在")
	}
	if targetLedger.IsPersonal {
		return nil, errTargetMustShared
	}

	var src model.Transaction
	if err := s.db.First(&src, "id = ? AND ledger_id = ?", srcID, srcLedgerID).Error; err != nil {
		return nil, errors.New("流水不存在")
	}
	if src.OpType != "expense" && src.OpType != "income" {
		return nil, errTransferNoPush
	}
	if src.SourceID != nil {
		return nil, errors.New("不能推送同步副本")
	}

	var dup int64
	s.db.Model(&model.Transaction{}).
		Where("ledger_id = ? AND source_id = ?", targetLedgerID, src.ID).
		Count(&dup)
	if dup > 0 {
		return nil, errAlreadyPushed
	}

	accountID, paymentAccountID, err := s.mapTransactionAccountsToLedger(&src, targetLedgerID)
	if err != nil {
		return nil, err
	}
	categoryID, err := s.mapCategoryToLedger(src.CategoryID, targetLedgerID, src.OpType)
	if err != nil {
		return nil, err
	}

	srcIDCopy := src.ID
	copyTx := &model.Transaction{
		AccountID:        accountID,
		PaymentAccountID: paymentAccountID,
		CategoryID:       categoryID,
		OpType:           src.OpType,
		Amount:           src.Amount,
		Remark:           src.Remark,
		OccurredAt:       src.OccurredAt,
		SourceID:         &srcIDCopy,
	}

	return s.createInternal(targetLedgerID, userID, copyTx)
}

// PushMonthResult 整月推送结果
type PushMonthResult struct {
	Pushed  int `json:"pushed"`
	Skipped int `json:"skipped"`
}

// PushBatchResult 批量同步结果；已同步或不支持同步的流水计入 skipped。
type PushBatchResult struct {
	Pushed  int `json:"pushed"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// PushBatch 将选中的个人账本流水逐笔同步到目标账本，保持单笔同步的幂等语义。
func (s *TransactionService) PushBatch(srcLedgerID uint, ids []uint, targetLedgerID, userID uint) (*PushBatchResult, error) {
	transactionIDs, err := normalizeTransactionIDs(ids)
	if err != nil {
		return nil, err
	}
	if !s.isMember(srcLedgerID, userID) || !s.isMember(targetLedgerID, userID) {
		return nil, errNotMember
	}
	var srcLedger, targetLedger model.Ledger
	if err := s.db.First(&srcLedger, srcLedgerID).Error; err != nil || !srcLedger.IsPersonal {
		return nil, errNotPersonalSource
	}
	if err := s.db.First(&targetLedger, targetLedgerID).Error; err != nil || targetLedger.IsPersonal {
		return nil, errTargetMustShared
	}

	result := &PushBatchResult{}
	for _, id := range transactionIDs {
		_, pushErr := s.PushToLedger(srcLedgerID, id, targetLedgerID, userID)
		switch {
		case pushErr == nil:
			result.Pushed++
		case errors.Is(pushErr, errAlreadyPushed), errors.Is(pushErr, errTransferNoPush):
			result.Skipped++
		default:
			result.Failed++
		}
	}
	return result, nil
}

// PushMonth 整月同步个人账本 income/expense 到其他类型账本
func (s *TransactionService) PushMonth(srcLedgerID uint, month string, targetLedgerID, userID uint) (*PushMonthResult, error) {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return nil, errors.New("月份格式错误，请使用 YYYY-MM")
	}
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	var list []model.Transaction
	err = s.db.Where(
		"ledger_id = ? AND occurred_at >= ? AND occurred_at <= ? AND op_type IN ? AND source_id IS NULL AND deleted_at IS NULL",
		srcLedgerID, start, end, []string{"expense", "income"},
	).Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("查询当月流水失败: %w", err)
	}

	result := &PushMonthResult{}
	for _, tx := range list {
		_, err := s.PushToLedger(srcLedgerID, tx.ID, targetLedgerID, userID)
		if err != nil {
			if errors.Is(err, errAlreadyPushed) {
				result.Skipped++
				continue
			}
			result.Skipped++
			continue
		}
		result.Pushed++
	}
	return result, nil
}

// syncCopies 源流水变更后同步所有副本
func (s *TransactionService) syncCopies(src *model.Transaction) error {
	var copies []model.Transaction
	if err := s.db.Where("source_id = ?", src.ID).Find(&copies).Error; err != nil {
		return err
	}

	for i := range copies {
		copy := copies[i]
		accountID, paymentAccountID, err := s.mapTransactionAccountsToLedger(src, copy.LedgerID)
		if err != nil {
			continue
		}
		categoryID, err := s.mapCategoryToLedger(src.CategoryID, copy.LedgerID, src.OpType)
		if err != nil {
			continue
		}

		_ = s.db.Transaction(func(tx *gorm.DB) error {
			if err := s.applyBalance(tx, &copy, -1); err != nil {
				return err
			}
			copy.AccountID = accountID
			copy.CategoryID = categoryID
			copy.OpType = src.OpType
			copy.Amount = src.Amount
			copy.Remark = src.Remark
			copy.OccurredAt = src.OccurredAt
			copy.ToAccountID = nil
			copy.PaymentAccountID = paymentAccountID
			if err := tx.Save(&copy).Error; err != nil {
				return err
			}
			return s.applyBalance(tx, &copy, 1)
		})
	}
	return nil
}

// RepairSyncedPaymentRoutes 修复旧版本同步副本中丢失的支付渠道，
// 同时把曾经回退到错误类型的资金账户重新映射，并保持目标账户余额正确。
// 该操作幂等，可在服务启动时安全执行。
func (s *TransactionService) RepairSyncedPaymentRoutes() (int, error) {
	var copies []model.Transaction
	if err := s.db.Where("source_id IS NOT NULL").Find(&copies).Error; err != nil {
		return 0, fmt.Errorf("查询同步副本失败: %w", err)
	}

	repaired := 0
	var firstErr error
	for i := range copies {
		copyTx := copies[i]
		if copyTx.SourceID == nil {
			continue
		}
		var source model.Transaction
		if err := s.db.First(&source, *copyTx.SourceID).Error; err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("读取同步副本 %d 的源流水失败: %w", copyTx.ID, err)
			}
			continue
		}

		accountID, paymentAccountID, err := s.mapTransactionAccountsToLedger(&source, copyTx.LedgerID)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("映射同步副本 %d 的支付方式失败: %w", copyTx.ID, err)
			}
			continue
		}
		paymentUnchanged := (copyTx.PaymentAccountID == nil && paymentAccountID == nil) ||
			(copyTx.PaymentAccountID != nil && paymentAccountID != nil && *copyTx.PaymentAccountID == *paymentAccountID)
		if copyTx.AccountID == accountID && paymentUnchanged {
			continue
		}

		err = s.db.Transaction(func(tx *gorm.DB) error {
			if err := s.applyBalance(tx, &copyTx, -1); err != nil {
				return err
			}
			copyTx.AccountID = accountID
			copyTx.PaymentAccountID = paymentAccountID
			if err := tx.Model(&copyTx).Updates(map[string]interface{}{
				"account_id":         accountID,
				"payment_account_id": paymentAccountID,
			}).Error; err != nil {
				return fmt.Errorf("更新同步副本支付方式失败: %w", err)
			}
			return s.applyBalance(tx, &copyTx, 1)
		})
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("修复同步副本 %d 失败: %w", copyTx.ID, err)
			}
			continue
		}
		repaired++
	}
	return repaired, firstErr
}

// deleteCopiesInTx 源流水删除后在同一事务中级联删除并冲回所有副本余额。
func (s *TransactionService) deleteCopiesInTx(tx *gorm.DB, srcID uint) error {
	var copies []model.Transaction
	if err := tx.Where("source_id = ? AND deleted_at IS NULL", srcID).Find(&copies).Error; err != nil {
		return err
	}

	for i := range copies {
		copy := &copies[i]
		if err := s.applyBalance(tx, copy, -1); err != nil {
			return err
		}
		if err := tx.Delete(copy).Error; err != nil {
			return fmt.Errorf("删除同步流水失败: %w", err)
		}
	}
	return nil
}
