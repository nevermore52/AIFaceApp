package repository

import (
	"database/sql"
	"time"

	"telegram-ai-face-bot/web/internal/models"
)

type GenerationRepository struct {
	db *sql.DB
}

func NewGenerationRepository(db *sql.DB) *GenerationRepository {
	return &GenerationRepository{db: db}
}

func (r *GenerationRepository) GetByID(id int64) (*models.GenerationRequest, error) {
	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output,
			   prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used,
			   created_at, completed_at
		FROM generation_requests WHERE id = $1`

	req := &models.GenerationRequest{}
	err := r.db.QueryRow(query, id).Scan(
		&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status,
		&req.InputImage, &req.Output, &req.Prompt, &req.ErrorMsg,
		&req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed,
		&req.CreatedAt, &req.CompletedAt,
	)
	return req, err
}

func (r *GenerationRepository) GetByUserID(userID int64, limit, offset int) ([]*models.GenerationRequest, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM generation_requests WHERE user_id = $1`
	err := r.db.QueryRow(countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output,
			   prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used,
			   created_at, completed_at
		FROM generation_requests WHERE user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*models.GenerationRequest
	for rows.Next() {
		req := &models.GenerationRequest{}
		if err := rows.Scan(
			&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status,
			&req.InputImage, &req.Output, &req.Prompt, &req.ErrorMsg,
			&req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed,
			&req.CreatedAt, &req.CompletedAt,
		); err != nil {
			return nil, 0, err
		}
		requests = append(requests, req)
	}
	return requests, total, rows.Err()
}

func (r *GenerationRepository) GetAll(limit, offset int) ([]*models.GenerationRequest, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM generation_requests`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output,
			   prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used,
			   created_at, completed_at
		FROM generation_requests ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*models.GenerationRequest
	for rows.Next() {
		req := &models.GenerationRequest{}
		if err := rows.Scan(
			&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status,
			&req.InputImage, &req.Output, &req.Prompt, &req.ErrorMsg,
			&req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed,
			&req.CreatedAt, &req.CompletedAt,
		); err != nil {
			return nil, 0, err
		}
		requests = append(requests, req)
	}
	return requests, total, rows.Err()
}

func (r *GenerationRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM generation_requests`).Scan(&total); err != nil {
		return nil, err
	}
	stats["total_generations"] = total

	var completed int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM generation_requests WHERE status = 'completed'`).Scan(&completed); err != nil {
		return nil, err
	}
	stats["completed_generations"] = completed

	var today int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM generation_requests WHERE created_at > NOW() - INTERVAL '24 hours'`).Scan(&today); err != nil {
		return nil, err
	}
	stats["today_generations"] = today

	return stats, nil
}

// GetStatsSince returns detailed stats since the given time. Pass zero time for all-time.
func (r *GenerationRepository) GetStatsSince(since time.Time) (map[string]interface{}, error) {
	var where string
	var args []interface{}
	if !since.IsZero() {
		where = "WHERE created_at >= $1"
		args = append(args, since)
	}

	query := `
		SELECT
			COUNT(*),
			COUNT(CASE WHEN status = 'completed' THEN 1 END),
			COUNT(CASE WHEN status = 'failed' THEN 1 END),
			COUNT(CASE WHEN status IN ('pending', 'processing') THEN 1 END),
			COALESCE(ROUND(COUNT(CASE WHEN status = 'completed' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 1), 0),
			COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) FILTER (WHERE status = 'completed' AND completed_at IS NOT NULL), 0)
		FROM generation_requests ` + where

	var total, completed, failed, processing int
	var successRate, avgTime float64
	if err := r.db.QueryRow(query, args...).Scan(&total, &completed, &failed, &processing, &successRate, &avgTime); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests":              total,
		"completed_requests":          completed,
		"failed_requests":             failed,
		"processing_requests":         processing,
		"success_rate":                successRate,
		"avg_processing_time_seconds": avgTime,
	}, nil
}

func (r *GenerationRepository) GetTopUsers(limit int) ([]*models.TopUser, error) {
	query := `
		SELECT
			user_id,
			COALESCE(MAX(username), '') as username,
			COUNT(*) as total_generations,
			SUM(CASE WHEN model_type = 'image' THEN 1 ELSE 0 END) as photo_generations,
			SUM(CASE WHEN model_type = 'video' THEN 1 ELSE 0 END) as video_generations,
			SUM(CASE WHEN model_type = 'music' THEN 1 ELSE 0 END) as music_generations,
			SUM(CASE WHEN model_type = 'text' THEN 1 ELSE 0 END) as text_generations,
			COALESCE(SUM(tokens_used), 0) as tokens_spent
		FROM generation_requests
		WHERE created_at > NOW() - INTERVAL '24 hours'
		GROUP BY user_id
		ORDER BY total_generations DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.TopUser
	for rows.Next() {
		u := &models.TopUser{}
		if err := rows.Scan(&u.UserID, &u.Username, &u.TotalGenerations, &u.PhotoGenerations, &u.VideoGenerations, &u.MusicGenerations, &u.TextGenerations, &u.TokensSpent); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Gallery Ideas Management

type GalleryIdea struct {
	ID        int64     `json:"id"`
	Model     string    `json:"model"`
	Output    string    `json:"output"`
	Prompt    string    `json:"prompt"`
	Priority  *int      `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *GenerationRepository) CreateGalleryIdea(model, output, prompt string, priority *int) (*GalleryIdea, error) {
	idea := &GalleryIdea{}
	err := r.db.QueryRow(`
		INSERT INTO gallery_ideas (model, output, prompt, priority, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, model, output, prompt, priority, created_at, updated_at
	`, model, output, prompt, priority).Scan(&idea.ID, &idea.Model, &idea.Output, &idea.Prompt, &idea.Priority, &idea.CreatedAt, &idea.UpdatedAt)
	return idea, err
}

func (r *GenerationRepository) GetGalleryIdeas(limit, offset int) ([]*GalleryIdea, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM gallery_ideas`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`
		SELECT id, model, output, prompt, priority, created_at, updated_at
		FROM gallery_ideas
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ideas []*GalleryIdea
	for rows.Next() {
		idea := &GalleryIdea{}
		if err := rows.Scan(&idea.ID, &idea.Model, &idea.Output, &idea.Prompt, &idea.Priority, &idea.CreatedAt, &idea.UpdatedAt); err != nil {
			continue
		}
		ideas = append(ideas, idea)
	}
	return ideas, total, rows.Err()
}

// GetGalleryIdeasSorted returns ideas sorted by mode:
//   "all"  — priority ASC NULLS LAST, then created_at DESC
//   "new"  — created_at DESC (default)
func (r *GenerationRepository) GetGalleryIdeasSorted(sort string, limit, offset int) ([]*GalleryIdea, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM gallery_ideas`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	orderClause := `created_at DESC`
	if sort == "all" {
		orderClause = `priority ASC NULLS LAST, created_at DESC`
	}

	rows, err := r.db.Query(`
		SELECT id, model, output, prompt, priority, created_at, updated_at
		FROM gallery_ideas
		ORDER BY `+orderClause+`
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ideas []*GalleryIdea
	for rows.Next() {
		idea := &GalleryIdea{}
		if err := rows.Scan(&idea.ID, &idea.Model, &idea.Output, &idea.Prompt, &idea.Priority, &idea.CreatedAt, &idea.UpdatedAt); err != nil {
			continue
		}
		ideas = append(ideas, idea)
	}
	return ideas, total, rows.Err()
}

// GetOccupiedPriorities returns the list of priority slots already taken.
func (r *GenerationRepository) GetOccupiedPriorities(excludeID int64) ([]int, error) {
	var rows *sql.Rows
	var err error
	if excludeID > 0 {
		rows, err = r.db.Query(`SELECT priority FROM gallery_ideas WHERE priority IS NOT NULL AND id <> $1 ORDER BY priority`, excludeID)
	} else {
		rows, err = r.db.Query(`SELECT priority FROM gallery_ideas WHERE priority IS NOT NULL ORDER BY priority`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []int
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err == nil {
			result = append(result, p)
		}
	}
	return result, rows.Err()
}

func (r *GenerationRepository) UpdateGalleryIdea(id int64, model, output, prompt string, priority *int) error {
	_, err := r.db.Exec(`
		UPDATE gallery_ideas
		SET model = $1, output = $2, prompt = $3, priority = $4, updated_at = NOW()
		WHERE id = $5
	`, model, output, prompt, priority, id)
	return err
}

func (r *GenerationRepository) DeleteGalleryIdea(id int64) error {
	_, err := r.db.Exec(`DELETE FROM gallery_ideas WHERE id = $1`, id)
	return err
}
