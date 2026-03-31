package repository

import (
	"database/sql"
	"fmt"

	"telegram-ai-face-bot/web/internal/models"
)

type QuotaRepository struct {
	db *sql.DB
}

func NewQuotaRepository(db *sql.DB) *QuotaRepository {
	return &QuotaRepository{db: db}
}

func (r *QuotaRepository) GetByTelegramID(telegramID int64) (*models.UserQuota, error) {
	query := `
		SELECT id, telegram_id, text_daily, text_extra, image_weekly, image_extra,
			   music_weekly, music_extra, video_weekly, video_extra, created_at, updated_at
		FROM user_quotas WHERE telegram_id = $1`

	quota := &models.UserQuota{}
	err := r.db.QueryRow(query, telegramID).Scan(
		&quota.ID, &quota.TelegramID, &quota.TextDaily, &quota.TextExtra,
		&quota.ImageWeekly, &quota.ImageExtra, &quota.MusicWeekly, &quota.MusicExtra,
		&quota.VideoWeekly, &quota.VideoExtra, &quota.CreatedAt, &quota.UpdatedAt,
	)
	return quota, err
}

func (r *QuotaRepository) GetOrCreate(telegramID int64) (*models.UserQuota, error) {
	quota, err := r.GetByTelegramID(telegramID)
	if err == nil {
		return quota, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	query := `
		INSERT INTO user_quotas (telegram_id, text_daily, text_extra, image_weekly, image_extra, music_weekly, music_extra, video_weekly, video_extra)
		VALUES ($1, 10, 0, 0, 0, 0, 0, 0, 0)
		RETURNING id, telegram_id, text_daily, text_extra, image_weekly, image_extra,
				  music_weekly, music_extra, video_weekly, video_extra, created_at, updated_at`

	quota = &models.UserQuota{}
	err = r.db.QueryRow(query, telegramID).Scan(
		&quota.ID, &quota.TelegramID, &quota.TextDaily, &quota.TextExtra,
		&quota.ImageWeekly, &quota.ImageExtra, &quota.MusicWeekly, &quota.MusicExtra,
		&quota.VideoWeekly, &quota.VideoExtra, &quota.CreatedAt, &quota.UpdatedAt,
	)
	return quota, err
}

func (r *QuotaRepository) AddExtra(telegramID int64, category models.QuotaCategory, amount int) error {
	var column string
	switch category {
	case models.QuotaCategoryText:
		column = "text_extra"
	case models.QuotaCategoryImage:
		column = "image_extra"
	case models.QuotaCategoryMusic:
		column = "music_extra"
	case models.QuotaCategoryVideo:
		column = "video_extra"
	default:
		return fmt.Errorf("unknown category: %s", category)
	}

	query := fmt.Sprintf(`
		UPDATE user_quotas SET %s = %s + $2, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1`, column, column)
	_, err := r.db.Exec(query, telegramID, amount)
	return err
}

func (r *QuotaRepository) AddPrimary(telegramID int64, category models.QuotaCategory, amount int) error {
	var column string
	switch category {
	case models.QuotaCategoryText:
		column = "text_daily"
	case models.QuotaCategoryImage:
		column = "image_weekly"
	case models.QuotaCategoryMusic:
		column = "music_weekly"
	case models.QuotaCategoryVideo:
		column = "video_weekly"
	default:
		return fmt.Errorf("unknown category: %s", category)
	}

	query := fmt.Sprintf(`
		UPDATE user_quotas SET %s = %s + $2, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1`, column, column)
	_, err := r.db.Exec(query, telegramID, amount)
	return err
}

func (r *QuotaRepository) Consume(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error) {
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
		return 0, 0, fmt.Errorf("unknown category: %s", category)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var primary, extra int
	row := tx.QueryRow(fmt.Sprintf(`SELECT %s, %s FROM user_quotas WHERE telegram_id = $1 FOR UPDATE`, primaryCol, extraCol), telegramID)
	if err = row.Scan(&primary, &extra); err != nil {
		return 0, 0, err
	}

	if primary+extra < amount {
		return 0, 0, fmt.Errorf("insufficient quota: need %d, have %d", amount, primary+extra)
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

	_, err = tx.Exec(fmt.Sprintf(`
		UPDATE user_quotas SET %s = $2, %s = $3, updated_at = CURRENT_TIMESTAMP
		WHERE telegram_id = $1`, primaryCol, extraCol), telegramID, primary, extra)
	if err != nil {
		return 0, 0, err
	}

	return primaryUsed, extraUsed, tx.Commit()
}
