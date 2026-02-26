package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"telegram-ai-face-bot/internal/models"
)

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func generateReferralCode() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (s *UserService) GetOrCreateUser(telegramID int64, username, firstName, lastName, languageCode string) (*models.User, error) {
	_, err := s.GetUserByTelegramID(telegramID)
	if err == nil {
		return s.UpdateUserInfo(telegramID, username, firstName, lastName, languageCode)
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	return s.CreateUser(telegramID, username, firstName, lastName, languageCode, nil)
}
func (s *UserService) GetOrCreateUserWithReferrer(telegramID int64, username, firstName, lastName, languageCode string, referrerCode string) (*models.User, error) {
	_, err := s.GetUserByTelegramID(telegramID)
	if err == nil {
		return s.UpdateUserInfo(telegramID, username, firstName, lastName, languageCode)
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	var referrerID *int64
	if referrerCode != "" {
		referrer, err := s.GetUserByReferralCode(referrerCode)
		if err == nil && referrer.TelegramID != telegramID {
			referrerID = &referrer.TelegramID
		}
	}

	user, err := s.CreateUser(telegramID, username, firstName, lastName, languageCode, referrerID)
	if err != nil {
		return nil, err
	}

	if referrerID != nil {
		s.AddReferralBonus(*referrerID)
	}

	return user, nil
}

func (s *UserService) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
			   is_premium, is_admin, COALESCE(referrer_id, 0), COALESCE(referral_code, ''), 
			   COALESCE(referrals_count, 0), created_at, updated_at, is_blocked
		FROM users WHERE telegram_id = $1`

	user := &models.User{}
	var referrerID int64
	err := s.db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&referrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)

	if referrerID != 0 {
		user.ReferrerID = &referrerID
	}

	return user, err
}

func (s *UserService) GetUserByReferralCode(code string) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
			   is_premium, is_admin, COALESCE(referrer_id, 0), COALESCE(referral_code, ''), 
			   COALESCE(referrals_count, 0), created_at, updated_at, is_blocked
		FROM users WHERE referral_code = $1`

	user := &models.User{}
	var referrerID int64
	err := s.db.QueryRow(query, code).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&referrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)

	if referrerID != 0 {
		user.ReferrerID = &referrerID
	}

	return user, err
}

func (s *UserService) CreateUser(telegramID int64, username, firstName, lastName, languageCode string, referrerID *int64) (*models.User, error) {
	referralCode := generateReferralCode()

	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, language_code, referrer_id, referral_code, referrals_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0)
		RETURNING id, telegram_id, username, first_name, last_name, language_code,
				  is_premium, is_admin, created_at, updated_at, is_blocked`

	user := &models.User{}
	err := s.db.QueryRow(query, telegramID, username, firstName, lastName, languageCode, referrerID, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)
	if err != nil {
		return nil, err
	}

	if _, quotaErr := s.db.Exec(`
		INSERT INTO user_quotas (telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra)
		VALUES ($1, 10, 0, 0, 0, 0, 0, 0, 0)
		ON CONFLICT (telegram_id) DO NOTHING
	`, telegramID); quotaErr != nil {
		return nil, quotaErr
	}

	user.ReferrerID = referrerID
	user.ReferralCode = referralCode

	return user, nil
}

func (s *UserService) UpdateUserInfo(telegramID int64, username, firstName, lastName, languageCode string) (*models.User, error) {
	query := `
		UPDATE users
		SET username = $2, first_name = $3, last_name = $4, language_code = $5, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
		RETURNING id, telegram_id, username, first_name, last_name, language_code,
				  is_premium, is_admin, created_at, updated_at, is_blocked`

	user := &models.User{}
	err := s.db.QueryRow(query, telegramID, username, firstName, lastName, languageCode).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)

	return user, err
}

func (s *UserService) GetQuota(telegramID int64) (*models.UserQuota, error) {
	if err := s.ensureDailyTextQuota(telegramID); err != nil {
		return nil, err
	}
	if err := s.ensureWeeklyQuota(telegramID); err != nil {
		return nil, err
	}
	if err := s.ensureCategorySettings(); err != nil {
		return nil, err
	}
	return s.GetOrCreateUserQuota(telegramID)
}

func (s *UserService) ConsumeQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	_, _, err := s.ConsumeQuotaDetailed(telegramID, category, amount)
	return err
}

// ConsumeQuotaDetailed списывает запросы и возвращает, сколько ушло из основного и дополнительного бакета.
func (s *UserService) ConsumeQuotaDetailed(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error) {
	if amount < 1 {
		amount = 1
	}

	if err = s.ensureDailyTextQuota(telegramID); err != nil {
		return
	}
	if err = s.ensureWeeklyQuota(telegramID); err != nil {
		return
	}
	if err = s.ensureCategorySettings(); err != nil {
		return
	}

	if _, err = s.GetOrCreateUserQuota(telegramID); err != nil {
		return
	}

	tx, txErr := s.db.Begin()
	if txErr != nil {
		err = txErr
		return
	}
	defer tx.Rollback()

	var primaryCol, extraCol string
	switch category {
	case models.QuotaCategoryText:
		primaryCol, extraCol = "text_daily", "text_extra"
	case models.QuotaCategoryImage:
		primaryCol, extraCol = "image_weekly", "image_extra"
	case models.QuotaCategoryMusic:
		primaryCol, extraCol = "music_weekly", "music_extra"
	case models.QuotaCategoryVideo:
		primaryCol, extraCol = "video_weekly", "video_extra"
	default:
		err = fmt.Errorf("unknown quota category: %s", category)
		return
	}

	var primary, extra int
	row := tx.QueryRow(fmt.Sprintf(`SELECT %s, %s FROM user_quotas WHERE telegram_id = $1 FOR UPDATE`, primaryCol, extraCol), telegramID)
	if err = row.Scan(&primary, &extra); err != nil {
		return
	}

	if primary+extra < amount {
		err = fmt.Errorf("недостаточно запросов (%d нужно, доступно %d)", amount, primary+extra)
		return
	}

	origPrimary, origExtra := primary, extra
	remain := amount
	if primary >= remain {
		primary -= remain
		remain = 0
	} else {
		remain -= primary
		primary = 0
	}

	if remain > 0 {
		extra -= remain
	}

	primaryUsed = origPrimary - primary
	extraUsed = origExtra - extra

	if _, err = tx.Exec(fmt.Sprintf(`
		UPDATE user_quotas
		SET %s = $2, %s = $3
		WHERE telegram_id = $1
	`, primaryCol, extraCol), telegramID, primary, extra); err != nil {
		return
	}

	err = tx.Commit()
	return
}

// RefundQuota возвращает запросы в те же бакеты (основной/дополнительный), откуда они были списаны.
func (s *UserService) RefundQuota(telegramID int64, category models.QuotaCategory, primaryAmount, extraAmount int) error {
	if primaryAmount < 0 || extraAmount < 0 {
		return fmt.Errorf("refund amounts must be non-negative")
	}
	if primaryAmount == 0 && extraAmount == 0 {
		return nil
	}

	if _, err := s.GetOrCreateUserQuota(telegramID); err != nil {
		return err
	}
	if err := s.ensureDailyTextQuota(telegramID); err != nil {
		return err
	}
	if err := s.ensureWeeklyQuota(telegramID); err != nil {
		return err
	}
	if err := s.ensureCategorySettings(); err != nil {
		return err
	}

	var primaryCol, extraCol string
	switch category {
	case models.QuotaCategoryText:
		primaryCol, extraCol = "text_daily", "text_extra"
	case models.QuotaCategoryImage:
		primaryCol, extraCol = "image_weekly", "image_extra"
	case models.QuotaCategoryMusic:
		primaryCol, extraCol = "music_weekly", "music_extra"
	case models.QuotaCategoryVideo:
		primaryCol, extraCol = "video_weekly", "video_extra"
	default:
		return fmt.Errorf("unknown quota category: %s", category)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(fmt.Sprintf(`
		UPDATE user_quotas
		SET %s = %s + $2,
			%s = %s + $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, primaryCol, primaryCol, extraCol, extraCol), telegramID, primaryAmount, extraAmount); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserService) AddExtraQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	if amount < 1 {
		return fmt.Errorf("amount must be positive")
	}

	if _, err := s.GetOrCreateUserQuota(telegramID); err != nil {
		return err
	}

	if err := s.ensureDailyTextQuota(telegramID); err != nil {
		return err
	}
	if err := s.ensureWeeklyQuota(telegramID); err != nil {
		return err
	}
	if err := s.ensureCategorySettings(); err != nil {
		return err
	}

	var extraCol string
	switch category {
	case models.QuotaCategoryText:
		extraCol = "text_extra"
	case models.QuotaCategoryImage:
		extraCol = "image_extra"
	case models.QuotaCategoryMusic:
		extraCol = "music_extra"
	case models.QuotaCategoryVideo:
		extraCol = "video_extra"
	default:
		return fmt.Errorf("unknown quota category: %s", category)
	}

	_, err := s.db.Exec(fmt.Sprintf(`
		UPDATE user_quotas
		SET %s = %s + $2, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, extraCol, extraCol), telegramID, amount)
	return err
}

func (s *UserService) ensureChannelTrialColumn() error {
	_, err := s.db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS channel_trial_claimed BOOLEAN NOT NULL DEFAULT FALSE`)
	return err
}

func (s *UserService) HasClaimedChannelTrial(telegramID int64) (bool, error) {
	if err := s.ensureChannelTrialColumn(); err != nil {
		return false, err
	}
	var claimed bool
	err := s.db.QueryRow(`SELECT channel_trial_claimed FROM users WHERE telegram_id = $1`, telegramID).Scan(&claimed)
	return claimed, err
}

func (s *UserService) MarkChannelTrialClaimed(telegramID int64) error {
	if err := s.ensureChannelTrialColumn(); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET channel_trial_claimed = TRUE, updated_at = CURRENT_TIMESTAMP WHERE telegram_id = $1`, telegramID)
	return err
}

var allCategories = []string{"photo", "video", "music", "chat"}

func (s *UserService) ensureCategorySettings() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, cat := range allCategories {
		if _, err := tx.Exec(`INSERT INTO category_settings (category, enabled) VALUES ($1, TRUE) ON CONFLICT (category) DO NOTHING`, cat); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *UserService) GetCategorySettings() ([]models.CategorySetting, error) {
	if err := s.ensureCategorySettings(); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT category, enabled FROM category_settings ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.CategorySetting
	for rows.Next() {
		var c models.CategorySetting
		if err := rows.Scan(&c.Category, &c.Enabled); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *UserService) SetCategoryEnabled(category string, enabled bool) error {
	category = strings.ToLower(category)
	if err := s.ensureCategorySettings(); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE category_settings SET enabled = $2 WHERE category = $1`, category, enabled)
	return err
}

func (s *UserService) IsCategoryEnabled(category string) (bool, error) {
	category = strings.ToLower(category)
	if err := s.ensureCategorySettings(); err != nil {
		return false, err
	}
	var enabled bool
	err := s.db.QueryRow(`SELECT enabled FROM category_settings WHERE category = $1`, category).Scan(&enabled)
	return enabled, err
}

func (s *UserService) ensureAppSettings() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		INSERT INTO app_settings (key, value) VALUES ('payments_enabled', 'true')
		ON CONFLICT (key) DO NOTHING;
		INSERT INTO app_settings (key, value) VALUES ('subscriptions_enabled', 'true')
		ON CONFLICT (key) DO NOTHING;
		INSERT INTO app_settings (key, value) VALUES ('nano_banana_defapi', 'false')
		ON CONFLICT (key) DO NOTHING;
		INSERT INTO app_settings (key, value) VALUES ('nano_banana_provider', 'kieapi')
		ON CONFLICT (key) DO NOTHING;
	`); err != nil {
		return err
	}
	return nil
}

func (s *UserService) SetPaymentsEnabled(enabled bool) error {
	if err := s.ensureAppSettings(); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO app_settings (key, value) VALUES ('payments_enabled', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, strconv.FormatBool(enabled))
	return err
}

func (s *UserService) IsPaymentsEnabled() (bool, error) {
	if err := s.ensureAppSettings(); err != nil {
		return true, err
	}
	var raw string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'payments_enabled'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return true, nil
	}
	return enabled, nil
}

func (s *UserService) SetNanoBananaDefAPIEnabled(enabled bool) error {
	if err := s.ensureAppSettings(); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO app_settings (key, value) VALUES ('nano_banana_defapi', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, strconv.FormatBool(enabled))
	return err
}

func (s *UserService) IsNanoBananaDefAPIEnabled() (bool, error) {
	if err := s.ensureAppSettings(); err != nil {
		return false, err
	}
	var raw string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'nano_banana_defapi'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, nil
	}
	return enabled, nil
}

func (s *UserService) SetNanoBananaProvider(provider string) error {
	if err := s.ensureAppSettings(); err != nil {
		return err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "defapi" {
		provider = "kieapi"
	}
	_, err := s.db.Exec(`INSERT INTO app_settings (key, value) VALUES ('nano_banana_provider', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, provider)
	return err
}

func (s *UserService) GetNanoBananaProvider() (string, error) {
	if err := s.ensureAppSettings(); err != nil {
		return "kieapi", err
	}
	var raw string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'nano_banana_provider'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return "kieapi", nil
	}
	if err != nil {
		return "kieapi", err
	}
	provider := strings.ToLower(strings.TrimSpace(raw))
	if provider != "defapi" {
		provider = "kieapi"
	}
	return provider, nil
}

func (s *UserService) SetSubscriptionsEnabled(enabled bool) error {
	if err := s.ensureAppSettings(); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO app_settings (key, value) VALUES ('subscriptions_enabled', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, strconv.FormatBool(enabled))
	return err
}

func (s *UserService) IsSubscriptionsEnabled() (bool, error) {
	if err := s.ensureAppSettings(); err != nil {
		return true, err
	}
	var raw string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'subscriptions_enabled'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return true, nil
	}
	return enabled, nil
}

func (s *UserService) ensureDailyTextQuota(telegramID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var subType string
	var subEnd *time.Time
	if err := tx.QueryRow(`SELECT subscription_type, subscription_end FROM users WHERE telegram_id = $1`, telegramID).Scan(&subType, &subEnd); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO user_quotas (telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra)
		VALUES ($1, 10, 0, 0, 0, 0, 0, 0, 0)
		ON CONFLICT (telegram_id) DO NOTHING
	`, telegramID); err != nil {
		return err
	}

	var currentDaily int
	var updatedAt time.Time
	if err := tx.QueryRow(`SELECT text_daily, updated_at FROM user_quotas WHERE telegram_id = $1 FOR UPDATE`, telegramID).Scan(&currentDaily, &updatedAt); err != nil {
		return err
	}

	activeSub := s.isSubscriptionActive(subType, subEnd)
	if subType != "" && !activeSub {
		if err := s.resetSubscriptionTx(tx, telegramID); err != nil {
			return err
		}
		subType = ""
		activeSub = false
	}

	msk := time.FixedZone("MSK", 3*3600)
	today := time.Now().In(msk).Format("2006-01-02")
	needsUpdate := updatedAt.In(msk).Format("2006-01-02") != today

	newDaily := 10
	if activeSub {
		switch strings.ToLower(subType) {
		case "mini":
			newDaily = 50
		case "start":
			newDaily = 100
		case "pro":
			newDaily = 200
		}
	}

	if !needsUpdate {
		return tx.Commit()
	}

	if _, err := tx.Exec(`
		UPDATE user_quotas
		SET text_daily = $2, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID, newDaily); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserService) ensureWeeklyQuota(telegramID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var subType string
	var subEnd *time.Time
	if err := tx.QueryRow(`SELECT subscription_type, subscription_end FROM users WHERE telegram_id = $1`, telegramID).Scan(&subType, &subEnd); err != nil {
		return err
	}
	activeSub := s.isSubscriptionActive(subType, subEnd)

	if _, err := tx.Exec(`
		INSERT INTO user_quotas (telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra)
		VALUES ($1, 10, 0, 0, 0, 0, 0, 0, 0)
		ON CONFLICT (telegram_id) DO NOTHING
	`, telegramID); err != nil {
		return err
	}

	var updatedAt time.Time
	if err := tx.QueryRow(`SELECT updated_at FROM user_quotas WHERE telegram_id = $1 FOR UPDATE`, telegramID).Scan(&updatedAt); err != nil {
		return err
	}

	now := time.Now().UTC()
	elapsed := now.Sub(updatedAt.UTC())
	if subType != "" && !activeSub {
		if err := s.resetSubscriptionTx(tx, telegramID); err != nil {
			return err
		}
		subType = ""
		activeSub = false
	}

	if activeSub {
		if elapsed < 7*24*time.Hour {
			return tx.Commit()
		}
		if err := s.setSubscriptionWeekly(telegramID, subType); err != nil {
			return err
		}
		return tx.Commit()
	}

	if elapsed < 7*24*time.Hour {
		return tx.Commit()
	}
	if _, err := tx.Exec(`
		UPDATE user_quotas
		SET image_weekly = $2,
			music_weekly = $3,
			video_weekly = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID, 0, 0, 0); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserService) AddReferralBonus(referrerID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE users SET referrals_count = referrals_count + 1 WHERE telegram_id = $1`, referrerID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserService) AddReferralPurchaseBonus(referrerID int64, category models.QuotaCategory, purchaseQty int) error {
	bonus := purchaseQty * 20 / 100
	if bonus <= 0 {
		return nil
	}

	return s.AddExtraQuota(referrerID, category, bonus)
}

func (s *UserService) GetOrCreateUserQuota(telegramID int64) (*models.UserQuota, error) {
	quota := &models.UserQuota{}

	err := s.db.QueryRow(`
		WITH inserted AS (
			INSERT INTO user_quotas (telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra)
			VALUES ($1, 10, 0, 0, 0, 0, 0, 0, 0)
			ON CONFLICT (telegram_id) DO NOTHING
			RETURNING id, telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra, created_at, updated_at
		)
		SELECT id, telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra, created_at, updated_at
		FROM inserted
		UNION ALL
		SELECT id, telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra, created_at, updated_at
		FROM user_quotas
		WHERE telegram_id = $1
		LIMIT 1
	`, telegramID).Scan(
		&quota.ID, &quota.TelegramID, &quota.TextDaily, &quota.TextExtra, &quota.ImageWeekly, &quota.ImageExtra, &quota.MusicWeekly, &quota.MusicExtra, &quota.VideoWeekly, &quota.VideoExtra,
		&quota.CreatedAt, &quota.UpdatedAt,
	)

	return quota, err
}

func (s *UserService) UpdateUserQuota(telegramID int64, textDaily, imageWeekly, musicWeekly, videoWeekly int) error {
	_, err := s.db.Exec(`
		INSERT INTO user_quotas (telegram_id, text_daily, image_weekly, music_weekly, video_weekly)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (telegram_id) DO UPDATE
		SET text_daily = EXCLUDED.text_daily,
			image_weekly = EXCLUDED.image_weekly,
			music_weekly = EXCLUDED.music_weekly,
			video_weekly = EXCLUDED.video_weekly,
			updated_at = CURRENT_TIMESTAMP
	`, telegramID, textDaily, imageWeekly, musicWeekly, videoWeekly)
	return err
}

func (s *UserService) IsUserAdmin(telegramID int64) (bool, error) {
	var isAdmin bool
	err := s.db.QueryRow(`SELECT is_admin FROM users WHERE telegram_id = $1`, telegramID).Scan(&isAdmin)
	return isAdmin, err
}

func (s *UserService) GetAdminUsers() ([]*models.User, error) {
	rows, err := s.db.Query(`
		SELECT telegram_id, username, first_name, last_name
		FROM users
		WHERE is_admin = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.TelegramID, &user.Username, &user.FirstName, &user.LastName); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserService) isSubscriptionActive(subType string, subEnd *time.Time) bool {
	if strings.TrimSpace(subType) == "" {
		return false
	}
	if subEnd == nil {
		return false
	}
	return time.Now().Before(subEnd.UTC())
}

func (s *UserService) SubscriptionDiscount(telegramID int64) float64 {
	var subType string
	var subEnd *time.Time
	if err := s.db.QueryRow(`SELECT subscription_type, subscription_end FROM users WHERE telegram_id = $1`, telegramID).Scan(&subType, &subEnd); err != nil {
		return 1.0
	}
	if !s.isSubscriptionActive(subType, subEnd) {
		return 1.0
	}
	switch strings.ToLower(subType) {
	case "mini":
		return 0.9
	case "start":
		return 0.85
	case "pro":
		return 0.8
	default:
		return 1.0
	}
}

func (s *UserService) GetSubscriptionLabel(telegramID int64) (string, bool) {
	var subType string
	var subEnd *time.Time
	if err := s.db.QueryRow(`SELECT subscription_type, subscription_end FROM users WHERE telegram_id = $1`, telegramID).Scan(&subType, &subEnd); err != nil {
		return "Free", false
	}
	active := s.isSubscriptionActive(subType, subEnd)
	if !active {
		return "Free", false
	}
	switch strings.ToLower(subType) {
	case "mini":
		return "Mini", true
	case "start":
		return "Start", true
	case "pro":
		return "Pro", true
	default:
		return "Free", false
	}
}

func (s *UserService) GetSubscriptionInfo(telegramID int64) (string, bool, *time.Time) {
	var subType string
	var subEnd *time.Time
	if err := s.db.QueryRow(`SELECT subscription_type, subscription_end FROM users WHERE telegram_id = $1`, telegramID).Scan(&subType, &subEnd); err != nil {
		return "Free", false, nil
	}
	if !s.isSubscriptionActive(subType, subEnd) {
		return "Free", false, nil
	}
	label := "Free"
	switch strings.ToLower(subType) {
	case "mini":
		label = "Mini"
	case "start":
		label = "Start"
	case "pro":
		label = "Pro"
	default:
		label = "Free"
	}
	return label, true, subEnd
}

func (s *UserService) SetSubscription(telegramID int64, subType string, days int) error {
	if err := s.ensureSubscriptionColumns(); err != nil {
		log.Printf("ensureSubscriptionColumns error: %v", err)
		return err
	}

	subType = strings.ToLower(strings.TrimSpace(subType))
	var end *time.Time
	if subType != "" && days > 0 {
		e := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
		end = &e
	}
	updateFn := func() error {
		_, err := s.db.Exec(`
			UPDATE users
			SET subscription_type = $2::text,
				subscription_started_at = CASE WHEN $2::text = '' THEN NULL ELSE NOW() END,
				subscription_end = $3,
				updated_at = CURRENT_TIMESTAMP
			WHERE telegram_id = $1
		`, telegramID, subType, end)
		return err
	}

	if err := updateFn(); err != nil {
		if err2 := s.ensureSubscriptionColumns(); err2 != nil {
			log.Printf("ensureSubscriptionColumns retry error: %v", err2)
			return err
		}
		if errRetry := updateFn(); errRetry != nil {
			log.Printf("SetSubscription update error: %v", errRetry)
			return errRetry
		}
	}
	if subType == "" {
		return s.ResetSubscription(telegramID)
	}
	_ = s.setSubscriptionQuotas(telegramID, subType)
	_ = s.ensureDailyTextQuota(telegramID)

	return nil
}

func (s *UserService) ensureSubscriptionColumns() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	alters := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_type VARCHAR(20) DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_started_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_end TIMESTAMP WITH TIME ZONE;`,
	}
	for _, q := range alters {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *UserService) resetSubscriptionTx(tx *sql.Tx, telegramID int64) error {
	if _, err := tx.Exec(`
		UPDATE users
		SET subscription_type = '',
			subscription_started_at = NULL,
			subscription_end = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE user_quotas
		SET text_daily = 10,
			image_weekly = 0,
			music_weekly = 0,
			video_weekly = 0,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID); err != nil {
		return err
	}
	return nil
}

func (s *UserService) ResetSubscription(telegramID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.resetSubscriptionTx(tx, telegramID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *UserService) setSubscriptionQuotas(telegramID int64, subType string) error {
	subType = strings.ToLower(strings.TrimSpace(subType))
	image, music, video := 0, 0, 0
	textDaily := 10
	switch subType {
	case "mini":
		image, music, video = 25, 3, 0
		textDaily = 50
	case "start":
		image, music, video = 40, 5, 2
		textDaily = 100
	case "pro":
		image, music, video = 90, 10, 4
		textDaily = 200
	default:
		return nil
	}
	_, err := s.db.Exec(`
		UPDATE user_quotas
		SET text_daily = $2,
			image_weekly = $3,
			music_weekly = $4,
			video_weekly = $5,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID, textDaily, image, music, video)
	return err
}

// subscriptionQuotaValues возвращает (image, music, video) для типа подписки.
func subscriptionQuotaValues(subType string) (image, music, video int) {
	switch strings.ToLower(strings.TrimSpace(subType)) {
	case "mini":
		return 25, 3, 0
	case "start":
		return 40, 5, 2
	case "pro":
		return 90, 10, 5
	default:
		return 0, 0, 0
	}
}

// SubscriptionQuotas возвращает (image, music, video) квоты для типа подписки.
func (s *UserService) SubscriptionQuotas(subType string) (image, music, video int) {
	return subscriptionQuotaValues(subType)
}

func (s *UserService) setSubscriptionWeekly(telegramID int64, subType string) error {
	subType = strings.ToLower(strings.TrimSpace(subType))
	image, music, video := subscriptionQuotaValues(subType)
	if image == 0 && music == 0 && video == 0 {
		return nil
	}
	_, err := s.db.Exec(`
		SET image_weekly = $2,
			music_weekly = $3,
			video_weekly = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1
	`, telegramID, image, music, video)
	return err
}

func (s *UserService) EnsureDailyQuota(telegramID int64) error {
	return s.ensureDailyTextQuota(telegramID)
}

func (s *UserService) EnsureWeeklyQuota(telegramID int64) error {
	return s.ensureWeeklyQuota(telegramID)
}

func (s *UserService) SetUserAdmin(telegramID int64, isAdmin bool) error {
	query := `UPDATE users SET is_admin = $2, updated_at = CURRENT_TIMESTAMP WHERE telegram_id = $1`
	_, err := s.db.Exec(query, telegramID, isAdmin)
	return err
}

func (s *UserService) GetAllUsers(limit, offset int) ([]*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
			   is_premium, is_admin, COALESCE(referrer_id, 0), COALESCE(referral_code, ''),
			   COALESCE(referrals_count, 0), created_at, updated_at, is_blocked
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		var referrerID int64
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
			&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
			&referrerID, &user.ReferralCode, &user.ReferralsCount,
			&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
		)
		if err != nil {
			return nil, err
		}
		if referrerID != 0 {
			user.ReferrerID = &referrerID
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (s *UserService) CountUsers() (int, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total)
	return total, err
}

// GetUsersForTrialReminder возвращает пользователей, которые:
// - зарегистрировались более часа назад
// - не получили пробную генерацию (channel_trial_claimed = false)
// - ещё не получали напоминание (trial_reminder_sent = false)
func (s *UserService) GetUsersForTrialReminder() ([]*models.User, error) {
	if err := s.ensureChannelTrialColumn(); err != nil {
		return nil, err
	}
	if err := s.ensureTrialReminderColumn(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
			   is_premium, is_admin, COALESCE(referrer_id, 0), COALESCE(referral_code, ''),
			   COALESCE(referrals_count, 0), created_at, updated_at, is_blocked
		FROM users
		WHERE channel_trial_claimed = FALSE
		  AND trial_reminder_sent = FALSE
		  AND created_at < NOW() - INTERVAL '2 hours'
		ORDER BY created_at ASC
		LIMIT 100`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		var referrerID int64
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
			&user.LanguageCode, &user.IsPremium, &user.IsAdmin,
			&referrerID, &user.ReferralCode, &user.ReferralsCount,
			&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
		)
		if err != nil {
			return nil, err
		}
		if referrerID != 0 {
			user.ReferrerID = &referrerID
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// MarkTrialReminderSent помечает, что напоминание о пробной генерации было отправлено
func (s *UserService) MarkTrialReminderSent(telegramID int64) error {
	if err := s.ensureTrialReminderColumn(); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET trial_reminder_sent = TRUE, updated_at = CURRENT_TIMESTAMP WHERE telegram_id = $1`, telegramID)
	return err
}

func (s *UserService) ensureTrialReminderColumn() error {
	_, err := s.db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS trial_reminder_sent BOOLEAN NOT NULL DEFAULT FALSE`)
	return err
}

func (s *UserService) RecordPayment(telegramID int64, username, firstName, lastName, paymentID, category string, qty int, amount float64) error {
	_, err := s.db.Exec(`
		INSERT INTO completed_payments (telegram_id, username, first_name, last_name, payment_id, category, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		telegramID, username, firstName, lastName, paymentID, category, qty, amount)
	return err
}

func (s *UserService) GetRecentPayments(limit int) ([]*models.Payment, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, telegram_id, username, first_name, last_name, payment_id, category, qty, amount, created_at
		FROM completed_payments
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(&p.ID, &p.TelegramID, &p.Username, &p.FirstName, &p.LastName, &p.PaymentID, &p.Category, &p.Qty, &p.Amount, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (s *UserService) GetPaymentStats(since time.Time) (*models.PaymentStats, error) {
	stats := &models.PaymentStats{}
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM completed_payments
		WHERE created_at >= $1`, since).Scan(&stats.Count, &stats.TotalAmount)
	return stats, err
}

func (s *UserService) GetPaymentStatsAll() (*models.PaymentStats, error) {
	stats := &models.PaymentStats{}
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM completed_payments`).Scan(&stats.Count, &stats.TotalAmount)
	return stats, err
}

func (s *UserService) GetUserStats(telegramID int64) (map[string]any, error) {
	query := `
		SELECT
			COALESCE(u.referrals_count, 0),
			COUNT(gr.id) as total_generations,
			COUNT(CASE WHEN gr.status = 'completed' THEN 1 END) as completed_generations
		FROM users u
		LEFT JOIN generation_requests gr ON u.telegram_id = gr.user_id
		WHERE u.telegram_id = $1
		GROUP BY u.referrals_count`

	var referralsCount, totalGenerations, completedGenerations int
	err := s.db.QueryRow(query, telegramID).Scan(&referralsCount, &totalGenerations, &completedGenerations)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"referrals_count":       referralsCount,
		"total_generations":     totalGenerations,
		"completed_generations": completedGenerations,
	}, nil
}
