package model

import (
	"testing"
	"time"
)

func TestBillingCycleRange(t *testing.T) {
	loc := time.Local
	// 账单日 15：3 月 20 日 → 本账期 3/16–4/15
	from := time.Date(2026, 3, 20, 10, 0, 0, 0, loc)
	start, end := BillingCycleRange(15, from, 0)
	if start.Format("2006-01-02") != "2026-03-16" {
		t.Fatalf("start=%s", start)
	}
	if end.Format("2006-01-02") != "2026-04-15" {
		t.Fatalf("end=%s", end)
	}
	// 3 月 10 日 → 本账期 2/16–3/15
	from = time.Date(2026, 3, 10, 10, 0, 0, 0, loc)
	start, end = BillingCycleRange(15, from, 0)
	if start.Format("2006-01-02") != "2026-02-16" || end.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("start=%s end=%s", start, end)
	}
}

func TestDaysUntilDue(t *testing.T) {
	loc := time.Local
	from := time.Date(2026, 3, 20, 10, 0, 0, 0, loc)
	if d := DaysUntilDue(22, from); d != 2 {
		t.Fatalf("want 2 got %d", d)
	}
	if d := DaysUntilDue(20, from); d != 0 {
		t.Fatalf("want 0 got %d", d)
	}
}
