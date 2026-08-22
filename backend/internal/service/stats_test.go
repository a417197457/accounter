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

func TestCategoryBreakdownGroupsByParentNameAndIcon(t *testing.T) {
	_ = config.Load()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "stats_user", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, IsPersonal: true, InviteCode: "STAT01"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	account := model.Account{LedgerID: ledger.ID, UserID: user.ID, Name: "现金", Type: "cash"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	parent := model.Category{LedgerID: ledger.ID, UserID: user.ID, Name: "餐饮", Kind: "expense", Icon: "餐"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	child := model.Category{LedgerID: ledger.ID, UserID: user.ID, Name: "午餐", Kind: "expense", Icon: "午", ParentID: &parent.ID}
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	for categoryID, amount := range map[uint]int64{parent.ID: 10, child.ID: 25} {
		tx := model.Transaction{
			LedgerID: ledger.ID, UserID: user.ID, AccountID: account.ID, CategoryID: &categoryID,
			OpType: "expense", Amount: decimal.NewFromInt(amount), OccurredAt: time.Now(),
		}
		if err := db.Create(&tx).Error; err != nil {
			t.Fatal(err)
		}
	}

	result, err := (&StatsService{db: db}).CategoryBreakdown(ledger.ID, "expense", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("父子分类应合并为一个统计项，实际 %+v", result)
	}
	if result[0].CategoryID != parent.ID || result[0].CategoryName != "餐饮" || result[0].Icon != "餐" {
		t.Fatalf("统计项应使用一级分类名称和图标，实际 %+v", result[0])
	}
	if !result[0].Amount.Equal(decimal.NewFromInt(35)) {
		t.Fatalf("合并金额应为 35，实际 %s", result[0].Amount)
	}
}
