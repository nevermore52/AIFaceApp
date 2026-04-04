package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"telegram-ai-face-bot/web/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func generateReferralCode() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (r *UserRepository) GetByID(id int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, email, username, first_name, last_name, avatar_url,
			   language_code, is_premium, is_admin, referrer_id, referral_code,
			   referrals_count, subscription_type, subscription_started_at, subscription_end,
			   created_at, updated_at, is_blocked, channel_trial_claimed
		FROM users WHERE id = $1`

	user := &models.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked, &user.ChannelTrialClaimed,
	)
	return user, err
}

func (r *UserRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, email, username, first_name, last_name, avatar_url,
			   language_code, is_premium, is_admin, referrer_id, referral_code,
			   referrals_count, subscription_type, subscription_started_at, subscription_end,
			   created_at, updated_at, is_blocked, channel_trial_claimed
		FROM users WHERE telegram_id = $1`

	user := &models.User{}
	err := r.db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked, &user.ChannelTrialClaimed,
	)
	return user, err
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, telegram_id, email, username, first_name, last_name, avatar_url,
			   language_code, is_premium, is_admin, referrer_id, referral_code,
			   referrals_count, subscription_type, subscription_started_at, subscription_end,
			   created_at, updated_at, is_blocked
		FROM users WHERE email = $1`

	user := &models.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)
	return user, err
}

func (r *UserRepository) CreateFromTelegram(telegramID int64, username, firstName, lastName, languageCode string, avatarURL *string) (*models.User, error) {
	referralCode := generateReferralCode()

	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, language_code, avatar_url, referral_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, telegram_id, email, username, first_name, last_name, avatar_url,
				  language_code, is_premium, is_admin, referrer_id, referral_code,
				  referrals_count, subscription_type, subscription_started_at, subscription_end,
				  created_at, updated_at, is_blocked, channel_trial_claimed`

	user := &models.User{}
	err := r.db.QueryRow(query, telegramID, username, firstName, lastName, languageCode, avatarURL, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked, &user.ChannelTrialClaimed,
	)
	return user, err
}

func (r *UserRepository) CreateFromGoogle(email, firstName, lastName string, avatarURL *string) (*models.User, error) {
	referralCode := generateReferralCode()
	username := "user_" + referralCode

	query := `
		INSERT INTO users (email, username, first_name, last_name, avatar_url, referral_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, telegram_id, email, username, first_name, last_name, avatar_url,
				  language_code, is_premium, is_admin, referrer_id, referral_code,
				  referrals_count, subscription_type, subscription_started_at, subscription_end,
				  created_at, updated_at, is_blocked, channel_trial_claimed`

	user := &models.User{}
	err := r.db.QueryRow(query, email, username, firstName, lastName, avatarURL, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked, &user.ChannelTrialClaimed,
	)
	return user, err
}

func (r *UserRepository) MarkChannelTrialClaimed(userID int64) error {
	_, err := r.db.Exec(`UPDATE users SET channel_trial_claimed = TRUE WHERE id = $1`, userID)
	return err
}

func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users SET
			username = $2, first_name = $3, last_name = $4, avatar_url = $5,
			language_code = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	_, err := r.db.Exec(query, user.ID, user.Username, user.FirstName, user.LastName,
		user.AvatarURL, user.LanguageCode)
	return err
}

func (r *UserRepository) LinkTelegramID(userID, telegramID int64, username, firstName, lastName string) error {
	// Always set username from TG (may be empty if user has no @username).
	// Only override first/last name when TG has a non-empty value.
	query := `UPDATE users SET
		telegram_id = $2,
		username    = $3,
		first_name  = CASE WHEN $4 <> '' THEN $4 ELSE first_name END,
		last_name   = CASE WHEN $5 <> '' THEN $5 ELSE last_name  END,
		updated_at  = CURRENT_TIMESTAMP
	WHERE id = $1`
	_, err := r.db.Exec(query, userID, telegramID, username, firstName, lastName)
	return err
}

func (r *UserRepository) LinkEmail(userID int64, email string) error {
	query := `UPDATE users SET email = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(query, userID, email)
	return err
}

func (r *UserRepository) MergeAccounts(keepUserID, deleteUserID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Перенести генерации хранящиеся по внутреннему ID (веб-генерации)
	_, err = tx.Exec(`UPDATE generation_requests SET user_id = $1 WHERE user_id = $2`, keepUserID, deleteUserID)
	if err != nil {
		return err
	}

	// Перенести генерации хранящиеся по telegram_id (бот-генерации),
	// чтобы они не удалились через CASCADE при удалении аккаунта
	_, err = tx.Exec(`
		UPDATE generation_requests SET user_id = $1
		WHERE user_id = (SELECT telegram_id FROM users WHERE id = $2 AND telegram_id IS NOT NULL)
	`, keepUserID, deleteUserID)
	if err != nil {
		return err
	}

	// Перенести подписку если у удаляемого аккаунта она лучше/активнее
	_, err = tx.Exec(`
		UPDATE users AS k
		SET subscription_type = d.subscription_type,
		    subscription_end = d.subscription_end,
		    subscription_started_at = d.subscription_started_at
		FROM users AS d
		WHERE k.id = $1 AND d.id = $2
		  AND d.subscription_end > NOW()
		  AND (k.subscription_end IS NULL OR k.subscription_end <= NOW())
	`, keepUserID, deleteUserID)
	if err != nil {
		return err
	}

	// Перенести квоты: суммируем extra-квоты удаляемого аккаунта к keepUser
	// Ищем по внутреннему ID и по telegram_id удаляемого аккаунта
	_, _ = tx.Exec(`
		INSERT INTO user_quotas (telegram_id, text_extra, image_extra, music_extra, video_extra)
		SELECT $1,
		       COALESCE(src.text_extra, 0),
		       COALESCE(src.image_extra, 0),
		       COALESCE(src.music_extra, 0),
		       COALESCE(src.video_extra, 0)
		FROM user_quotas src
		WHERE src.telegram_id = $2
		   OR src.telegram_id = (SELECT telegram_id FROM users WHERE id = $2 AND telegram_id IS NOT NULL)
		LIMIT 1
		ON CONFLICT (telegram_id) DO UPDATE SET
		    text_extra  = user_quotas.text_extra  + EXCLUDED.text_extra,
		    image_extra = user_quotas.image_extra + EXCLUDED.image_extra,
		    music_extra = user_quotas.music_extra + EXCLUDED.music_extra,
		    video_extra = user_quotas.video_extra + EXCLUDED.video_extra,
		    updated_at  = CURRENT_TIMESTAMP
	`, keepUserID, deleteUserID)

	// Обнулить referrer_id у юзеров которые были приглашены удаляемым аккаунтом
	// (они снова привяжутся через telegram_id после merge)
	_, _ = tx.Exec(`
		UPDATE users SET referrer_id = NULL
		WHERE referrer_id = (SELECT telegram_id FROM users WHERE id = $1)
	`, deleteUserID)

	// Удалить старый аккаунт
	_, err = tx.Exec(`DELETE FROM users WHERE id = $1`, deleteUserID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepository) IsAdmin(userID int64) (bool, error) {
	var isAdmin bool
	err := r.db.QueryRow(`SELECT is_admin FROM users WHERE id = $1`, userID).Scan(&isAdmin)
	return isAdmin, err
}

func (r *UserRepository) GetAll(limit, offset int) ([]*models.User, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, telegram_id, email, username, first_name, last_name, avatar_url,
			   language_code, is_premium, is_admin, referrer_id, referral_code,
			   referrals_count, subscription_type, subscription_started_at, subscription_end,
			   created_at, updated_at, is_blocked
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
			&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
			&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
			&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
			&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

func (r *UserRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalUsers int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	stats["total_users"] = totalUsers

	var activeSubscriptions int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE subscription_end > NOW()`).Scan(&activeSubscriptions); err != nil {
		return nil, fmt.Errorf("count subscriptions: %w", err)
	}
	stats["active_subscriptions"] = activeSubscriptions

	var todayUsers int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '24 hours'`).Scan(&todayUsers); err != nil {
		return nil, fmt.Errorf("count today users: %w", err)
	}
	stats["today_users"] = todayUsers

	return stats, nil
}
