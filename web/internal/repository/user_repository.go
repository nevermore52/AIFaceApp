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
			   created_at, updated_at, is_blocked
		FROM users WHERE id = $1`

	user := &models.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)
	return user, err
}

func (r *UserRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, email, username, first_name, last_name, avatar_url,
			   language_code, is_premium, is_admin, referrer_id, referral_code,
			   referrals_count, subscription_type, subscription_started_at, subscription_end,
			   created_at, updated_at, is_blocked
		FROM users WHERE telegram_id = $1`

	user := &models.User{}
	err := r.db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
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
				  created_at, updated_at, is_blocked`

	user := &models.User{}
	err := r.db.QueryRow(query, telegramID, username, firstName, lastName, languageCode, avatarURL, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)
	return user, err
}

func (r *UserRepository) CreateFromGoogle(email, firstName, lastName string, avatarURL *string) (*models.User, error) {
	referralCode := generateReferralCode()

	query := `
		INSERT INTO users (email, first_name, last_name, avatar_url, referral_code)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, telegram_id, email, username, first_name, last_name, avatar_url,
				  language_code, is_premium, is_admin, referrer_id, referral_code,
				  referrals_count, subscription_type, subscription_started_at, subscription_end,
				  created_at, updated_at, is_blocked`

	user := &models.User{}
	err := r.db.QueryRow(query, email, firstName, lastName, avatarURL, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.AvatarURL, &user.LanguageCode, &user.IsPremium, &user.IsAdmin,
		&user.ReferrerID, &user.ReferralCode, &user.ReferralsCount,
		&user.SubscriptionType, &user.SubscriptionStartedAt, &user.SubscriptionEnd,
		&user.CreatedAt, &user.UpdatedAt, &user.IsBlocked,
	)
	return user, err
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

func (r *UserRepository) LinkTelegramID(userID int64, telegramID int64) error {
	query := `UPDATE users SET telegram_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(query, userID, telegramID)
	return err
}

func (r *UserRepository) LinkEmail(userID int64, email string) error {
	query := `UPDATE users SET email = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(query, userID, email)
	return err
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
