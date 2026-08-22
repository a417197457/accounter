package service

import (
	"fmt"
	"testing"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
)

func TestBudgetSpentUsesSelectedMonth(t *testing.T) {
	_ = config.Load()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "budget_user", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, IsPersonal: true, InviteCode: "BUDG01"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	account := model.Account{LedgerID: ledger.ID, UserID: user.ID, Name: "现金", Type: "cash", Balance: decimal.Zero}
	category := model.Category{LedgerID: ledger.ID, UserID: user.ID, Name: "餐饮", Kind: "expense"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	for _, tx := range []model.Transaction{
		{LedgerID: ledger.ID, UserID: user.ID, AccountID: account.ID, CategoryID: &category.ID, OpType: "expense", Amount: decimal.NewFromInt(30), OccurredAt: time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local)},
		{LedgerID: ledger.ID, UserID: user.ID, AccountID: account.ID, CategoryID: &category.ID, OpType: "expense", Amount: decimal.NewFromInt(80), OccurredAt: time.Date(2026, 8, 8, 12, 0, 0, 0, time.Local)},
	} {
		if err := db.Create(&tx).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &BudgetService{db: db}
	if err := svc.Set(ledger.ID, user.ID, &category.ID, decimal.NewFromInt(500)); err != nil {
		t.Fatal(err)
	}

	july, err := svc.ListBudgets(ledger.ID, time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local))
	if err != nil || len(july) != 1 || !july[0].Spent.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("July spent = %+v, err=%v", july, err)
	}
	august, err := svc.ListBudgets(ledger.ID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local))
	if err != nil || len(august) != 1 || !august[0].Spent.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("August spent = %+v, err=%v", august, err)
	}
}
