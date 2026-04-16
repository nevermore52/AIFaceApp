package repository

import (
	"database/sql"
	"strings"
	"time"
)

type PromoCode struct {
	ID               int64      `json:"id"`
	Code             string     `json:"code"`
	Description      string     `json:"description"`
	ImageTokens      int        `json:"image_tokens"`
	VideoTokens      int        `json:"video_tokens"`
	TextTokens       int        `json:"text_tokens"`
	MusicTokens      int        `json:"music_tokens"`
	MaxActivations   *int       `json:"max_activations"` // nil = unlimited
	ActivationsCount int        `json:"activations_count"`
	ExpiresAt        *time.Time `json:"expires_at"` // nil = no expiry
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type PromoRepository struct {
	db *sql.DB
}

func NewPromoRepository(db *sql.DB) *PromoRepository {
	return &PromoRepository{db: db}
}

func (r *PromoRepository) GetAll(limit, offset int) ([]*PromoCode, int, error) {
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM promo_codes`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`
		SELECT id, code, description, image_tokens, video_tokens, text_tokens, music_tokens,
		       max_activations, activations_count, expires_at, is_active, created_at, updated_at
		FROM promo_codes ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*PromoCode
	for rows.Next() {
		p := &PromoCode{}
		if err := rows.Scan(&p.ID, &p.Code, &p.Description,
			&p.ImageTokens, &p.VideoTokens, &p.TextTokens, &p.MusicTokens,
			&p.MaxActivations, &p.ActivationsCount, &p.ExpiresAt,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		list = append(list, p)
	}
	return list, total, rows.Err()
}

func (r *PromoRepository) GetByCode(code string) (*PromoCode, error) {
	p := &PromoCode{}
	err := r.db.QueryRow(`
		SELECT id, code, description, image_tokens, video_tokens, text_tokens, music_tokens,
		       max_activations, activations_count, expires_at, is_active, created_at, updated_at
		FROM promo_codes WHERE UPPER(code) = UPPER($1)
	`, code).Scan(&p.ID, &p.Code, &p.Description,
		&p.ImageTokens, &p.VideoTokens, &p.TextTokens, &p.MusicTokens,
		&p.MaxActivations, &p.ActivationsCount, &p.ExpiresAt,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *PromoRepository) Create(p *PromoCode) (*PromoCode, error) {
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	out := &PromoCode{}
	err := r.db.QueryRow(`
		INSERT INTO promo_codes (code, description, image_tokens, video_tokens, text_tokens, music_tokens,
		                         max_activations, expires_at, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		RETURNING id, code, description, image_tokens, video_tokens, text_tokens, music_tokens,
		          max_activations, activations_count, expires_at, is_active, created_at, updated_at
	`, p.Code, p.Description, p.ImageTokens, p.VideoTokens, p.TextTokens, p.MusicTokens,
		p.MaxActivations, p.ExpiresAt, p.IsActive).Scan(
		&out.ID, &out.Code, &out.Description,
		&out.ImageTokens, &out.VideoTokens, &out.TextTokens, &out.MusicTokens,
		&out.MaxActivations, &out.ActivationsCount, &out.ExpiresAt,
		&out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *PromoRepository) Update(p *PromoCode) error {
	_, err := r.db.Exec(`
		UPDATE promo_codes SET
			description=$2, image_tokens=$3, video_tokens=$4, text_tokens=$5, music_tokens=$6,
			max_activations=$7, expires_at=$8, is_active=$9, updated_at=NOW()
		WHERE id=$1
	`, p.ID, p.Description, p.ImageTokens, p.VideoTokens, p.TextTokens, p.MusicTokens,
		p.MaxActivations, p.ExpiresAt, p.IsActive)
	return err
}

func (r *PromoRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM promo_codes WHERE id=$1`, id)
	return err
}

func (r *PromoRepository) HasUserActivated(promoID int64, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM promo_activations WHERE promo_code_id=$1 AND user_id=$2)
	`, promoID, userID).Scan(&exists)
	return exists, err
}

// Activate creates a promo_activations record and increments activations_count atomically.
func (r *PromoRepository) Activate(promoID int64, userID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO promo_activations (promo_code_id, user_id) VALUES ($1,$2)
	`, promoID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE promo_codes SET activations_count = activations_count + 1, updated_at=NOW() WHERE id=$1
	`, promoID); err != nil {
		return err
	}
	return tx.Commit()
}
