package service

import (
	"fmt"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
)

func setupPreferenceTest(t *testing.T) (*BookkeepingPreferenceService, model.User, model.Ledger, []model.Account) {
	t.Helper()
	_ = config.Load()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	user := model.User{Username: "preference_user", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, IsPersonal: true, InviteCode: "PREF01"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	accounts := []model.Account{
		{LedgerID: ledger.ID, UserID: user.ID, Name: "现金", Type: "cash", Balance: decimal.Zero, Sort: 1},
		{LedgerID: ledger.ID, UserID: user.ID, Name: "支付宝", Type: "alipay", Balance: decimal.Zero, Sort: 2},
		{LedgerID: ledger.ID, UserID: user.ID, Name: "招商信用卡", Type: "credit", Balance: decimal.Zero, Sort: 3},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.AccountLink{
		LedgerID: ledger.ID, PaymentAccountID: accounts[1].ID, FundingAccountID: accounts[2].ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return &BookkeepingPreferenceService{db: db}, user, ledger, accounts
}

func TestBookkeepingPreferenceDefaultsToAlipayBalance(t *testing.T) {
	svc, user, ledger, accounts := setupPreferenceTest(t)
	view, err := svc.Get(ledger.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.AccountMode != "default" || view.DefaultAccountID != accounts[1].ID || view.DefaultPaymentAccountID != nil {
		t.Fatalf("unexpected default preference: %+v", view)
	}
	if view.DefaultAccountLabel != "支付宝余额" {
		t.Fatalf("label want 支付宝余额 got %s", view.DefaultAccountLabel)
	}
}

func TestBookkeepingPreferenceAcceptsLinkedCardRoute(t *testing.T) {
	svc, user, ledger, accounts := setupPreferenceTest(t)
	paymentID := accounts[1].ID
	view, err := svc.Update(ledger.ID, user.ID, "default", accounts[2].ID, &paymentID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DefaultAccountLabel != "支付宝 · 招商信用卡" {
		t.Fatalf("unexpected linked route: %+v", view)
	}
}
