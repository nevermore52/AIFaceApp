package repository

import (
	"database/sql"

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

	// Debug: check total generations in DB
	var totalAll int
	r.db.QueryRow(`SELECT COUNT(*) FROM generation_requests`).Scan(&totalAll)

	// Debug logging - always log to help diagnose
	println("GetByUserID: searching for userID=", userID, "found=", total, "total_in_db=", totalAll)

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
