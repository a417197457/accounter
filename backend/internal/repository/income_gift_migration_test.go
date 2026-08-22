package repository

import (
	"fmt"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
)

func TestEnsureIncomeGiftCategoriesIsIdempotent(t *testing.T) {
	db, err := NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "income_gift_migration", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger := model.Ledger{Name: "个人", OwnerID: user.ID, Type: model.LedgerTypePersonal, InviteCode: "IGM001"}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	other := model.Category{LedgerID: ledger.ID, UserID: user.ID, Name: "其他", Kind: "income", Sort: 6}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	if err := ensureIncomeGiftCategories(db); err != nil {
		t.Fatal(err)
	}
	if err := ensureIncomeGiftCategories(db); err != nil {
		t.Fatal(err)
	}
	var gifts []model.Category
	if err := db.Where("ledger_id = ? AND name = ? AND kind = ?", ledger.ID, "礼金", "income").Find(&gifts).Error; err != nil {
		t.Fatal(err)
	}
	if len(gifts) != 1 || gifts[0].Sort != 6 {
		t.Fatalf("unexpected income gift categories: %+v", gifts)
	}
	if err := db.First(&other, other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if other.Sort != 7 {
		t.Fatalf("other sort=%d, want 7", other.Sort)
	}
}
