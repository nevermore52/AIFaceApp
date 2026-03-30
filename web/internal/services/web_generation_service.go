package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"telegram-ai-face-bot/web/internal/kieapi"
)

type WebGenerationService struct {
	db          *sql.DB
	kieClient   *kieapi.Client
	callbackURL string
}

func NewWebGenerationService(db *sql.DB, kieClient *kieapi.Client, callbackURL string) *WebGenerationService {
	return &WebGenerationService{
		db:          db,
		kieClient:   kieClient,
		callbackURL: callbackURL,
	}
}

type GenerationRequest struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Model          string     `json:"model"`
	ModelType      string     `json:"model_type"`
	Prompt         string     `json:"prompt"`
	InputImage     string     `json:"input_image,omitempty"`
	Status         string     `json:"status"`
	Output         *string    `json:"output,omitempty"`
	ErrorMsg       *string    `json:"error_msg,omitempty"`
	ExternalTaskID string     `json:"external_task_id,omitempty"`
	TokensUsed     int        `json:"tokens_used"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type CreateGenerationRequest struct {
	Model       string   `json:"model" binding:"required"`
	Prompt      string   `json:"prompt" binding:"required"`
	ImageURLs   []string `json:"image_urls"`
	AspectRatio string   `json:"aspect_ratio"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	TokenCost   int    `json:"token_cost"`
}

func (s *WebGenerationService) GetAvailableModels() []ModelInfo {
	return []ModelInfo{
		{ID: "google/nano-banana", Name: "Nano Banana", Type: "image", Description: "Генерация изображений", TokenCost: 1},
		{ID: "google/nano-banana-pro", Name: "Nano Banana Pro", Type: "image", Description: "Продвинутая генерация изображений", TokenCost: 4},
		{ID: "nano-banana-2", Name: "Nano Banana 2", Type: "image", Description: "Генерация и редактирование изображений", TokenCost: 2},
		{ID: "seedream/4.5-edit", Name: "Seedream 4.5", Type: "image", Description: "Редактирование изображений", TokenCost: 3},
		{ID: "veo3_fast", Name: "Veo 3.1 Fast", Type: "video", Description: "Генерация видео", TokenCost: 1},
		{ID: "wan/2-6-image-to-video", Name: "Wan 2.6", Type: "video", Description: "Генерация видео из изображения", TokenCost: 2},
		{ID: "kling-2.6/image-to-video", Name: "Kling 2.6", Type: "video", Description: "Генерация видео с звуком", TokenCost: 1},
		{ID: "music-suno", Name: "Suno Music", Type: "music", Description: "Генерация музыки", TokenCost: 1},
		{ID: "google/gemini-3-flash", Name: "Gemini 3 Flash", Type: "text", Description: "Текстовая модель", TokenCost: 1},
		{ID: "openai/gpt-5-mini", Name: "GPT-5 mini", Type: "text", Description: "Текстовая модель", TokenCost: 1},
		{ID: "openai/gpt-5-nano", Name: "GPT-5 nano", Type: "text", Description: "Текстовая модель", TokenCost: 1},
		{ID: "chat-gpt-4.1mini", Name: "GPT-4.1 mini", Type: "text", Description: "Текстовая модель", TokenCost: 1},
	}
}

func (s *WebGenerationService) CreateGeneration(userID int64, req CreateGenerationRequest) (*GenerationRequest, error) {
	if s.kieClient == nil {
		return nil, fmt.Errorf("generation service not configured")
	}
	if s.callbackURL == "" {
		return nil, fmt.Errorf("callback URL not configured")
	}

	modelType := "image"
	if strings.Contains(req.Model, "video") {
		modelType = "video"
	}

	inputImage := ""
	if len(req.ImageURLs) > 0 {
		inputImage = strings.Join(req.ImageURLs, ",")
	}

	query := `
		INSERT INTO generation_requests (user_id, model_type, model, status, input_image, prompt, tokens_used)
		VALUES ($1, $2, $3, 'pending', $4, $5, 1)
		RETURNING id, user_id, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, created_at, completed_at`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, userID, modelType, req.Model, inputImage, req.Prompt).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generation request: %w", err)
	}

	go s.processGeneration(genReq, req)

	return genReq, nil
}

func (s *WebGenerationService) processGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	input := map[string]any{
		"prompt": req.Prompt,
	}

	if req.AspectRatio != "" {
		input["aspect_ratio"] = req.AspectRatio
	}

	model := strings.ToLower(strings.TrimSpace(req.Model))

	if model == "nano-banana-2" || model == "nano-banana-pro" {
		if len(req.ImageURLs) > 0 {
			input["image_input"] = req.ImageURLs
		} else {
			input["image_input"] = []string{}
		}
		input["resolution"] = "1K"
	} else {
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = req.ImageURLs
		}
	}

	if model == "seedream/4.5-edit" {
		input["quality"] = "basic"
	}

	taskReq := kieapi.CreateTaskRequest{
		Model:       req.Model,
		CallBackURL: s.callbackURL,
		Input:       input,
	}

	taskID, err := s.kieClient.CreateTask(taskReq)
	if err != nil {
		log.Printf("KieAPI task creation failed for request %d: %v", genReq.ID, err)
		_ = s.updateStatus(genReq.ID, "failed", err.Error())
		return
	}

	_ = s.updateExternalTaskID(genReq.ID, taskID)
	log.Printf("KieAPI task created: requestID=%d taskID=%s", genReq.ID, taskID)
}

func (s *WebGenerationService) HandleCallback(payload kieapi.CallbackPayload) error {
	taskID := payload.TaskIDValue()
	if taskID == "" {
		return fmt.Errorf("callback missing taskId")
	}

	genReq, err := s.getByExternalTaskID(taskID)
	if err != nil {
		return fmt.Errorf("generation request not found for taskId %s: %w", taskID, err)
	}

	status := strings.ToLower(payload.StatusValue())
	if status != "success" && status != "completed" && status != "succeeded" {
		reason := payload.Msg
		if reason == "" {
			reason = fmt.Sprintf("status=%s", payload.StatusValue())
		}
		return s.updateStatus(genReq.ID, "failed", reason)
	}

	resultURL := payload.ResultURL()
	if resultURL == "" {
		return s.updateStatus(genReq.ID, "failed", "callback missing result URL")
	}

	return s.completeRequest(genReq.ID, resultURL)
}

func (s *WebGenerationService) GetByID(id int64) (*GenerationRequest, error) {
	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, created_at, completed_at
		FROM generation_requests WHERE id = $1`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, id).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return genReq, nil
}

func (s *WebGenerationService) getByExternalTaskID(taskID string) (*GenerationRequest, error) {
	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, created_at, completed_at
		FROM generation_requests WHERE external_task_id = $1`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, taskID).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return genReq, nil
}

func (s *WebGenerationService) updateStatus(id int64, status, errorMsg string) error {
	var query string
	if errorMsg != "" {
		query = `UPDATE generation_requests SET status = $2, error_msg = $3, completed_at = NOW() WHERE id = $1`
		_, err := s.db.Exec(query, id, status, errorMsg)
		return err
	}
	query = `UPDATE generation_requests SET status = $2 WHERE id = $1`
	_, err := s.db.Exec(query, id, status)
	return err
}

func (s *WebGenerationService) updateExternalTaskID(id int64, taskID string) error {
	_, err := s.db.Exec(`UPDATE generation_requests SET external_task_id = $2 WHERE id = $1`, id, taskID)
	return err
}

func (s *WebGenerationService) completeRequest(id int64, resultURL string) error {
	_, err := s.db.Exec(`UPDATE generation_requests SET status = 'completed', output = $2, completed_at = NOW() WHERE id = $1`, id, resultURL)
	return err
}
