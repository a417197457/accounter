package ai

import (
	"testing"
	"time"
)

func TestNormalizeTimeAtUsesCurrentClockForDateOnly(t *testing.T) {
	now := time.Date(2026, 8, 8, 14, 35, 27, 0, time.Local)
	if got := normalizeTimeAt("2026-08-07", now); got != "2026-08-07 14:35:27" {
		t.Fatalf("纯日期应沿用当前时分秒，实际 %q", got)
	}
	if got := normalizeTimeAt("2026-08-07 09:20", now); got != "2026-08-07 09:20:00" {
		t.Fatalf("缺少秒时应补零，实际 %q", got)
	}
	if got := normalizeTimeAt("2026-08-07 09:20:18", now); got != "2026-08-07 09:20:18" {
		t.Fatalf("完整时间不应被修改，实际 %q", got)
	}
}
