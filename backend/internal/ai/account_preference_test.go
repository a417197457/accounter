package ai

import (
	"errors"
	"fmt"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
	"accounter/backend/internal/service"

	"github.com/shopspring/decimal"
)

func setupAIAccountPreferenceTest(t *testing.T) (*Service, model.Ledger, model.Account, model.Account) {
	t.Helper()
	_ = config.Load()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "ai_preference_user", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, IsPersonal: true, InviteCode: "AIPR01"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	alipay := model.Account{LedgerID: ledger.ID, UserID: user.ID, Name: "支付宝", Type: "alipay", Balance: decimal.Zero}
	cash := model.Account{LedgerID: ledger.ID, UserID: user.ID, Name: "现金", Type: "cash", Balance: decimal.Zero}
	if err := db.Create(&alipay).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&cash).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{db: db}, ledger, alipay, cash
}

func TestParseDraftUsesConfiguredDefaultWhenAccountNotSpecified(t *testing.T) {
	svc, ledger, alipay, _ := setupAIAccountPreferenceTest(t)
	pref := &service.BookkeepingPreferenceView{
		AccountMode: "default", DefaultAccountID: alipay.ID,
		DefaultAccountName: alipay.Name, DefaultAccountLabel: "支付宝余额",
	}
	draft, err := svc.parseDraft(ledger.ID,
		`{"op_type":"expense","amount":"25","account_name":"现金","account_specified":false,"category_name":"午餐"}`,
		pref,
	)
	if err != nil {
		t.Fatal(err)
	}
	if draft.AccountID != alipay.ID || draft.AccountName != "支付宝" || draft.AccountSpecified {
		t.Fatalf("default route was not enforced: %+v", draft)
	}
}

func TestParseDraftStrictModeRequiresExplicitAccount(t *testing.T) {
	svc, ledger, _, _ := setupAIAccountPreferenceTest(t)
	_, err := svc.parseDraft(ledger.ID,
		`{"op_type":"expense","amount":"25","account_name":"支付宝","account_specified":false}`,
		&service.BookkeepingPreferenceView{AccountMode: "strict"},
	)
	if !errors.Is(err, errAccountDetailRequired) {
		t.Fatalf("want account detail error, got %v", err)
	}
}

func TestParseDraftMatchesIncomeCategoryKind(t *testing.T) {
	svc, ledger, alipay, _ := setupAIAccountPreferenceTest(t)
	expenseGift := model.Category{LedgerID: ledger.ID, UserID: ledger.OwnerID, Name: "礼金", Kind: "expense", Icon: "🧧"}
	incomeGift := model.Category{LedgerID: ledger.ID, UserID: ledger.OwnerID, Name: "礼金", Kind: "income", Icon: "💌"}
	if err := svc.db.Create(&expenseGift).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&incomeGift).Error; err != nil {
		t.Fatal(err)
	}
	pref := &service.BookkeepingPreferenceView{
		AccountMode: "default", DefaultAccountID: alipay.ID,
		DefaultAccountName: alipay.Name, DefaultAccountLabel: "支付宝余额",
	}
	draft, err := svc.parseDraft(ledger.ID,
		`{"op_type":"income","amount":"1688","account_name":"支付宝","account_specified":false,"category_name":"礼金"}`,
		pref,
	)
	if err != nil {
		t.Fatal(err)
	}
	if draft.CategoryID == nil || *draft.CategoryID != incomeGift.ID {
		t.Fatalf("income gift category mismatch: %+v", draft)
	}
}
