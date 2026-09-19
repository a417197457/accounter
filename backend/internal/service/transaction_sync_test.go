package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestLedgerTypesAndSyncedCopyDeletion(t *testing.T) {
	_ = config.Load()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:transaction_sync_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	user := model.User{Username: "t_sync", PasswordHash: "x", Nickname: "同步测试"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, user.ID, "个人", model.LedgerTypePersonal)

	ledgerSvc := &LedgerService{db: db}
	travel, _, err := ledgerSvc.Create(user.ID, "云南之旅", model.LedgerTypeTravel)
	if err != nil {
		t.Fatalf("create travel ledger: %v", err)
	}
	gift, _, err := ledgerSvc.Create(user.ID, "婚礼礼账", model.LedgerTypeGift)
	if err != nil {
		t.Fatalf("create gift ledger: %v", err)
	}
	if travel.Type != model.LedgerTypeTravel || gift.Type != model.LedgerTypeGift {
		t.Fatalf("unexpected ledger types: travel=%s gift=%s", travel.Type, gift.Type)
	}

	sourceAccount, sourceCategory := createSyncTestData(t, db, personal.ID, user.ID)
	txSvc := &TransactionService{db: db}
	source, err := txSvc.Create(personal.ID, user.ID, &model.Transaction{
		AccountID:  sourceAccount.ID,
		CategoryID: &sourceCategory.ID,
		OpType:     "expense",
		Amount:     decimal.NewFromInt(88),
		Remark:     "测试同步删除规则",
		OccurredAt: time.Date(2026, 8, 22, 12, 0, 0, 0, time.Local),
	})
	if err != nil {
		t.Fatalf("create source transaction: %v", err)
	}

	travelCopy, err := txSvc.PushToLedger(personal.ID, source.ID, travel.ID, user.ID)
	if err != nil {
		t.Fatalf("push travel copy: %v", err)
	}
	giftCopy, err := txSvc.PushToLedger(personal.ID, source.ID, gift.ID, user.ID)
	if err != nil {
		t.Fatalf("push gift copy: %v", err)
	}

	// 在目标账本删除副本，不影响个人账本原流水及其他账本副本。
	if err := txSvc.Delete(travel.ID, user.ID, travelCopy.ID); err != nil {
		t.Fatalf("delete target copy: %v", err)
	}
	if err := db.First(&model.Transaction{}, source.ID).Error; err != nil {
		t.Fatalf("source should remain: %v", err)
	}
	if err := db.First(&model.Transaction{}, giftCopy.ID).Error; err != nil {
		t.Fatalf("other copy should remain: %v", err)
	}
	if err := db.First(&model.Transaction{}, travelCopy.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("travel copy should be deleted, got: %v", err)
	}

	// 删除个人账本原流水，仍存在的其他账本副本会级联删除。
	if err := txSvc.Delete(personal.ID, user.ID, source.ID); err != nil {
		t.Fatalf("delete source: %v", err)
	}
	if err := db.First(&model.Transaction{}, giftCopy.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("gift copy should cascade delete, got: %v", err)
	}
}

func TestBatchPushAndDeleteTransactions(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:transaction_batch_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "t_batch", PasswordHash: "x", Nickname: "批量测试"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, user.ID, "个人", model.LedgerTypePersonal)
	travel := createSyncTestLedger(t, db, user.ID, "旅行", model.LedgerTypeTravel)
	gift := createSyncTestLedger(t, db, user.ID, "礼账", model.LedgerTypeGift)
	personalAccount, personalCategory := createSyncTestData(t, db, personal.ID, user.ID)
	createSyncTestData(t, db, travel.ID, user.ID)
	createSyncTestData(t, db, gift.ID, user.ID)

	txSvc := &TransactionService{db: db}
	sources := make([]*model.Transaction, 0, 2)
	for i, amount := range []int64{30, 50} {
		item, createErr := txSvc.Create(personal.ID, user.ID, &model.Transaction{
			AccountID: personalAccount.ID, CategoryID: &personalCategory.ID, OpType: "expense",
			Amount: decimal.NewFromInt(amount), Remark: fmt.Sprintf("批量%d", i), OccurredAt: time.Now(),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		sources = append(sources, item)
	}
	if _, err := txSvc.PushToLedger(personal.ID, sources[0].ID, gift.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	result, err := txSvc.PushBatch(personal.ID, []uint{sources[0].ID, sources[1].ID}, gift.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pushed != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("unexpected push result: %+v", result)
	}

	if _, err := txSvc.DeleteBatch(personal.ID, user.ID, []uint{sources[0].ID, 999999}); err == nil {
		t.Fatal("batch containing a missing transaction must fail")
	}
	if err := db.First(&model.Transaction{}, sources[0].ID).Error; err != nil {
		t.Fatalf("failed batch must roll back: %v", err)
	}
	deleted, err := txSvc.DeleteBatch(personal.ID, user.ID, []uint{sources[0].ID, sources[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%d, want 2", deleted)
	}
	var remainingCopies int64
	if err := db.Model(&model.Transaction{}).Where("source_id IN ?", []uint{sources[0].ID, sources[1].ID}).Count(&remainingCopies).Error; err != nil {
		t.Fatal(err)
	}
	if remainingCopies != 0 {
		t.Fatalf("source copies should be deleted, remaining=%d", remainingCopies)
	}
}

func TestPushAndSourceUpdatePreservePaymentRoute(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:transaction_payment_sync_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "t_payment_sync", PasswordHash: "x", Nickname: "支付方式同步测试"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, user.ID, "个人", model.LedgerTypePersonal)
	travel := createSyncTestLedger(t, db, user.ID, "旅行", model.LedgerTypeTravel)

	sourcePayment := model.Account{LedgerID: personal.ID, UserID: user.ID, Name: "微信", Type: "wechat"}
	sourceFunding := model.Account{LedgerID: personal.ID, UserID: user.ID, Name: "中信信用卡", Type: "credit"}
	targetCash := model.Account{LedgerID: travel.ID, UserID: user.ID, Name: "现金", Type: "cash", Sort: 1}
	targetPayment := model.Account{LedgerID: travel.ID, UserID: user.ID, Name: "微信", Type: "wechat", Sort: 2}
	targetFunding := model.Account{LedgerID: travel.ID, UserID: user.ID, Name: "信用卡", Type: "credit", Sort: 3}
	for _, account := range []*model.Account{&sourcePayment, &sourceFunding, &targetCash, &targetPayment, &targetFunding} {
		if err := db.Create(account).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.AccountLink{
		LedgerID: personal.ID, PaymentAccountID: sourcePayment.ID, FundingAccountID: sourceFunding.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	sourceCategory := model.Category{LedgerID: personal.ID, UserID: user.ID, Name: "餐饮", Kind: "expense"}
	targetCategory := model.Category{LedgerID: travel.ID, UserID: user.ID, Name: "餐饮", Kind: "expense"}
	if err := db.Create(&sourceCategory).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&targetCategory).Error; err != nil {
		t.Fatal(err)
	}

	txSvc := &TransactionService{db: db}
	sourcePaymentID := sourcePayment.ID
	source, err := txSvc.Create(personal.ID, user.ID, &model.Transaction{
		AccountID: sourceFunding.ID, PaymentAccountID: &sourcePaymentID, CategoryID: &sourceCategory.ID,
		OpType: "expense", Amount: decimal.NewFromInt(70), Remark: "微信绑定信用卡支付", OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	copyTx, err := txSvc.PushToLedger(personal.ID, source.ID, travel.ID, user.ID)
	if err != nil {
		t.Fatalf("push transaction: %v", err)
	}
	if copyTx.AccountID != targetFunding.ID {
		t.Fatalf("funding account=%d, want target credit account %d", copyTx.AccountID, targetFunding.ID)
	}
	if copyTx.PaymentAccountID == nil || *copyTx.PaymentAccountID != targetPayment.ID {
		t.Fatalf("payment account=%v, want target wechat account %d", copyTx.PaymentAccountID, targetPayment.ID)
	}
	var linkCount int64
	if err := db.Model(&model.AccountLink{}).Where(
		"ledger_id = ? AND payment_account_id = ? AND funding_account_id = ?",
		travel.ID, targetPayment.ID, targetFunding.ID,
	).Count(&linkCount).Error; err != nil {
		t.Fatal(err)
	}
	if linkCount != 1 {
		t.Fatalf("target payment route should be created, count=%d", linkCount)
	}

	// 模拟旧版本已经生成的错误副本：支付渠道丢失，信用卡误落到排序第一的现金账户。
	if err := db.Model(&model.Transaction{}).Where("id = ?", copyTx.ID).Updates(map[string]interface{}{
		"account_id": targetCash.ID, "payment_account_id": nil,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Account{}).Where("id = ?", targetFunding.ID).Update("balance", decimal.Zero).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Account{}).Where("id = ?", targetCash.ID).Update("balance", decimal.NewFromInt(-70)).Error; err != nil {
		t.Fatal(err)
	}
	repaired, err := txSvc.RepairSyncedPaymentRoutes()
	if err != nil {
		t.Fatalf("repair legacy copy: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("repaired=%d, want 1", repaired)
	}
	var repairedCopy model.Transaction
	if err := db.First(&repairedCopy, copyTx.ID).Error; err != nil {
		t.Fatal(err)
	}
	if repairedCopy.AccountID != targetFunding.ID || repairedCopy.PaymentAccountID == nil || *repairedCopy.PaymentAccountID != targetPayment.ID {
		t.Fatalf("legacy copy was not repaired: account=%d payment=%v", repairedCopy.AccountID, repairedCopy.PaymentAccountID)
	}
	var repairedCash, repairedCredit model.Account
	if err := db.First(&repairedCash, targetCash.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&repairedCredit, targetFunding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !repairedCash.Balance.IsZero() || !repairedCredit.Balance.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("repair balances cash=%s credit=%s", repairedCash.Balance, repairedCredit.Balance)
	}

	// 修改源流水后，副本仍应保留目标账本中的支付渠道映射。
	updated, err := txSvc.Update(personal.ID, user.ID, source.ID, &model.Transaction{
		AccountID: sourceFunding.ID, PaymentAccountID: &sourcePaymentID, CategoryID: &sourceCategory.ID,
		OpType: "expense", Amount: decimal.NewFromInt(88), Remark: "修改后仍用微信支付", OccurredAt: source.OccurredAt,
	})
	if err != nil {
		t.Fatalf("update source transaction: %v", err)
	}
	if updated.PaymentAccountID == nil {
		t.Fatal("source payment account unexpectedly cleared")
	}
	var refreshedCopy model.Transaction
	if err := db.First(&refreshedCopy, copyTx.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshedCopy.AccountID != targetFunding.ID || refreshedCopy.PaymentAccountID == nil || *refreshedCopy.PaymentAccountID != targetPayment.ID {
		t.Fatalf("updated copy lost payment route: account=%d payment=%v", refreshedCopy.AccountID, refreshedCopy.PaymentAccountID)
	}
}

func TestTransferCanSyncAndKeepsBothAccountBalances(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:transaction_transfer_sync_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "t_transfer_sync", PasswordHash: "x", Nickname: "转账同步测试"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, user.ID, "个人", model.LedgerTypePersonal)
	family := createSyncTestLedger(t, db, user.ID, "家庭", model.LedgerTypeFamily)

	sourceSavings := model.Account{LedgerID: personal.ID, UserID: user.ID, Name: "招商储蓄卡", Type: "savings", Balance: decimal.NewFromInt(2000)}
	sourceCredit := model.Account{LedgerID: personal.ID, UserID: user.ID, Name: "中信信用卡", Type: "credit", Balance: decimal.NewFromInt(500)}
	targetSavings := model.Account{LedgerID: family.ID, UserID: user.ID, Name: "招商储蓄卡", Type: "savings", Balance: decimal.NewFromInt(1000)}
	targetCredit := model.Account{LedgerID: family.ID, UserID: user.ID, Name: "中信信用卡", Type: "credit", Balance: decimal.NewFromInt(300)}
	for _, account := range []*model.Account{&sourceSavings, &sourceCredit, &targetSavings, &targetCredit} {
		if err := db.Create(account).Error; err != nil {
			t.Fatal(err)
		}
	}

	txSvc := &TransactionService{db: db}
	occurredAt := time.Date(2026, 9, 19, 20, 30, 0, 0, time.Local)
	sourceCreditID := sourceCredit.ID
	source, err := txSvc.Create(personal.ID, user.ID, &model.Transaction{
		AccountID: sourceSavings.ID, ToAccountID: &sourceCreditID, OpType: "transfer",
		Amount: decimal.NewFromInt(100), Remark: "信用卡还款", OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	copyTx, err := txSvc.PushToLedger(personal.ID, source.ID, family.ID, user.ID)
	if err != nil {
		t.Fatalf("push transfer: %v", err)
	}
	if copyTx.OpType != "transfer" || copyTx.AccountID != targetSavings.ID || copyTx.ToAccountID == nil || *copyTx.ToAccountID != targetCredit.ID {
		t.Fatalf("unexpected transfer copy: op=%s from=%d to=%v", copyTx.OpType, copyTx.AccountID, copyTx.ToAccountID)
	}
	assertAccountBalance(t, db, targetSavings.ID, "900")
	assertAccountBalance(t, db, targetCredit.ID, "200")

	updated, err := txSvc.Update(personal.ID, user.ID, source.ID, &model.Transaction{
		AccountID: sourceSavings.ID, ToAccountID: &sourceCreditID, OpType: "transfer",
		Amount: decimal.NewFromInt(150), Remark: "修改后的信用卡还款", OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("update source transfer: %v", err)
	}
	if !updated.Amount.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("updated amount=%s", updated.Amount)
	}
	var updatedCopy model.Transaction
	if err := db.First(&updatedCopy, copyTx.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedCopy.ToAccountID == nil || *updatedCopy.ToAccountID != targetCredit.ID || !updatedCopy.Amount.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("updated copy lost transfer route: to=%v amount=%s", updatedCopy.ToAccountID, updatedCopy.Amount)
	}
	assertAccountBalance(t, db, targetSavings.ID, "850")
	assertAccountBalance(t, db, targetCredit.ID, "150")

	if err := txSvc.Delete(personal.ID, user.ID, source.ID); err != nil {
		t.Fatalf("delete source transfer: %v", err)
	}
	assertAccountBalance(t, db, targetSavings.ID, "1000")
	assertAccountBalance(t, db, targetCredit.ID, "300")

	second, err := txSvc.Create(personal.ID, user.ID, &model.Transaction{
		AccountID: sourceSavings.ID, ToAccountID: &sourceCreditID, OpType: "transfer",
		Amount: decimal.NewFromInt(80), Remark: "整月同步还款", OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	monthResult, err := txSvc.PushMonth(personal.ID, "2026-09", family.ID, user.ID)
	if err != nil {
		t.Fatalf("push transfer month: %v", err)
	}
	if monthResult.Pushed != 1 || monthResult.Skipped != 0 {
		t.Fatalf("unexpected month result: %+v", monthResult)
	}
	var monthCopy model.Transaction
	if err := db.First(&monthCopy, "source_id = ? AND ledger_id = ?", second.ID, family.ID).Error; err != nil {
		t.Fatalf("month transfer copy missing: %v", err)
	}
	if monthCopy.ToAccountID == nil || *monthCopy.ToAccountID != targetCredit.ID {
		t.Fatalf("month transfer target=%v, want %d", monthCopy.ToAccountID, targetCredit.ID)
	}
}

func assertAccountBalance(t *testing.T, db *gorm.DB, accountID uint, expected string) {
	t.Helper()
	var account model.Account
	if err := db.First(&account, accountID).Error; err != nil {
		t.Fatal(err)
	}
	if !account.Balance.Equal(decimal.RequireFromString(expected)) {
		t.Fatalf("account %d balance=%s, want %s", accountID, account.Balance, expected)
	}
}

func TestClearSharedLedgerOwnerOnly(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:transaction_clear_shared_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "t_clear_owner", PasswordHash: "x", Nickname: "创建者"}
	member := model.User{Username: "t_clear_member", PasswordHash: "x", Nickname: "普通成员"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, owner.ID, "个人", model.LedgerTypePersonal)
	travel := createSyncTestLedger(t, db, owner.ID, "旅行", model.LedgerTypeTravel)
	if err := db.Create(&model.Membership{LedgerID: travel.ID, UserID: member.ID, Role: "member"}).Error; err != nil {
		t.Fatal(err)
	}
	sourceAccount, sourceCategory := createSyncTestData(t, db, personal.ID, owner.ID)
	targetAccount := model.Account{LedgerID: travel.ID, UserID: owner.ID, Name: "现金", Type: "cash", Balance: decimal.Zero}
	targetCategory := model.Category{LedgerID: travel.ID, UserID: owner.ID, Name: "餐饮", Kind: "expense"}
	if err := db.Create(&targetAccount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&targetCategory).Error; err != nil {
		t.Fatal(err)
	}

	txSvc := &TransactionService{db: db}
	sources := make([]*model.Transaction, 0, 2)
	for _, amount := range []int64{30, 50} {
		source, createErr := txSvc.Create(personal.ID, owner.ID, &model.Transaction{
			AccountID: sourceAccount.ID, CategoryID: &sourceCategory.ID, OpType: "expense",
			Amount: decimal.NewFromInt(amount), Remark: "待同步", OccurredAt: time.Now(),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		sources = append(sources, source)
		if _, pushErr := txSvc.PushToLedger(personal.ID, source.ID, travel.ID, owner.ID); pushErr != nil {
			t.Fatal(pushErr)
		}
	}

	if _, err := txSvc.ClearSharedLedger(travel.ID, member.ID); err == nil || err.Error() != "只有账本创建者可以一键清空流水" {
		t.Fatalf("member clear error=%v", err)
	}
	if _, err := txSvc.ClearSharedLedger(personal.ID, owner.ID); err == nil || err.Error() != "个人账本不能一键清空，请在明细中批量删除" {
		t.Fatalf("personal clear error=%v", err)
	}

	deleted, err := txSvc.ClearSharedLedger(travel.ID, owner.ID)
	if err != nil {
		t.Fatalf("owner clear shared ledger: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%d, want 2", deleted)
	}
	var targetCount int64
	if err := db.Model(&model.Transaction{}).Where("ledger_id = ?", travel.ID).Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("target transactions=%d, want 0", targetCount)
	}
	for _, source := range sources {
		if err := db.First(&model.Transaction{}, source.ID).Error; err != nil {
			t.Fatalf("source transaction %d should remain: %v", source.ID, err)
		}
	}
	var clearedAccount model.Account
	if err := db.First(&clearedAccount, targetAccount.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !clearedAccount.Balance.IsZero() {
		t.Fatalf("target balance=%s, want 0", clearedAccount.Balance)
	}
	if _, err := txSvc.PushToLedger(personal.ID, sources[0].ID, travel.ID, owner.ID); err != nil {
		t.Fatalf("source should be pushable again after clear: %v", err)
	}
}

func createSyncTestLedger(t *testing.T, db *gorm.DB, userID uint, name, ledgerType string) model.Ledger {
	t.Helper()
	ledger := model.Ledger{
		Name: name, OwnerID: userID, Type: ledgerType, IsPersonal: ledgerType == model.LedgerTypePersonal,
		InviteCode: model.GenInviteCode(),
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Membership{LedgerID: ledger.ID, UserID: userID, Role: "owner"}).Error; err != nil {
		t.Fatal(err)
	}
	return ledger
}

func createSyncTestData(t *testing.T, db *gorm.DB, ledgerID, userID uint) (model.Account, model.Category) {
	t.Helper()
	account := model.Account{
		LedgerID: ledgerID, UserID: userID, Name: "现金", Type: "cash", Balance: decimal.NewFromInt(1000),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	category := model.Category{
		LedgerID: ledgerID, UserID: userID, Name: "餐饮", Kind: "expense", Icon: "🍽️",
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	return account, category
}
