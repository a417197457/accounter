package service

import (
	"errors"
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
