package service

import (
	"testing"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
)

func TestTransferToCreditReducesDebt(t *testing.T) {
	// NewDB 依赖 config.Get() 的方言分支；先 Load 默认 sqlite。
	_ = config.Load()

	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: "file:credit_repay_test?mode=memory&cache=shared"},
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	user := model.User{Username: "t_credit", PasswordHash: "x", Nickname: "t"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, IsPersonal: true, InviteCode: "TEST01"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	_ = db.Create(&model.Membership{LedgerID: ledger.ID, UserID: user.ID, Role: "owner"})

	savings := model.Account{
		LedgerID: ledger.ID, UserID: user.ID, Name: "储蓄卡", Type: "savings",
		Balance: decimal.NewFromInt(10000),
	}
	credit := model.Account{
		LedgerID: ledger.ID, UserID: user.ID, Name: "信用卡", Type: "credit",
		Balance: decimal.NewFromInt(800),
	}
	if err := db.Create(&savings).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&credit).Error; err != nil {
		t.Fatal(err)
	}

	svc := &TransactionService{db: db}
	to := credit.ID
	tx := &model.Transaction{
		AccountID:   savings.ID,
		ToAccountID: &to,
		OpType:      "transfer",
		Amount:      decimal.NewFromInt(300),
		OccurredAt:  time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local),
	}
	if _, err := svc.createInternal(ledger.ID, user.ID, tx); err != nil {
		t.Fatalf("transfer: %v", err)
	}

	var afterCredit, afterSavings model.Account
	_ = db.First(&afterCredit, credit.ID)
	_ = db.First(&afterSavings, savings.ID)

	if !afterCredit.Balance.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("credit debt want 500 got %s (还款应减少欠款)", afterCredit.Balance)
	}
	if !afterSavings.Balance.Equal(decimal.NewFromInt(9700)) {
		t.Fatalf("savings want 9700 got %s", afterSavings.Balance)
	}
}
