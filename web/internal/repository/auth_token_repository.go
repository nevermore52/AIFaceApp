package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type WebAuthToken struct {
	ID           int64      `json:"id"`
	Token        string     `json:"token"`
	TelegramID   *int64     `json:"telegram_id"`
	Status       string     `json:"status"` // pending, confirmed, expired
	AccessToken  *string    `json:"access_token"`
	RefreshToken *string    `json:"refresh_token"`
	CreatedAt    time.Time  `json:"created_at"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
}

type AuthTokenRepository struct {
	db *sql.DB
}

func NewAuthTokenRepository(db *sql.DB) *AuthTokenRepository {
	return &AuthTokenRepository{db: db}
}

func generateAuthToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (r *AuthTokenRepository) Create() (*WebAuthToken, error) {
	token := generateAuthToken()

	query := `
		INSERT INTO web_auth_tokens (token, status)
		VALUES ($1, 'pending')
		RETURNING id, token, telegram_id, status, access_token, refresh_token, created_at, confirmed_at`

	t := &WebAuthToken{}
	err := r.db.QueryRow(query, token).Scan(
		&t.ID, &t.Token, &t.TelegramID, &t.Status,
		&t.AccessToken, &t.RefreshToken, &t.CreatedAt, &t.ConfirmedAt,
	)
	return t, err
}

func (r *AuthTokenRepository) GetByToken(token string) (*WebAuthToken, error) {
	query := `
		SELECT id, token, telegram_id, status, access_token, refresh_token, created_at, confirmed_at
		FROM web_auth_tokens WHERE token = $1`

	t := &WebAuthToken{}
	err := r.db.QueryRow(query, token).Scan(
		&t.ID, &t.Token, &t.TelegramID, &t.Status,
		&t.AccessToken, &t.RefreshToken, &t.CreatedAt, &t.ConfirmedAt,
	)
	return t, err
}

func (r *AuthTokenRepository) Confirm(token string, telegramID int64, accessToken, refreshToken string) error {
	query := `
		UPDATE web_auth_tokens
		SET status = 'confirmed', telegram_id = $2, access_token = $3, refresh_token = $4, confirmed_at = NOW()
		WHERE token = $1 AND status = 'pending'`

	result, err := r.db.Exec(query, token, telegramID, accessToken, refreshToken)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("token not found or already confirmed")
	}
	return nil
}

func (r *AuthTokenRepository) IsExpired(t *WebAuthToken) bool {
	return time.Since(t.CreatedAt) > 5*time.Minute
}

func (r *AuthTokenRepository) CleanupExpired() error {
	_, err := r.db.Exec(`DELETE FROM web_auth_tokens WHERE created_at < NOW() - INTERVAL '10 minutes'`)
	return err
}
