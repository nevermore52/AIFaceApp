package services

import (
	"bytes"
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
	"telegram-ai-face-bot/web/internal/models"
)

type QuotaService interface {
	GetByID(id int64) (*models.User, error)
	GetQuota(telegramID int64) (*models.UserQuota, error)
	ConsumeQuota(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error)
	AddExtraQuota(telegramID int64, category models.QuotaCategory, amount int) error
	AddPrimaryQuota(telegramID int64, category models.QuotaCategory, amount int) error
}

type WebGenerationService struct {
	db           *sql.DB
	kieClient    *kieapi.Client
	callbackURL  string
	httpClient   *http.Client
	quotaService QuotaService
}

func NewWebGenerationService(db *sql.DB, kieClient *kieapi.Client, callbackURL string, quotaService QuotaService) *WebGenerationService {
	return &WebGenerationService{
		db:           db,
		kieClient:    kieClient,
		callbackURL:  callbackURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		quotaService: quotaService,
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
	Source         string     `json:"source"`
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
	// Видео параметры (Wan, Kling, Veo)
	Duration string `json:"duration"`
	Sound    string `json:"sound"`
	// Veo параметры
	Watermark         string `json:"watermark"`
	Seeds             int    `json:"seeds"`
	EnableFallback    bool   `json:"enableFallback"`
	EnableTranslation bool   `json:"enableTranslation"`
	GenerationType    string `json:"generationType"`
	// Suno Music параметры
	Instrumental bool   `json:"instrumental"`
	VocalGender  string `json:"vocal_gender"`
	// Информация о потраченных квотах (заполняется сервером)
	PrimaryUsed int `json:"primary_used"`
	ExtraUsed   int `json:"extra_used"`
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
		{ID: "gpt-4.1-mini", Name: "GPT-4.1 mini", Type: "text", Description: "Текстовая модель", TokenCost: 1},
	}
}

// getTokenCost определяет стоимость генерации в зависимости от модели и настроек
func (s *WebGenerationService) getTokenCost(model string, req CreateGenerationRequest) int {
	model = strings.ToLower(strings.TrimSpace(model))

	// Базовые стоимости из GetAvailableModels
	baseCosts := map[string]int{
		"google/nano-banana":       1,
		"google/nano-banana-pro":   4,
		"nano-banana-2":            2,
		"seedream/4.5-edit":        3,
		"veo3_fast":                1,
		"wan/2-6-image-to-video":   2,
		"kling-2.6/image-to-video": 1,
		"music-suno":               1,
		"google/gemini-3-flash":    1,
		"openai/gpt-5-mini":        1,
		"openai/gpt-5-nano":        1,
		"gpt-4.1-mini":             1,
	}

	baseCost := baseCosts[model]
	if baseCost == 0 {
		return 1 // По умолчанию
	}

	// Модификаторы стоимости для разных настроек
	if model == "nano-banana-2" {
		// Разрешение: 1K (базовое), 2K (+1), 4K (+3)
		switch req.Resolution {
		case "4K":
			return baseCost + 3
		case "2K":
			return baseCost + 1
		default:
			return baseCost
		}
	}

	if model == "google/nano-banana-pro" || model == "nano-banana-pro" {
		// Разрешение: 2K (базовое), 5K (+2)
		switch req.Resolution {
		case "5K":
			return baseCost + 2
		default:
			return baseCost
		}
	}

	if model == "wan/2-6-image-to-video" {
		// Длительность видео: 5с (базовое), 10с (+1)
		switch req.Duration {
		case "10":
			return baseCost + 1
		default:
			return baseCost
		}
	}

	if model == "kling-2.6/image-to-video" {
		// Звук: без звука (базовое), со звуком (+1)
		if req.Sound == "true" {
			return baseCost + 1
		}
	}

	return baseCost
}

func (s *WebGenerationService) CreateGeneration(userID int64, username string, req CreateGenerationRequest) (*GenerationRequest, error) {
	if s.kieClient == nil {
		return nil, fmt.Errorf("generation service not configured")
	}
	if s.callbackURL == "" {
		return nil, fmt.Errorf("callback URL not configured")
	}

	// Определяем тип модели
	modelType := "image"
	model := strings.ToLower(strings.TrimSpace(req.Model))

	if model == "music-suno" || strings.Contains(model, "suno") {
		modelType = "music"
	} else if isTextModel(model) {
		modelType = "text"
	} else if strings.Contains(model, "video") {
		modelType = "video"
	}

	// Определяем стоимость генерации
	tokenCost := s.getTokenCost(req.Model, req)

	// Проверяем и списываем квоты
	if s.quotaService != nil {
		// Получаем пользователя для получения telegram_id
		user, err := s.quotaService.GetByID(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		if user.TelegramID == nil {
			return nil, fmt.Errorf("telegram account not linked")
		}

		// Определяем категорию квоты
		var category models.QuotaCategory
		switch modelType {
		case "text":
			category = models.QuotaCategoryText
		case "image":
			category = models.QuotaCategoryImage
		case "music":
			category = models.QuotaCategoryMusic
		case "video":
			category = models.QuotaCategoryVideo
		default:
			return nil, fmt.Errorf("unknown model type: %s", modelType)
		}

		// Списываем квоту (с правильной стоимостью)
		primaryUsed, extraUsed, err := s.quotaService.ConsumeQuota(*user.TelegramID, category, tokenCost)
		if err != nil {
			return nil, fmt.Errorf("insufficient quota: %w", err)
		}

		log.Printf("Quota consumed: telegramID=%d category=%s amount=%d primary=%d extra=%d",
			*user.TelegramID, category, tokenCost, primaryUsed, extraUsed)

		// Сохраняем информацию о потраченных квотах для возврата при ошибке
		req.PrimaryUsed = primaryUsed
		req.ExtraUsed = extraUsed
	}

	inputImage := ""
	if len(req.ImageURLs) > 0 {
		inputImage = strings.Join(req.ImageURLs, ",")
	}

	query := `
		INSERT INTO generation_requests (user_id, username, model_type, model, status, input_image, prompt, tokens_used, source)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, 'web')
		RETURNING id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, source, created_at, completed_at`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, userID, username, modelType, req.Model, inputImage, req.Prompt, tokenCost).Scan(
		&genReq.ID, &genReq.UserID, &genReq.Username, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.Source, &genReq.CreatedAt, &genReq.CompletedAt,
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

	// Вспомогательная функция для возврата квот при ошибке
	refundQuota := func(primaryUsed, extraUsed int) {
		if s.quotaService != nil && (primaryUsed > 0 || extraUsed > 0) {
			// Получаем пользователя для получения telegram_id
			user, err := s.quotaService.GetByID(genReq.UserID)
			if err == nil && user.TelegramID != nil {
				// Определяем категорию квоты
				var category models.QuotaCategory
				switch genReq.ModelType {
				case "text":
					category = models.QuotaCategoryText
				case "image":
					category = models.QuotaCategoryImage
				case "music":
					category = models.QuotaCategoryMusic
				case "video":
					category = models.QuotaCategoryVideo
				}

				// Возвращаем квоты в те же типы, откуда были потрачены
				if primaryUsed > 0 {
					_ = s.quotaService.AddPrimaryQuota(*user.TelegramID, category, primaryUsed)
					log.Printf("Refunded primary quota: telegramID=%d category=%s amount=%d", *user.TelegramID, category, primaryUsed)
				}
				if extraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(*user.TelegramID, category, extraUsed)
					log.Printf("Refunded extra quota: telegramID=%d category=%s amount=%d", *user.TelegramID, category, extraUsed)
				}
			}
		}
	}

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
			input["image_input"] = s.normalizeImageURLs(req.ImageURLs)
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
			input["image_input"] = s.normalizeImageURLs(req.ImageURLs)
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
			input["image_urls"] = s.normalizeImageURLs(req.ImageURLs)
		}
		duration := req.Duration
		if duration == "" {
			duration = "5"
		}
		input["duration"] = duration
		input["resolution"] = "1080p"
		input["nsfw_checker"] = false
	} else if model == "kling-2.6/image-to-video" {
		// Kling 2.6: длительность и звук
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = s.normalizeImageURLs(req.ImageURLs)
		}
		duration := req.Duration
		if duration == "" {
			duration = "5"
		}
		input["duration"] = duration
		// Звук: false по умолчанию, true если явно передано
		sound := false
		if req.Sound == "true" {
			sound = true
		}
		input["sound"] = sound
	} else if model == "veo3_fast" {
		// Veo 3.1 Fast: используем imageUrls (camelCase) как в примере API
		if len(req.ImageURLs) > 0 {
			input["imageUrls"] = s.normalizeImageURLs(req.ImageURLs)
		}
		// Aspect ratio по умолчанию
		aspectRatio := req.AspectRatio
		if aspectRatio == "" {
			aspectRatio = "16:9"
		}
		input["aspect_ratio"] = aspectRatio

		// Опциональные параметры для Veo
		if req.Watermark != "" {
			input["watermark"] = req.Watermark
		}
		if req.Seeds != 0 {
			input["seeds"] = req.Seeds
		}
		input["enableFallback"] = req.EnableFallback
		input["enableTranslation"] = req.EnableTranslation

		// Тип генерации (REFERENCE_2_VIDEO по умолчанию для образов)
		generationType := req.GenerationType
		if generationType == "" {
			generationType = "REFERENCE_2_VIDEO"
		}
		input["generationType"] = generationType
	} else {
		// Остальные модели
		if len(req.ImageURLs) > 0 {
			input["image_urls"] = s.normalizeImageURLs(req.ImageURLs)
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

	// Log the request for debugging
	inputJSON, _ := json.Marshal(input)
	log.Printf("KieAPI request for model %s: input=%s", req.Model, string(inputJSON))

	var taskID string
	var err error

	// Use different endpoint for Veo 3.1 Fast
	if model == "veo3_fast" {
		taskID, err = s.kieClient.CreateVeoTask(taskReq)
	} else {
		taskID, err = s.kieClient.CreateTask(taskReq)
	}

	if err != nil {
		log.Printf("KieAPI task creation failed for request %d: %v", genReq.ID, err)
		_ = s.updateStatus(genReq.ID, "failed", err.Error())
		refundQuota(req.PrimaryUsed, req.ExtraUsed)
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
		model == "gpt-4.1-mini"
}

func (s *WebGenerationService) processMusicGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	audioURL, taskID, err := s.generateMusicSuno(req.Prompt, req.VocalGender, req.Instrumental)
	if err != nil {
		log.Printf("Suno generation failed: %v", err)
		_ = s.updateStatus(genReq.ID, "failed", fmt.Sprintf("Music generation failed: %v", err))
		// Возвращаем квоту
		if s.quotaService != nil {
			user, err := s.quotaService.GetByID(genReq.UserID)
			if err == nil && user.TelegramID != nil {
				if req.PrimaryUsed > 0 {
					_ = s.quotaService.AddPrimaryQuota(*user.TelegramID, models.QuotaCategoryMusic, req.PrimaryUsed)
				}
				if req.ExtraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(*user.TelegramID, models.QuotaCategoryMusic, req.ExtraUsed)
				}
			}
		}
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
		// Регистрируем задачу в боте чтобы он знал о ней когда придет callback
		_ = s.registerSunoTaskInBot(taskID, genReq.UserID)
		log.Printf("Music generation started with taskID: %s", taskID)
		return
	}

	_ = s.updateStatus(genReq.ID, "failed", "No audio URL or task ID returned from Suno API")
	// Возвращаем квоту
	if s.quotaService != nil {
		user, err := s.quotaService.GetByID(genReq.UserID)
		if err == nil && user.TelegramID != nil {
			if req.PrimaryUsed > 0 {
				_ = s.quotaService.AddPrimaryQuota(*user.TelegramID, models.QuotaCategoryMusic, req.PrimaryUsed)
			}
			if req.ExtraUsed > 0 {
				_ = s.quotaService.AddExtraQuota(*user.TelegramID, models.QuotaCategoryMusic, req.ExtraUsed)
			}
		}
	}
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
		// Возвращаем квоту
		if s.quotaService != nil {
			user, err := s.quotaService.GetByID(genReq.UserID)
			if err == nil && user.TelegramID != nil {
				if req.PrimaryUsed > 0 {
					_ = s.quotaService.AddPrimaryQuota(*user.TelegramID, models.QuotaCategoryText, req.PrimaryUsed)
				}
				if req.ExtraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(*user.TelegramID, models.QuotaCategoryText, req.ExtraUsed)
				}
			}
		}
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

func (s *WebGenerationService) HandleSunoCallback(payload map[string]any) error {
	// Log the raw payload
	payloadJSON, _ := json.Marshal(payload)
	log.Printf("HandleSunoCallback received: %s", string(payloadJSON))

	// Извлекаем taskID из payload
	taskID := findStringByKeys(payload, "taskId", "task_id", "id")
	if taskID == "" {
		log.Printf("ERROR: callback missing taskId. Payload keys: %v", getMapKeys(payload))
		return fmt.Errorf("callback missing taskId")
	}
	log.Printf("Found taskID: %s", taskID)

	genReq, err := s.getByExternalTaskID(taskID)
	if err != nil {
		log.Printf("ERROR: generation request not found for taskId %s: %v", taskID, err)
		return fmt.Errorf("generation request not found for taskId %s: %w", taskID, err)
	}
	log.Printf("Found generation request: id=%d", genReq.ID)

	// Проверяем статус
	status := findStringByKeys(payload, "status", "state")
	log.Printf("Status from callback: %s", status)
	if status != "" && status != "success" && status != "completed" && status != "succeeded" {
		reason := findStringByKeys(payload, "message", "msg", "error")
		if reason == "" {
			reason = fmt.Sprintf("status=%s", status)
		}
		log.Printf("Generation failed with status %s: %s", status, reason)
		return s.updateStatus(genReq.ID, "failed", reason)
	}

	// Извлекаем URL аудио
	audioURL := findStringByKeys(
		payload,
		"audio_url", "audioUrl", "audio",
		"audioUrlHigh", "audio_url_high", "audio_high", "audio_high_url",
		"download_url", "downloadUrl",
		"result_url", "resultUrl",
		"streaming_url", "streamingUrl",
		"preview_url", "previewUrl",
		"url",
	)
	if audioURL == "" {
		audioURL = findFirstURL(payload)
	}

	if audioURL == "" {
		log.Printf("ERROR: callback missing audio URL. Payload keys: %v", getMapKeys(payload))
		return s.updateStatus(genReq.ID, "failed", "callback missing audio URL")
	}

	log.Printf("Suno callback processed: taskID=%s audioURL=%s, updating request=%d", taskID, audioURL, genReq.ID)
	result := s.completeRequest(genReq.ID, audioURL)
	log.Printf("completeRequest result: %v", result)
	return result
}

func (s *WebGenerationService) GetByID(id int64) (*GenerationRequest, error) {
	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, source, created_at, completed_at
		FROM generation_requests WHERE id = $1`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, id).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.Source, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return genReq, nil
}

func (s *WebGenerationService) getByExternalTaskID(taskID string) (*GenerationRequest, error) {
	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, source, created_at, completed_at
		FROM generation_requests WHERE external_task_id = $1`

	genReq := &GenerationRequest{}
	err := s.db.QueryRow(query, taskID).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.Source, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		log.Printf("ERROR getByExternalTaskID: taskID=%s error=%v", taskID, err)
		return nil, err
	}
	log.Printf("getByExternalTaskID found: taskID=%s id=%d", taskID, genReq.ID)
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
	log.Printf("updateExternalTaskID: id=%d taskID=%s", id, taskID)
	_, err := s.db.Exec(`UPDATE generation_requests SET external_task_id = $2 WHERE id = $1`, id, taskID)
	if err != nil {
		log.Printf("ERROR updateExternalTaskID: %v", err)
	} else {
		log.Printf("updateExternalTaskID success")
	}
	return err
}

func (s *WebGenerationService) completeRequest(id int64, resultURL string) error {
	log.Printf("completeRequest: id=%d resultURL=%s", id, resultURL)
	_, err := s.db.Exec(`UPDATE generation_requests SET status = 'completed', output = $2, completed_at = NOW() WHERE id = $1`, id, resultURL)
	if err != nil {
		log.Printf("ERROR completeRequest: %v", err)
	} else {
		log.Printf("completeRequest success")
	}
	return err
}

// Вспомогательные функции для парсинга ответов (используются в HandleSunoCallback)
func getMapKeys(m any) []string {
	switch val := m.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		return keys
	}
	return nil
}

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

// generateChat генерирует текст через DefAPI
func (s *WebGenerationService) generateChat(model string, messages []map[string]string) (string, error) {
	apiKey := os.Getenv("DEF_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("DEF_API_KEY is not set")
	}
	baseURL := os.Getenv("DEF_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.defapi.org"
	}

	payload := map[string]any{
		"model":             model,
		"messages":          messages,
		"stream":            false,
		"temperature":       0.7,
		"top_p":             1,
		"frequency_penalty": 0,
		"presence_penalty":  0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal defapi payload: %w", err)
	}

	url := baseURL + "/api/v1/chat/completions"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("create defapi request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("defapi request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read defapi response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("defapi api error: %s", string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse defapi response: %w", err)
	}

	if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unexpected defapi response format: %s", string(respBody))
}

// normalizeImageURLs converts image URLs to direct downloadable format
// For imgur.com URLs, converts them to i.imgur.com direct downloads
func (s *WebGenerationService) normalizeImageURLs(imageURLs []string) []string {
	var result []string
	for _, url := range imageURLs {
		originalURL := url
		if strings.Contains(url, "imgur.com") && !strings.Contains(url, "i.imgur.com") {
			// Convert imgur.com URLs to direct download URLs (but not if already i.imgur.com)
			// Example: https://imgur.com/X6GS3XA -> https://i.imgur.com/X6GS3XA.jpg
			url = strings.ReplaceAll(url, "imgur.com", "i.imgur.com")
			log.Printf("Normalized imgur URL: %s -> %s", originalURL, url)
		}
		// Add file extension if needed
		if strings.Contains(url, "imgur.com") && !strings.HasSuffix(url, ".jpg") && !strings.HasSuffix(url, ".png") && !strings.HasSuffix(url, ".jpeg") {
			url = url + ".jpg"
			log.Printf("Added extension to imgur URL: %s", url)
		}
		result = append(result, url)
	}
	return result
}

// registerSunoTaskInBot notifies the bot about a new Suno task
// This ensures the bot recognizes the task when callback arrives
func (s *WebGenerationService) registerSunoTaskInBot(taskID string, userID int64) error {
	botWebhookURL := os.Getenv("BOT_SUNO_REGISTER_URL")
	if botWebhookURL == "" {
		// If no webhook URL configured, just log and continue
		log.Printf("BOT_SUNO_REGISTER_URL not configured, skipping task registration in bot")
		return nil
	}

	payload := map[string]any{
		"taskId": taskID,
		"userId": userID,
		"source": "web",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling task registration payload: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", botWebhookURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		log.Printf("Error creating task registration request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending task registration to bot: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Bot returned error for task registration: status=%d body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("bot returned status %d", resp.StatusCode)
	}

	log.Printf("Task registered in bot: taskID=%s userID=%d", taskID, userID)
	return nil
}
