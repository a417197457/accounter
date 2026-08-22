package service

import (
	"fmt"
	"testing"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestDeleteSharedLedgerKeepsPersonalSource(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "ledger_delete_owner", PasswordHash: "x"}
	member := model.User{Username: "ledger_delete_member", PasswordHash: "x"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	personal := createSyncTestLedger(t, db, owner.ID, "个人", model.LedgerTypePersonal)
	shared := createSyncTestLedger(t, db, owner.ID, "旅行", model.LedgerTypeTravel)
	if err := db.Create(&model.Membership{LedgerID: shared.ID, UserID: member.ID, Role: "member"}).Error; err != nil {
		t.Fatal(err)
	}
	personalAccount, personalCategory := createSyncTestData(t, db, personal.ID, owner.ID)
	sharedAccount, sharedCategory := createSyncTestData(t, db, shared.ID, owner.ID)
	source := model.Transaction{
		LedgerID: personal.ID, UserID: owner.ID, AccountID: personalAccount.ID, CategoryID: &personalCategory.ID,
		OpType: "expense", Amount: decimal.NewFromInt(1688), OccurredAt: time.Now(),
	}
	if err := db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	copy := model.Transaction{
		LedgerID: shared.ID, UserID: owner.ID, AccountID: sharedAccount.ID, CategoryID: &sharedCategory.ID,
		OpType: "expense", Amount: source.Amount, OccurredAt: source.OccurredAt, SourceID: &source.ID,
	}
	if err := db.Create(&copy).Error; err != nil {
		t.Fatal(err)
	}

	svc := &LedgerService{db: db}
	if _, err := svc.Delete(shared.ID, member.ID); err == nil {
		t.Fatal("member must not delete a ledger")
	}
	if _, err := svc.Delete(personal.ID, owner.ID); err == nil {
		t.Fatal("personal ledger must not be deleted")
	}
	if _, err := svc.Delete(shared.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&model.Ledger{}, shared.ID).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("shared ledger should be deleted, got %v", err)
	}
	if err := db.First(&model.Transaction{}, copy.ID).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("shared copy should be deleted, got %v", err)
	}
	if err := db.First(&model.Transaction{}, source.ID).Error; err != nil {
		t.Fatalf("personal source must remain: %v", err)
	}
	var memberships int64
	if err := db.Model(&model.Membership{}).Where("ledger_id = ?", shared.ID).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if memberships != 0 {
		t.Fatalf("memberships=%d, want 0", memberships)
	}
}
