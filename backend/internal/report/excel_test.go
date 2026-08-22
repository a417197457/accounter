package report

import (
	"testing"
	"time"

	"accounter/backend/internal/model"

	"github.com/shopspring/decimal"
)

func TestExportTransactionsWritesHeadersAndRows(t *testing.T) {
	category := model.Category{Name: "午餐"}
	account := model.Account{Name: "支付宝余额"}
	member := model.User{Nickname: "测试用户"}
	tx := model.Transaction{
		OpType:     "expense",
		Amount:     decimal.RequireFromString("25.80"),
		Remark:     "工作餐",
		OccurredAt: time.Date(2026, 8, 22, 12, 30, 0, 0, time.Local),
		Category:   &category,
		Account:    &account,
		Member:     &member,
	}

	f, err := ExportTransactions([]model.Transaction{tx}, "2026-08")
	if err != nil {
		t.Fatalf("ExportTransactions() error = %v", err)
	}
	defer f.Close()

	checks := map[string]string{
		"A1": "日期",
		"A2": "2026-08-22 12:30",
		"B2": "午餐",
		"C2": "支付宝余额",
		"F2": "支出",
		"G2": "25.8",
		"H2": "工作餐",
		"I2": "测试用户",
	}
	for cell, want := range checks {
		got, getErr := f.GetCellValue("流水明细", cell)
		if getErr != nil {
			t.Fatalf("GetCellValue(%s) error = %v", cell, getErr)
		}
		if got != want {
			t.Errorf("cell %s = %q, want %q", cell, got, want)
		}
	}
}
