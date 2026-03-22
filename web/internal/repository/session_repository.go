package repository

import (
	"database/sql"
	"time"

	"telegram-ai-face-bot/web/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *models.WebSession) error {
	query := `
		INSERT INTO web_sessions (user_id, token, refresh_token, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	return r.db.QueryRow(query,
		session.UserID, session.Token, session.RefreshToken,
		session.UserAgent, session.IPAddress, session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)
}

func (r *SessionRepository) GetByToken(token string) (*models.WebSession, error) {
	query := `
		SELECT id, user_id, token, refresh_token, user_agent, ip_address, expires_at, created_at
		FROM web_sessions WHERE token = $1 AND expires_at > NOW()`

	session := &models.WebSession{}
	err := r.db.QueryRow(query, token).Scan(
		&session.ID, &session.UserID, &session.Token, &session.RefreshToken,
		&session.UserAgent, &session.IPAddress, &session.ExpiresAt, &session.CreatedAt,
	)
	return session, err
}

func (r *SessionRepository) GetByRefreshToken(refreshToken string) (*models.WebSession, error) {
	query := `
		SELECT id, user_id, token, refresh_token, user_agent, ip_address, expires_at, created_at
		FROM web_sessions WHERE refresh_token = $1`

	session := &models.WebSession{}
	err := r.db.QueryRow(query, refreshToken).Scan(
		&session.ID, &session.UserID, &session.Token, &session.RefreshToken,
		&session.UserAgent, &session.IPAddress, &session.ExpiresAt, &session.CreatedAt,
	)
	return session, err
}

func (r *SessionRepository) Delete(token string) error {
	_, err := r.db.Exec(`DELETE FROM web_sessions WHERE token = $1`, token)
	return err
}

func (r *SessionRepository) DeleteByUserID(userID int64) error {
	_, err := r.db.Exec(`DELETE FROM web_sessions WHERE user_id = $1`, userID)
	return err
}

func (r *SessionRepository) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM web_sessions WHERE expires_at < NOW()`)
	return err
}

func (r *SessionRepository) UpdateToken(id int64, newToken string, newExpiry time.Time) error {
	query := `UPDATE web_sessions SET token = $2, expires_at = $3 WHERE id = $1`
	_, err := r.db.Exec(query, id, newToken, newExpiry)
	return err
}
