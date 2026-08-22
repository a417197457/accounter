package service

import (
	"errors"
	"fmt"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
)

func TestLedgerCreateLimitExcludesPersonalAndJoinedLedgers(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "ledger_limit_owner", PasswordHash: "x"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	personal := model.Ledger{Name: "个人", OwnerID: owner.ID, Type: model.LedgerTypePersonal, InviteCode: "LIMPER"}
	if err := db.Create(&personal).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 9; i++ {
		ledger := model.Ledger{
			Name: fmt.Sprintf("账本%d", i), OwnerID: owner.ID, Type: model.LedgerTypeTravel,
			InviteCode: fmt.Sprintf("LIM%03d", i),
		}
		if err := db.Create(&ledger).Error; err != nil {
			t.Fatal(err)
		}
	}
	otherOwner := model.User{Username: "ledger_limit_other", PasswordHash: "x"}
	if err := db.Create(&otherOwner).Error; err != nil {
		t.Fatal(err)
	}
	joined := model.Ledger{Name: "别人创建的账本", OwnerID: otherOwner.ID, Type: model.LedgerTypeFamily, InviteCode: "LIMJOIN"}
	if err := db.Create(&joined).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Membership{LedgerID: joined.ID, UserID: owner.ID, Role: "member"}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &LedgerService{db: db}
	if _, _, err := svc.Create(owner.ID, "第十个", model.LedgerTypeGift); err != nil {
		t.Fatalf("the tenth shared ledger should be allowed: %v", err)
	}
	if _, _, err := svc.Create(owner.ID, "第十一个", model.LedgerTypeFamily); !errors.Is(err, ErrLedgerCreateLimit) {
		t.Fatalf("expected ledger limit error, got %v", err)
	}
}
