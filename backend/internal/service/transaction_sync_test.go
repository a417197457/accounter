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
