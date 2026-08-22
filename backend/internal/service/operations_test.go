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

func TestOperationsUserSummaryAndBills(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	openID := "test-open-id"
	users := []model.User{
		{Username: "wx_internal", PasswordHash: "x", Nickname: "微信甲", WechatOpenID: &openID},
		{Username: "password_user", PasswordHash: "x", Nickname: "密码乙"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	personal := model.Ledger{Name: "个人", OwnerID: users[0].ID, Type: model.LedgerTypePersonal, InviteCode: "OPS001"}
	secondPersonal := model.Ledger{Name: "个人", OwnerID: users[1].ID, Type: model.LedgerTypePersonal, InviteCode: "OPS002"}
	shared := model.Ledger{Name: "旅游", OwnerID: users[0].ID, Type: model.LedgerTypeTravel, InviteCode: "OPS003"}
	for _, ledger := range []*model.Ledger{&personal, &secondPersonal, &shared} {
		if err := db.Create(ledger).Error; err != nil {
			t.Fatal(err)
		}
	}
	accounts := []model.Account{
		{LedgerID: personal.ID, UserID: users[0].ID, Name: "现金", Type: "cash"},
		{LedgerID: personal.ID, UserID: users[0].ID, Name: "储蓄卡", Type: "savings"},
		{LedgerID: shared.ID, UserID: users[0].ID, Name: "现金", Type: "cash"},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	category := model.Category{LedgerID: personal.ID, UserID: users[0].ID, Name: "餐饮", Kind: "expense"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	jan := time.Date(2026, 1, 12, 12, 0, 0, 0, time.Local)
	feb := time.Date(2026, 2, 8, 13, 0, 0, 0, time.Local)
	expense := model.Transaction{
		LedgerID: personal.ID, UserID: users[0].ID, AccountID: accounts[0].ID, CategoryID: &category.ID,
		OpType: "expense", Amount: decimal.NewFromInt(10), Remark: "午餐", OccurredAt: jan,
	}
	income := model.Transaction{
		LedgerID: personal.ID, UserID: users[0].ID, AccountID: accounts[1].ID,
		OpType: "income", Amount: decimal.NewFromInt(20), Remark: "奖金", OccurredAt: feb,
	}
	for _, transaction := range []*model.Transaction{&expense, &income} {
		if err := db.Create(transaction).Error; err != nil {
			t.Fatal(err)
		}
	}
	sourceID := expense.ID
	copyTransaction := model.Transaction{
		LedgerID: shared.ID, UserID: users[0].ID, AccountID: accounts[2].ID, OpType: "expense",
		Amount: decimal.NewFromInt(10), OccurredAt: jan, SourceID: &sourceID,
	}
	if err := db.Create(&copyTransaction).Error; err != nil {
		t.Fatal(err)
	}

	svc := &OperationsService{db: db}
	summaries, totalUsers, err := svc.ListUserSummaries(1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if totalUsers != 2 || len(summaries) != 2 {
		t.Fatalf("unexpected users: total=%d records=%d", totalUsers, len(summaries))
	}
	first := summaries[0]
	if first.LoginType != "wechat" || first.AccountCount != 2 || first.BillCount != 2 {
		t.Fatalf("unexpected first summary: %+v", first)
	}
	if !first.ExpenseTotal.Equal(decimal.NewFromInt(10)) || !first.IncomeTotal.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("unexpected totals: expense=%s income=%s", first.ExpenseTotal, first.IncomeTotal)
	}
	if first.LastOccurredAt == nil || !first.LastOccurredAt.Equal(feb) {
		t.Fatalf("unexpected latest bill time: %v", first.LastOccurredAt)
	}
	if summaries[1].BillCount != 0 || summaries[1].LastOccurredAt != nil {
		t.Fatalf("user without bills should have empty summary: %+v", summaries[1])
	}

	bills, totalBills, err := svc.ListBills(OpsBillFilter{UserID: users[0].ID, Month: "2026-01", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if totalBills != 1 || len(bills) != 1 || bills[0].Remark != "午餐" {
		t.Fatalf("unexpected January bills: total=%d records=%+v", totalBills, bills)
	}
	allBills, totalBills, err := svc.ListBills(OpsBillFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if totalBills != 2 || len(allBills) != 2 {
		t.Fatalf("synced copy should be excluded: total=%d records=%d", totalBills, len(allBills))
	}
	if _, _, err := svc.ListBills(OpsBillFilter{Month: "2026-13"}); err == nil {
		t.Fatal("invalid month should fail")
	}
	if _, _, err := svc.ListUserSummaries(1, 201); err == nil {
		t.Fatal("page size above the limit should fail")
	}
}
