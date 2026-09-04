package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
)

func setupAIEntitlementTest(t *testing.T) (*AIEntitlementService, model.User, *time.Time) {
	t.Helper()
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	user := model.User{Username: "entitlement_user", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 8, 4, 0, 0, 0, time.UTC)
	svc := &AIEntitlementService{
		db:                       db,
		now:                      func() time.Time { return now },
		activationSecret:         []byte("test-activation-secret-at-least-32-bytes"),
		trialDays:                7,
		trialDailyLimit:          2,
		activatedDailyLimit:      5,
		activationAttemptLimit:   5,
		activationIPAttemptLimit: 30,
	}
	return svc, user, &now
}

func TestAITrialStartsOnFirstSuccessfulUse(t *testing.T) {
	svc, user, now := setupAIEntitlementTest(t)

	before, err := svc.CheckAccess(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != "not_started" || before.TrialStartedAt != nil {
		t.Fatalf("首次使用前不应启动试用: %+v", before)
	}

	after, err := svc.RecordSuccessfulUse(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "trial" || after.TrialStartedAt == nil || after.TrialExpiresAt == nil {
		t.Fatalf("成功调用后应启动试用: %+v", after)
	}
	wantExpiry := now.Add(7 * 24 * time.Hour)
	if !after.TrialExpiresAt.Equal(wantExpiry) {
		t.Fatalf("试用到期时间错误: want=%s got=%s", wantExpiry, after.TrialExpiresAt)
	}
	if after.DailyUsed != 1 {
		t.Fatalf("成功调用次数应为 1，实际 %d", after.DailyUsed)
	}
}

func TestAITrialDailyLimitAndExpiration(t *testing.T) {
	svc, user, now := setupAIEntitlementTest(t)
	if _, err := svc.RecordSuccessfulUse(user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordSuccessfulUse(user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CheckAccess(user.ID); !errors.Is(err, ErrAIDailyLimitReached) {
		t.Fatalf("应触发每日上限，实际: %v", err)
	}

	*now = now.Add(8 * 24 * time.Hour)
	if _, err := svc.CheckAccess(user.ID); !errors.Is(err, ErrAIAccessExpired) {
		t.Fatalf("应触发试用到期，实际: %v", err)
	}
}

func TestActivationCodeIsOneTimeAndStoredAsHash(t *testing.T) {
	svc, user, now := setupAIEntitlementTest(t)
	*now = now.Add(8 * 24 * time.Hour)

	codes, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{Count: 1, BatchName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 1 || !strings.HasPrefix(codes[0].Code, "QZ-") {
		t.Fatalf("激活码格式错误: %+v", codes)
	}
	activated, err := svc.Activate(user.ID, strings.ToLower(codes[0].Code), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if activated.Status != "activated" || !activated.AccessAllowed {
		t.Fatalf("激活状态错误: %+v", activated)
	}
	if activated.ActivatedExpiresAt != nil {
		t.Fatalf("未指定期限的旧式激活码应保持永久权益: %+v", activated)
	}

	var stored model.ActivationCode
	if err := svc.db.First(&stored, codes[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CodeHash == codes[0].Code || strings.Contains(stored.CodeHash, "QZ") {
		t.Fatal("数据库不应保存激活码明文")
	}

	other := model.User{Username: "other_user", PasswordHash: "x"}
	if err := svc.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Activate(other.ID, codes[0].Code, "127.0.0.2"); !errors.Is(err, ErrActivationCodeInvalid) {
		t.Fatalf("同一激活码不应被第二个用户使用: %v", err)
	}
}

func TestTimedActivationUsesCalendarMonthsAndCanBeRenewedAfterExpiry(t *testing.T) {
	svc, user, now := setupAIEntitlementTest(t)
	*now = time.Date(2026, 1, 31, 12, 30, 0, 0, time.UTC)

	codes, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{
		Count: 1, BatchName: "one-month", DurationMonths: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if codes[0].DurationMonths != 1 {
		t.Fatalf("创建结果应返回权益月数: %+v", codes[0])
	}

	activated, err := svc.Activate(user.ID, codes[0].Code, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	wantExpiry := time.Date(2026, 2, 28, 12, 30, 0, 0, time.UTC)
	if activated.ActivatedExpiresAt == nil || !activated.ActivatedExpiresAt.Equal(wantExpiry) {
		t.Fatalf("月末到期时间应截断到目标月最后一天: want=%s got=%v", wantExpiry, activated.ActivatedExpiresAt)
	}
	if activated.Status != "activated" || activated.RemainingSeconds <= 0 {
		t.Fatalf("限时权益应处于激活状态: %+v", activated)
	}

	*now = wantExpiry
	if _, err := svc.CheckAccess(user.ID); !errors.Is(err, ErrAIAccessExpired) {
		t.Fatalf("到达权益截止时间后应禁止 AI 使用: %v", err)
	}

	renewalCodes, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{
		Count: 1, BatchName: "three-month", DurationMonths: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := svc.Activate(user.ID, renewalCodes[0].Code, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	wantRenewedExpiry := time.Date(2026, 5, 28, 12, 30, 0, 0, time.UTC)
	if renewed.ActivatedExpiresAt == nil || !renewed.ActivatedExpiresAt.Equal(wantRenewedExpiry) {
		t.Fatalf("续期后的到期时间错误: want=%s got=%v", wantRenewedExpiry, renewed.ActivatedExpiresAt)
	}

	summaries, err := svc.ListActivationCodes("three-month", "redeemed")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].DurationMonths != 3 {
		t.Fatalf("激活码列表应展示权益月数: %+v", summaries)
	}
}

func TestActivationCodeDurationValidation(t *testing.T) {
	svc, _, _ := setupAIEntitlementTest(t)
	for _, months := range []int{-1, 121} {
		if _, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{Count: 1, DurationMonths: months}); err == nil {
			t.Fatalf("期限 %d 个月应校验失败", months)
		}
	}
}

func TestActivationAttemptsAreRateLimited(t *testing.T) {
	svc, user, _ := setupAIEntitlementTest(t)
	svc.activationAttemptLimit = 2
	for i := 0; i < 2; i++ {
		if _, err := svc.Activate(user.ID, "invalid", "127.0.0.1"); !errors.Is(err, ErrActivationCodeInvalid) {
			t.Fatalf("第 %d 次应为无效码: %v", i+1, err)
		}
	}
	if _, err := svc.Activate(user.ID, "invalid", "127.0.0.1"); !errors.Is(err, ErrActivationRateLimited) {
		t.Fatalf("应触发激活限流: %v", err)
	}
}

func TestExpiredAndRevokedActivationCodesCannotBeUsed(t *testing.T) {
	svc, user, now := setupAIEntitlementTest(t)
	expiredAt := now.Add(-time.Minute)
	expiredCodes, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{Count: 1, ExpiresAt: &expiredAt})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Activate(user.ID, expiredCodes[0].Code, "127.0.0.1"); !errors.Is(err, ErrActivationCodeExpired) {
		t.Fatalf("过期码不应允许兑换: %v", err)
	}

	activeCodes, err := svc.CreateActivationCodes(ActivationCodeCreateOptions{Count: 1, BatchName: "revoke-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeActivationCode(activeCodes[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Activate(user.ID, activeCodes[0].Code, "127.0.0.1"); !errors.Is(err, ErrActivationCodeInvalid) {
		t.Fatalf("已作废的激活码不应允许兑换: %v", err)
	}
	revoked, err := svc.ListActivationCodes("revoke-test", "revoked")
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 1 || revoked[0].ID != activeCodes[0].ID {
		t.Fatalf("作废状态查询错误: %+v", revoked)
	}
}
