package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const activationAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

var (
	ErrAIAccessExpired        = errors.New("AI试用已结束，请激活后继续使用")
	ErrAIDailyLimitReached    = errors.New("今日AI使用次数已达上限，请明天再试")
	ErrActivationCodeInvalid  = errors.New("激活码无效或已被使用")
	ErrActivationCodeExpired  = errors.New("激活码已过期")
	ErrActivationRateLimited  = errors.New("激活尝试过于频繁，请一小时后再试")
	ErrActivationCodeRedeemed = errors.New("已兑换的激活码不能作废")
	ErrActivationCodeNotFound = errors.New("激活码不存在")
)

// AIEntitlementView 是前端可见的 AI 权益状态。
type AIEntitlementView struct {
	Status           string     `json:"status"` // not_started | trial | expired | activated
	AccessAllowed    bool       `json:"access_allowed"`
	TrialDays        int        `json:"trial_days"`
	TrialStartedAt   *time.Time `json:"trial_started_at,omitempty"`
	TrialExpiresAt   *time.Time `json:"trial_expires_at,omitempty"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	DailyUsed        int        `json:"daily_used"`
	DailyLimit       int        `json:"daily_limit"`
}

// ActivationCodeCreateOptions 是管理命令生成激活码时的批次参数。
type ActivationCodeCreateOptions struct {
	Count      int
	BatchName  string
	ExpiresAt  *time.Time
	AssignedTo string
	Note       string
}

// PlainActivationCode 仅在创建时返回一次明文。
type PlainActivationCode struct {
	ID        uint
	Code      string
	BatchName string
	ExpiresAt *time.Time
}

// ActivationCodeSummary 是管理命令可安全展示的激活码摘要。
type ActivationCodeSummary struct {
	ID               uint
	DisplaySuffix    string
	BatchName        string
	Status           string
	ExpiresAt        *time.Time
	AssignedTo       string
	RedeemedByUserID *uint
	RedeemedAt       *time.Time
	CreatedAt        time.Time
}

// AIEntitlementService 管理试用期、每日调用额度和永久激活。
type AIEntitlementService struct {
	db                       *gorm.DB
	now                      func() time.Time
	activationSecret         []byte
	trialDays                int
	trialDailyLimit          int
	activatedDailyLimit      int
	activationAttemptLimit   int
	activationIPAttemptLimit int
}

func NewAIEntitlementService() *AIEntitlementService {
	cfg := config.Get()
	return &AIEntitlementService{
		db:                       repository.GetDB(),
		now:                      time.Now,
		activationSecret:         []byte(cfg.AI.ActivationSecret),
		trialDays:                cfg.AI.TrialDays,
		trialDailyLimit:          cfg.AI.TrialDailyLimit,
		activatedDailyLimit:      cfg.AI.ActivatedDailyLimit,
		activationAttemptLimit:   cfg.AI.ActivationAttemptLimit,
		activationIPAttemptLimit: cfg.AI.ActivationIPAttemptLimit,
	}
}

func (s *AIEntitlementService) normalizedLimits() (int, int, int, int, int) {
	trialDays := s.trialDays
	if trialDays <= 0 {
		trialDays = 7
	}
	trialLimit := s.trialDailyLimit
	if trialLimit <= 0 {
		trialLimit = 50
	}
	activatedLimit := s.activatedDailyLimit
	if activatedLimit <= 0 {
		activatedLimit = 500
	}
	userAttemptLimit := s.activationAttemptLimit
	if userAttemptLimit <= 0 {
		userAttemptLimit = 5
	}
	ipAttemptLimit := s.activationIPAttemptLimit
	if ipAttemptLimit <= 0 {
		ipAttemptLimit = 30
	}
	return trialDays, trialLimit, activatedLimit, userAttemptLimit, ipAttemptLimit
}

func (s *AIEntitlementService) Status(userID uint) (*AIEntitlementView, error) {
	now := s.now().UTC()
	trialDays, trialLimit, activatedLimit, _, _ := s.normalizedLimits()
	view := &AIEntitlementView{
		Status:        "not_started",
		AccessAllowed: true,
		TrialDays:     trialDays,
		DailyLimit:    trialLimit,
	}

	var entitlement model.AIEntitlement
	err := s.db.Where("user_id = ?", userID).First(&entitlement).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil {
		view.TrialStartedAt = entitlement.TrialStartedAt
		view.TrialExpiresAt = entitlement.TrialExpiresAt
		view.ActivatedAt = entitlement.ActivatedAt
		switch {
		case entitlement.ActivatedAt != nil:
			view.Status = "activated"
			view.DailyLimit = activatedLimit
		case entitlement.TrialStartedAt == nil || entitlement.TrialExpiresAt == nil:
			view.Status = "not_started"
		case now.Before(entitlement.TrialExpiresAt.UTC()):
			view.Status = "trial"
			view.RemainingSeconds = int64(entitlement.TrialExpiresAt.UTC().Sub(now).Seconds())
		default:
			view.Status = "expired"
			view.AccessAllowed = false
		}
	}

	usageDate := now.In(time.Local).Format("2006-01-02")
	var usage model.AIDailyUsage
	if err := s.db.Where("user_id = ? AND usage_date = ?", userID, usageDate).First(&usage).Error; err == nil {
		view.DailyUsed = usage.RequestCount
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if view.AccessAllowed && view.DailyUsed >= view.DailyLimit {
		view.AccessAllowed = false
	}
	return view, nil
}

func (s *AIEntitlementService) CheckAccess(userID uint) (*AIEntitlementView, error) {
	view, err := s.Status(userID)
	if err != nil {
		return nil, err
	}
	if view.Status == "expired" {
		return view, ErrAIAccessExpired
	}
	if view.DailyUsed >= view.DailyLimit {
		return view, ErrAIDailyLimitReached
	}
	return view, nil
}

// RecordSuccessfulUse 在模型成功返回后开始试用并累计一次调用。
func (s *AIEntitlementService) RecordSuccessfulUse(userID uint) (*AIEntitlementView, error) {
	now := s.now().UTC()
	trialDays, _, _, _, _ := s.normalizedLimits()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var entitlement model.AIEntitlement
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&entitlement).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			expiresAt := now.Add(time.Duration(trialDays) * 24 * time.Hour)
			entitlement = model.AIEntitlement{
				UserID:         userID,
				TrialStartedAt: &now,
				TrialExpiresAt: &expiresAt,
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&entitlement).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if entitlement.ActivatedAt == nil && entitlement.TrialStartedAt == nil {
			expiresAt := now.Add(time.Duration(trialDays) * 24 * time.Hour)
			if err := tx.Model(&entitlement).Updates(map[string]interface{}{
				"trial_started_at": now,
				"trial_expires_at": expiresAt,
			}).Error; err != nil {
				return err
			}
		}

		usage := model.AIDailyUsage{
			UserID:       userID,
			UsageDate:    now.In(time.Local).Format("2006-01-02"),
			RequestCount: 1,
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "usage_date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"request_count": gorm.Expr("request_count + ?", 1)}),
		}).Create(&usage).Error
	}); err != nil {
		return nil, err
	}
	return s.Status(userID)
}

func normalizeActivationCode(code string) (string, error) {
	var builder strings.Builder
	for _, char := range strings.ToUpper(strings.TrimSpace(code)) {
		if char == '-' || unicode.IsSpace(char) {
			continue
		}
		builder.WriteRune(char)
	}
	normalized := builder.String()
	if len(normalized) != 18 || !strings.HasPrefix(normalized, "QZ") {
		return "", ErrActivationCodeInvalid
	}
	for _, char := range normalized[2:] {
		if !strings.ContainsRune(activationAlphabet, char) {
			return "", ErrActivationCodeInvalid
		}
	}
	return normalized, nil
}

func formatActivationCode(payload string) string {
	return "QZ-" + payload[0:4] + "-" + payload[4:8] + "-" + payload[8:12] + "-" + payload[12:16]
}

func randomActivationCode() (string, error) {
	payload := make([]byte, 16)
	max := big.NewInt(int64(len(activationAlphabet)))
	for i := range payload {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		payload[i] = activationAlphabet[index.Int64()]
	}
	return formatActivationCode(string(payload)), nil
}

func (s *AIEntitlementService) codeHash(normalizedCode string) string {
	mac := hmac.New(sha256.New, s.activationSecret)
	_, _ = mac.Write([]byte("activation:" + normalizedCode))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AIEntitlementService) ipHash(ip string) string {
	mac := hmac.New(sha256.New, s.activationSecret)
	_, _ = mac.Write([]byte("ip:" + strings.TrimSpace(ip)))
	return hex.EncodeToString(mac.Sum(nil))
}

func activationSuffix(normalizedCode string) string {
	if len(normalizedCode) <= 4 {
		return normalizedCode
	}
	return normalizedCode[len(normalizedCode)-4:]
}

func (s *AIEntitlementService) checkActivationRateLimit(userID uint, ipHash string) error {
	now := s.now().UTC()
	_, _, _, userLimit, ipLimit := s.normalizedLimits()
	since := now.Add(-time.Hour)
	var userFailures int64
	if err := s.db.Model(&model.AIActivationAttempt{}).
		Where("user_id = ? AND success = ? AND created_at >= ?", userID, false, since).
		Count(&userFailures).Error; err != nil {
		return err
	}
	if userFailures >= int64(userLimit) {
		return ErrActivationRateLimited
	}
	var ipFailures int64
	if err := s.db.Model(&model.AIActivationAttempt{}).
		Where("ip_hash = ? AND success = ? AND created_at >= ?", ipHash, false, since).
		Count(&ipFailures).Error; err != nil {
		return err
	}
	if ipFailures >= int64(ipLimit) {
		return ErrActivationRateLimited
	}
	return nil
}

func activationFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrActivationCodeExpired):
		return "expired"
	case errors.Is(err, ErrActivationRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrActivationCodeInvalid):
		return "invalid"
	default:
		return "internal"
	}
}

func (s *AIEntitlementService) recordActivationAttempt(userID uint, ipHash, suffix string, success bool, failure error) {
	attempt := model.AIActivationAttempt{
		UserID:     userID,
		IPHash:     ipHash,
		CodeSuffix: suffix,
		Success:    success,
		CreatedAt:  s.now().UTC(),
	}
	if failure != nil {
		attempt.FailureCode = activationFailureCode(failure)
	}
	_ = s.db.Create(&attempt).Error
	_ = s.db.Where("created_at < ?", s.now().UTC().AddDate(0, 0, -30)).Delete(&model.AIActivationAttempt{}).Error
}

func (s *AIEntitlementService) Activate(userID uint, rawCode, clientIP string) (*AIEntitlementView, error) {
	current, err := s.Status(userID)
	if err != nil {
		return nil, err
	}
	if current.Status == "activated" {
		return current, nil
	}

	ipHash := s.ipHash(clientIP)
	if err := s.checkActivationRateLimit(userID, ipHash); err != nil {
		return nil, err
	}
	normalized, err := normalizeActivationCode(rawCode)
	if err != nil {
		s.recordActivationAttempt(userID, ipHash, "", false, err)
		return nil, err
	}
	suffix := activationSuffix(normalized)
	now := s.now().UTC()

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var code model.ActivationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ?", s.codeHash(normalized)).First(&code).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrActivationCodeInvalid
			}
			return err
		}
		if code.RevokedAt != nil {
			return ErrActivationCodeInvalid
		}
		if code.ExpiresAt != nil && !now.Before(code.ExpiresAt.UTC()) {
			return ErrActivationCodeExpired
		}
		if code.RedeemedAt != nil {
			if code.RedeemedByUserID == nil || *code.RedeemedByUserID != userID {
				return ErrActivationCodeInvalid
			}
		} else {
			result := tx.Model(&model.ActivationCode{}).
				Where("id = ? AND redeemed_at IS NULL AND revoked_at IS NULL", code.ID).
				Updates(map[string]interface{}{"redeemed_by_user_id": userID, "redeemed_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrActivationCodeInvalid
			}
		}

		var entitlement model.AIEntitlement
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&entitlement).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return tx.Create(&model.AIEntitlement{
				UserID:           userID,
				ActivatedAt:      &now,
				ActivationCodeID: &code.ID,
			}).Error
		}
		if findErr != nil {
			return findErr
		}
		return tx.Model(&entitlement).Updates(map[string]interface{}{
			"activated_at":       now,
			"activation_code_id": code.ID,
		}).Error
	})
	if err != nil {
		s.recordActivationAttempt(userID, ipHash, suffix, false, err)
		return nil, err
	}
	s.recordActivationAttempt(userID, ipHash, suffix, true, nil)
	return s.Status(userID)
}

func (s *AIEntitlementService) CreateActivationCodes(options ActivationCodeCreateOptions) ([]PlainActivationCode, error) {
	if options.Count <= 0 || options.Count > 1000 {
		return nil, fmt.Errorf("单次生成数量必须为 1-1000")
	}
	if len(s.activationSecret) < 16 {
		return nil, fmt.Errorf("AI_ACTIVATION_SECRET 未正确配置")
	}

	created := make([]PlainActivationCode, 0, options.Count)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for len(created) < options.Count {
			plain, err := randomActivationCode()
			if err != nil {
				return err
			}
			normalized, _ := normalizeActivationCode(plain)
			record := model.ActivationCode{
				CodeHash:      s.codeHash(normalized),
				DisplaySuffix: activationSuffix(normalized),
				BatchName:     strings.TrimSpace(options.BatchName),
				ExpiresAt:     options.ExpiresAt,
				AssignedTo:    strings.TrimSpace(options.AssignedTo),
				Note:          strings.TrimSpace(options.Note),
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			created = append(created, PlainActivationCode{
				ID: record.ID, Code: plain, BatchName: record.BatchName, ExpiresAt: record.ExpiresAt,
			})
		}
		return nil
	})
	return created, err
}

func (s *AIEntitlementService) ListActivationCodes(batchName, status string) ([]ActivationCodeSummary, error) {
	query := s.db.Model(&model.ActivationCode{}).Order("id DESC").Limit(2000)
	if strings.TrimSpace(batchName) != "" {
		query = query.Where("batch_name = ?", strings.TrimSpace(batchName))
	}
	var records []model.ActivationCode
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	now := s.now().UTC()
	result := make([]ActivationCodeSummary, 0, len(records))
	for _, record := range records {
		currentStatus := "unused"
		switch {
		case record.RevokedAt != nil:
			currentStatus = "revoked"
		case record.RedeemedAt != nil:
			currentStatus = "redeemed"
		case record.ExpiresAt != nil && !now.Before(record.ExpiresAt.UTC()):
			currentStatus = "expired"
		}
		if status != "" && status != "all" && status != currentStatus {
			continue
		}
		result = append(result, ActivationCodeSummary{
			ID: record.ID, DisplaySuffix: record.DisplaySuffix, BatchName: record.BatchName,
			Status: currentStatus, ExpiresAt: record.ExpiresAt, AssignedTo: record.AssignedTo,
			RedeemedByUserID: record.RedeemedByUserID, RedeemedAt: record.RedeemedAt, CreatedAt: record.CreatedAt,
		})
	}
	return result, nil
}

func (s *AIEntitlementService) RevokeActivationCode(id uint) error {
	var code model.ActivationCode
	if err := s.db.First(&code, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrActivationCodeNotFound
		}
		return err
	}
	if code.RedeemedAt != nil {
		return ErrActivationCodeRedeemed
	}
	if code.RevokedAt != nil {
		return nil
	}
	now := s.now().UTC()
	return s.db.Model(&code).Update("revoked_at", now).Error
}
