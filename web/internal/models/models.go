package models

import (
	"time"
)

type User struct {
	ID                    int64      `json:"id" db:"id"`
	TelegramID            *int64     `json:"telegram_id,omitempty" db:"telegram_id"`
	Email                 *string    `json:"email,omitempty" db:"email"`
	Username              string     `json:"username" db:"username"`
	FirstName             string     `json:"first_name" db:"first_name"`
	LastName              string     `json:"last_name" db:"last_name"`
	AvatarURL             *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	LanguageCode          *string    `json:"language_code,omitempty" db:"language_code"`
	IsPremium             bool       `json:"is_premium" db:"is_premium"`
	IsAdmin               bool       `json:"is_admin" db:"is_admin"`
	ReferrerID            *int64     `json:"referrer_id,omitempty" db:"referrer_id"`
	ReferralCode          string     `json:"referral_code" db:"referral_code"`
	ReferralsCount        int        `json:"referrals_count" db:"referrals_count"`
	SubscriptionType      string     `json:"subscription_type" db:"subscription_type"`
	SubscriptionStartedAt *time.Time `json:"subscription_started_at,omitempty" db:"subscription_started_at"`
	SubscriptionEnd       *time.Time `json:"subscription_end,omitempty" db:"subscription_end"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
	IsBlocked             bool       `json:"is_blocked" db:"is_blocked"`
	ChannelTrialClaimed   bool       `json:"channel_trial_claimed" db:"channel_trial_claimed"`
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

type GenerationRequest struct {
	ID                int64      `json:"id" db:"id"`
	UserID            int64      `json:"user_id" db:"user_id"`
	Username          string     `json:"username" db:"username"`
	ModelType         string     `json:"model_type" db:"model_type"`
	Model             string     `json:"model" db:"model"`
	Status            string     `json:"status" db:"status"`
	InputImage        string     `json:"input_image" db:"input_image"`
	Output            *string    `json:"output,omitempty" db:"output"`
	Prompt            *string    `json:"prompt,omitempty" db:"prompt"`
	ErrorMsg          *string    `json:"error_msg,omitempty" db:"error_msg"`
	TokensUsed        int        `json:"tokens_used" db:"tokens_used"`
	TokensPrimaryUsed int        `json:"tokens_primary_used" db:"tokens_primary_used"`
	TokensExtraUsed   int        `json:"tokens_extra_used" db:"tokens_extra_used"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

type Payment struct {
	ID         int64     `json:"id" db:"id"`
	TelegramID int64     `json:"telegram_id" db:"telegram_id"`
	Username   string    `json:"username" db:"username"`
	FirstName  string    `json:"first_name" db:"first_name"`
	LastName   string    `json:"last_name" db:"last_name"`
	PaymentID  string    `json:"payment_id" db:"payment_id"`
	Category   string    `json:"category" db:"category"`
	Qty        int       `json:"qty" db:"qty"`
	Amount     float64   `json:"amount" db:"amount"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type WebSession struct {
	ID           int64     `json:"id" db:"id"`
	UserID       int64     `json:"user_id" db:"user_id"`
	Token        string    `json:"token" db:"token"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type AuthProvider string

const (
	AuthProviderTelegram AuthProvider = "telegram"
	AuthProviderGoogle   AuthProvider = "google"
)

type UserAuth struct {
	ID         int64        `json:"id" db:"id"`
	UserID     int64        `json:"user_id" db:"user_id"`
	Provider   AuthProvider `json:"provider" db:"provider"`
	ProviderID string       `json:"provider_id" db:"provider_id"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
}

type QuotaCategory string

const (
	QuotaCategoryText  QuotaCategory = "text"
	QuotaCategoryImage QuotaCategory = "image"
	QuotaCategoryMusic QuotaCategory = "music"
	QuotaCategoryVideo QuotaCategory = "video"
)

type CategorySetting struct {
	Category string `json:"category" db:"category"`
	Enabled  bool   `json:"enabled" db:"enabled"`
}

type PricePackage struct {
	Category string `json:"category"`
	Qty      int    `json:"qty"`
	Price    int    `json:"price"`
}

type SubscriptionPlan struct {
	Name        string `json:"name"`
	Price       int    `json:"price"`
	TextDaily   int    `json:"text_daily"`
	ImageWeekly int    `json:"image_weekly"`
	MusicWeekly int    `json:"music_weekly"`
	VideoWeekly int    `json:"video_weekly"`
	Discount    int    `json:"discount"`
}

type TopUser struct {
	UserID           int64  `json:"user_id"`
	Username         string `json:"username"`
	TotalGenerations int    `json:"total_generations"`
	PhotoGenerations int    `json:"photo_generations"`
	VideoGenerations int    `json:"video_generations"`
	MusicGenerations int    `json:"music_generations"`
	TextGenerations  int    `json:"text_generations"`
	TokensSpent      int    `json:"tokens_spent"`
}
