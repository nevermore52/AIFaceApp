package models

import (
	"time"
)

type User struct {
	ID             int64      `json:"id" db:"id"`
	TelegramID     int64      `json:"telegram_id" db:"telegram_id"`
	Username       string     `json:"username" db:"username"`
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	LanguageCode   string     `json:"language_code" db:"language_code"`
	IsPremium      bool       `json:"is_premium" db:"is_premium"`
	IsAdmin        bool       `json:"is_admin" db:"is_admin"`
	ReferrerID     *int64     `json:"referrer_id" db:"referrer_id"`
	ReferralCode   string     `json:"referral_code" db:"referral_code"`
	ReferralsCount int        `json:"referrals_count" db:"referrals_count"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	IsBlocked      bool       `json:"is_blocked" db:"is_blocked"`
	Subscription   string     `json:"subscription_type" db:"subscription_type"`
	SubStartedAt   *time.Time `json:"subscription_started_at" db:"subscription_started_at"`
	SubEnd         *time.Time `json:"subscription_end" db:"subscription_end"`
}

type UserQuota struct {
	ID          int64     `json:"id" db:"id"`
	TelegramID  int64     `json:"telegram_id" db:"telegram_id"`
	TextDaily   int       `json:"text_daily" db:"text_daily"`
	TextExtra   int       `json:"text_extra" db:"text_extra"`
	ImageWeekly int       `json:"image_weekly" db:"image_weekly"`
	ImageExtra  int       `json:"image_extra" db:"image_extra"`
	MusicWeekly int       `json:"music_weekly" db:"music_weekly"`
	MusicExtra  int       `json:"music_extra" db:"music_extra"`
	VideoWeekly int       `json:"video_weekly" db:"video_weekly"`
	VideoExtra  int       `json:"video_extra" db:"video_extra"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type QuotaCategory string

const (
	QuotaCategoryText  QuotaCategory = "text"
	QuotaCategoryImage QuotaCategory = "image"
	QuotaCategoryMusic QuotaCategory = "music"
	QuotaCategoryVideo QuotaCategory = "video"
)

type TokenTransaction struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	Amount      int       `json:"amount" db:"amount"`
	Type        string    `json:"type" db:"type"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type GenerationRequest struct {
	ID                int64      `json:"id" db:"id"`
	UserID            int64      `json:"user_id" db:"user_id"`
	Type              string     `json:"type" db:"type"`
	Status            string     `json:"status" db:"status"`
	InputImage        string     `json:"input_image" db:"input_image"`
	OutputImage       *string    `json:"output_image" db:"output_image"`
	Prompt            *string    `json:"prompt" db:"prompt"`
	ErrorMsg          *string    `json:"error_msg" db:"error_msg"`
	TokensUsed        int        `json:"tokens_used" db:"tokens_used"`
	TokensPrimaryUsed int        `json:"tokens_primary_used" db:"tokens_primary_used"`
	TokensExtraUsed   int        `json:"tokens_extra_used" db:"tokens_extra_used"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	CompletedAt       *time.Time `json:"completed_at" db:"completed_at"`
}

type CategorySetting struct {
	Category string `json:"category" db:"category"`
	Enabled  bool   `json:"enabled" db:"enabled"`
}
