package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"telegram-ai-face-bot/web/internal/kieapi"
)

type WebGenerationService struct {
	db          *sql.DB
	kieClient   *kieapi.Client
	callbackURL string
	httpClient  *http.Client
	defAPI      DefAPIClient
}

type DefAPIClient interface {
	CreateChatCompletion(model string, messages []map[string]string) (string, error)
}

func NewWebGenerationService(db *sql.DB, kieClient *kieapi.Client, callbackURL string, defAPIClient DefAPIClient) *WebGenerationService {
	return &WebGenerationService{
		db:          db,
		kieClient:   kieClient,
		callbackURL: callbackURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		defAPI:      defAPIClient,
	}
}

type GenerationRequest struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Username       string     `json:"username"`
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
	// Nano Banana 2/Pro параметры
	Resolution   string `json:"resolution"`
	GoogleSearch string `json:"google_search"`
	// Видео параметры (Wan, Kling)
	Duration string `json:"duration"`
	Sound    string `json:"sound"`
	// Suno Music параметры
	Instrumental bool   `json:"instrumental"`
	VocalGender  string `json:"vocal_gender"`
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

func (s *WebGenerationService) CreateGeneration(userID int64, username string, req CreateGenerationRequest) (*GenerationRequest, error) {
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
		INSERT INTO generation_requests (user_id, username, model_type, model, status, input_image, prompt, tokens_used)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, 1)
		RETURNING id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, created_at, completed_at`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, userID, username, modelType, req.Model, inputImage, req.Prompt).Scan(
		&genReq.ID, &genReq.UserID, &genReq.Username, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generation request: %w", err)
	}

	log.Printf("Generation created: id=%d userID=%d username=%s model=%s status=%s", genReq.ID, genReq.UserID, username, genReq.Model, genReq.Status)

	go s.processGeneration(genReq, req)

	return genReq, nil
}

func (s *WebGenerationService) processGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	model := strings.ToLower(strings.TrimSpace(req.Model))

	// Музыка через Suno API (не KieAPI)
	if model == "music-suno" || strings.Contains(model, "suno") {
		s.processMusicGeneration(genReq, req)
		return
	}

	// Текстовые модели через chat API (не KieAPI)
	if isTextModel(model) {
		s.processTextGeneration(genReq, req)
		return
	}

	// Остальные модели (фото, видео) через KieAPI
	input := map[string]any{
		"prompt": req.Prompt,
	}

	if req.AspectRatio != "" {
		input["aspect_ratio"] = req.AspectRatio
	}

	// Nano Banana 2: разрешение и Google поиск
	if model == "nano-banana-2" {
		if len(req.ImageURLs) > 0 {
			input["image_input"] = req.ImageURLs
		} else {
			input["image_input"] = []string{}
		}
		// Разрешение: 1K, 2K, 4K
		resolution := req.Resolution
		if resolution == "" {
			resolution = "1K"
		}
		input["resolution"] = resolution
		// Google поиск
		if req.GoogleSearch == "true" {
			input["google_search"] = true
		}
	} else if model == "google/nano-banana-pro" || model == "nano-banana-pro" {
		// Nano Banana Pro: разрешение 2K или 5K
		if len(req.ImageURLs) > 0 {
			input["image_input"] = req.ImageURLs
		} else {
			input["image_input"] = []string{}
		}
		resolution := req.Resolution
		if resolution == "" {
			resolution = "2K"
		}
		input["resolution"] = resolution
	} else if model == "wan/2-6-image-to-video" {
		// Wan 2.6: длительность и разрешение 1080p
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = req.ImageURLs
		}
		duration := req.Duration
		if duration == "" {
			duration = "5"
		}
		input["duration"] = duration
		input["resolution"] = "1080p"
	} else if model == "kling-2.6/image-to-video" {
		// Kling 2.6: длительность и звук
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = req.ImageURLs
		}
		duration := req.Duration
		if duration == "" {
			duration = "5"
		}
		input["duration"] = duration
		if req.Sound == "true" {
			input["sound"] = true
		}
	} else if model == "veo3_fast" {
		// Veo 3.1 Fast: формат видео
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = req.ImageURLs
		}
	} else {
		// Остальные модели
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
	log.Printf("KieAPI task created: requestID=%d taskID=%s model=%s", genReq.ID, taskID, req.Model)
}

func isTextModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "gpt") ||
		strings.Contains(model, "gemini") ||
		strings.Contains(model, "chat") ||
		model == "google/gemini-3-flash" ||
		model == "openai/gpt-5-mini" ||
		model == "openai/gpt-5-nano" ||
		model == "chat-gpt-4.1mini"
}

func (s *WebGenerationService) processMusicGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	audioURL, taskID, err := s.generateMusicSuno(req.Prompt, req.VocalGender, req.Instrumental)
	if err != nil {
		log.Printf("Suno generation failed: %v", err)
		_ = s.updateStatus(genReq.ID, "failed", fmt.Sprintf("Music generation failed: %v", err))
		return
	}

	// Если есть готовый audioURL, сразу завершаем
	if audioURL != "" {
		_ = s.completeGeneration(genReq.ID, audioURL)
		return
	}

	// Если только taskID, сохраняем его для обработки callback
	if taskID != "" {
		_ = s.updateExternalTaskID(genReq.ID, taskID)
		log.Printf("Music generation started with taskID: %s", taskID)
		return
	}

	_ = s.updateStatus(genReq.ID, "failed", "No audio URL or task ID returned from Suno API")
}

func (s *WebGenerationService) processTextGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	// Формируем сообщения для chat API
	messages := []map[string]string{
		{"role": "user", "content": req.Prompt},
	}

	response, err := s.generateChat(req.Model, messages)
	if err != nil {
		log.Printf("Chat generation failed: %v", err)
		_ = s.updateStatus(genReq.ID, "failed", fmt.Sprintf("Text generation failed: %v", err))
		return
	}

	_ = s.completeGeneration(genReq.ID, response)
}

func (s *WebGenerationService) completeGeneration(id int64, output string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE generation_requests
		SET status = 'completed', output = $1, completed_at = $2
		WHERE id = $3
	`, output, now, id)
	return err
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

// generateMusicSuno генерирует музыку через Suno API
func (s *WebGenerationService) generateMusicSuno(prompt, vocalGender string, instrumental bool) (string, string, error) {
	apiKey := os.Getenv("SUNO_API_KEY")
	if apiKey == "" {
		return "", "", fmt.Errorf("SUNO_API_KEY is not set")
	}
	callbackURL := os.Getenv("SUNO_CALLBACK_URL")
	if callbackURL == "" {
		return "", "", fmt.Errorf("SUNO_CALLBACK_URL is not set")
	}

	gender := strings.ToLower(vocalGender)
	if gender != "f" {
		gender = "m"
	}

	payload := map[string]any{
		"customMode":          false,
		"instrumental":        instrumental,
		"model":               "V5",
		"callBackUrl":         callbackURL,
		"prompt":              prompt,
		"vocalGender":         gender,
		"styleWeight":         0.65,
		"weirdnessConstraint": 0.65,
		"audioWeight":         0.75,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal suno payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.sunoapi.org/api/v1/generate", strings.NewReader(string(body)))
	if err != nil {
		return "", "", fmt.Errorf("create suno request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("suno request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("suno api error: %s", string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read suno response: %w", err)
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", "", fmt.Errorf("parse suno response: %w, raw=%s", err, string(raw))
	}

	taskID := findStringByKeys(generic, "taskId", "task_id")
	audio := findStringByKeys(
		generic,
		"audio_url", "audioUrl", "audio",
		"audioUrlHigh", "audio_url_high", "audio_high", "audio_high_url",
		"download_url", "downloadUrl",
		"result_url", "resultUrl",
		"streaming_url", "streamingUrl",
		"preview_url", "previewUrl",
		"url",
	)
	if audio == "" {
		audio = findFirstURL(generic)
	}

	if audio != "" {
		return audio, taskID, nil
	}
	if taskID != "" {
		return "", taskID, nil
	}

	return "", "", fmt.Errorf("suno response missing audio url and taskId: %s", string(raw))
}

// generateChat генерирует текст через DefAPI (используя готовую систему из Telegram бота)
func (s *WebGenerationService) generateChat(model string, messages []map[string]string) (string, error) {
	if s.defAPI == nil {
		return "", fmt.Errorf("defapi client is not configured")
	}
	return s.defAPI.CreateChatCompletion(model, messages)
}

// Вспомогательные функции для парсинга ответов
func findStringByKeys(v any, keys ...string) string {
	switch val := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if s, ok2 := raw.(string); ok2 && s != "" {
					return s
				}
			}
		}
		for _, vv := range val {
			if res := findStringByKeys(vv, keys...); res != "" {
				return res
			}
		}
	case []any:
		for _, vv := range val {
			if res := findStringByKeys(vv, keys...); res != "" {
				return res
			}
		}
	}
	return ""
}

func findFirstURL(v any) string {
	switch val := v.(type) {
	case map[string]any:
		for _, vv := range val {
			if u := findFirstURL(vv); u != "" {
				return u
			}
		}
	case []any:
		for _, vv := range val {
			if u := findFirstURL(vv); u != "" {
				return u
			}
		}
	case string:
		if strings.HasPrefix(val, "http") {
			return val
		}
	}
	return ""
}
