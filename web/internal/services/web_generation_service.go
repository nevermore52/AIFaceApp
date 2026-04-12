package services

import (
	"bytes"
	crand "crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"telegram-ai-face-bot/web/internal/kieapi"
	"telegram-ai-face-bot/web/internal/models"
)

type QuotaService interface {
	GetByID(id int64) (*models.User, error)
	GetQuota(telegramID int64) (*models.UserQuota, error)
	EnsureDailyReset(telegramID int64) error
	ConsumeQuota(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error)
	AddExtraQuota(telegramID int64, category models.QuotaCategory, amount int) error
	AddPrimaryQuota(telegramID int64, category models.QuotaCategory, amount int) error
}

type WebGenerationService struct {
	db            *sql.DB
	kieClient     *kieapi.Client
	callbackURL   string
	botWebhookURL string // URL бота для уведомлений о завершении генераций
	httpClient    *http.Client
	quotaService  QuotaService
	uploadDir     string
	// Suno: накапливаем 2 песни перед завершением
	sunoMu       sync.Mutex
	sunoPartials map[int64][]string    // genID -> полученные audio URLs
	sunoTimers   map[int64]*time.Timer // genID -> 6-мин таймер фоллбека
	// Текстовые модели: серверный контекст диалога
	chatMu       sync.Mutex
	chatContexts map[string][]map[string]string // "userID:model" -> messages
}

func NewWebGenerationService(db *sql.DB, kieClient *kieapi.Client, callbackURL string, quotaService QuotaService, uploadDir string) *WebGenerationService {
	botURL := os.Getenv("BOT_WEBHOOK_URL")
	if botURL == "" {
		botURL = os.Getenv("WEB_BACKEND_URL") // fallback; may be empty
	}
	s := &WebGenerationService{
		db:            db,
		kieClient:     kieClient,
		callbackURL:   callbackURL,
		botWebhookURL: botURL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		quotaService:  quotaService,
		uploadDir:     uploadDir,
		sunoPartials:  make(map[int64][]string),
		sunoTimers:    make(map[int64]*time.Timer),
		chatContexts:  make(map[string][]map[string]string),
	}
	if uploadDir != "" {
		go s.runUploadCleanup(uploadDir, 30*24*time.Hour, time.Hour)
	}
	return s
}

type MediaOutput struct {
	URL       string   `json:"url"`
	URLs      []string `json:"urls,omitempty"` // Несколько файлов (например, 2 песни Suno)
	Type      string   `json:"type"`           // "audio", "video", "image"
	Title     string   `json:"title,omitempty"`
	Duration  string   `json:"duration,omitempty"`
	Thumbnail string   `json:"thumbnail,omitempty"`
	Preview   string   `json:"preview,omitempty"`
	MimeType  string   `json:"mime_type,omitempty"`
	Size      int64    `json:"size,omitempty"`
}

type GenerationRequest struct {
	ID             int64        `json:"id"`
	UserID         int64        `json:"user_id"`
	Username       string       `json:"username"`
	Model          string       `json:"model"`
	ModelType      string       `json:"model_type"`
	Prompt         string       `json:"prompt"`
	InputImage     string       `json:"input_image,omitempty"`
	Status         string       `json:"status"`
	StatusMessage  string       `json:"status_message"`
	Output         *string      `json:"output,omitempty"`
	MediaOutput    *MediaOutput `json:"media_output,omitempty"`
	ErrorMsg       *string      `json:"error_msg,omitempty"`
	ExternalTaskID string       `json:"external_task_id,omitempty"`
	TokensUsed     int          `json:"tokens_used"`
	Source         string       `json:"source"`
	CreatedAt      time.Time    `json:"created_at"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
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
	// История чата для текстовых моделей (опционально)
	Messages []map[string]string `json:"messages"`
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
	Placeholder string `json:"placeholder"`
}

// Вспомогательная функция для минимума
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *WebGenerationService) GetAvailableModels() []ModelInfo {
	return []ModelInfo{
		{ID: "nano-banana-2", Name: "Nano Banana 2", Type: "image", Description: "Генерация и редактирование изображений", TokenCost: 2, Placeholder: "Опишите, что вы хотите получить в изображении"},
		{ID: "google/nano-banana", Name: "Nano Banana", Type: "image", Description: "Генерация изображений", TokenCost: 1, Placeholder: "Опишите, что вы хотите получить в изображении"},
		{ID: "google/nano-banana-pro", Name: "Nano Banana Pro", Type: "image", Description: "Продвинутая генерация изображений", TokenCost: 4, Placeholder: "Опишите, что вы хотите получить в изображении"},
		{ID: "seedream/4.5-edit", Name: "Seedream 4.5", Type: "image", Description: "Редактирование изображений", TokenCost: 3, Placeholder: "Опишите, что вы хотите получить в изображении"},
		{ID: "veo3_fast", Name: "Veo 3.1 Fast", Type: "video", Description: "Генерация видео", TokenCost: 1, Placeholder: "Опишите, что вы хотите получить в видео"},
		{ID: "wan/2-6-image-to-video", Name: "Wan 2.6", Type: "video", Description: "Генерация видео из изображения", TokenCost: 2, Placeholder: "Опишите, что вы хотите получить в видео"},
		{ID: "kling-2.6/image-to-video", Name: "Kling 2.6", Type: "video", Description: "Генерация видео с звуком", TokenCost: 1, Placeholder: "Опишите, что вы хотите получить в видео"},
		{ID: "music-suno", Name: "Suno Music", Type: "music", Description: "Генерация музыки", TokenCost: 1, Placeholder: "Опишите, что вы хотите получить в песне"},
		{ID: "google/gemini-3-flash", Name: "Gemini 3 Flash", Type: "text", Description: "Текстовая модель", TokenCost: 1, Placeholder: "Введите запрос"},
		{ID: "openai/gpt-5-mini", Name: "GPT-5 mini", Type: "text", Description: "Текстовая модель", TokenCost: 1, Placeholder: "Введите запрос"},
		{ID: "openai/gpt-5-nano", Name: "GPT-5 nano", Type: "text", Description: "Текстовая модель", TokenCost: 1, Placeholder: "Введите запрос"},
	}
}

// getStatusMessage генерирует человекопонятное сообщение о статусе
func (s *WebGenerationService) getStatusMessage(status, modelType string) string {
	switch status {
	case "pending":
		switch modelType {
		case "music":
			return "🎵 Готовлюсь сочинить песню..."
		case "video":
			return "🎬 Готовлюсь создать видео..."
		case "image":
			return "🎨 Готовлюсь нарисовать изображение..."
		case "text":
			return "💭 Готовлюсь сгенерировать текст..."
		default:
			return "⏳ Ожидаю начала генерации..."
		}
	case "processing":
		switch modelType {
		case "music":
			return "🎵 Сочиняю песню, это может занять время..."
		case "video":
			return "🎬 Создаю видео, это может занять время..."
		case "image":
			return "🎨 Рисую изображение, это может занять время..."
		case "text":
			return "💭 Генерирую текст, это может занять время..."
		default:
			return "⚡ Обрабатываю запрос..."
		}
	case "completed":
		switch modelType {
		case "music":
			return "🎵 Песня готова!"
		case "video":
			return "🎬 Видео готово!"
		case "image":
			return "🎨 Изображение готово!"
		case "text":
			return "💭 Текст сгенерирован!"
		default:
			return "✅ Готово!"
		}
	case "failed":
		switch modelType {
		case "music":
			return "❌ Не удалось сочинить песню"
		case "video":
			return "❌ Не удалось создать видео"
		case "image":
			return "❌ Не удалось нарисовать изображение"
		case "text":
			return "❌ Не удалось сгенерировать текст"
		default:
			return "❌ Ошибка генерации"
		}
	default:
		return status
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

		quotaUserID := user.ID
		if user.TelegramID != nil {
			quotaUserID = *user.TelegramID
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

		// Сбрасываем ежедневную квоту если новый день
		if category == models.QuotaCategoryText {
			if err := s.quotaService.EnsureDailyReset(quotaUserID); err != nil {
				log.Printf("EnsureDailyReset failed for user %d: %v", quotaUserID, err)
			}
		}

		// Списываем квоту (с правильной стоимостью)
		primaryUsed, extraUsed, err := s.quotaService.ConsumeQuota(quotaUserID, category, tokenCost)
		if err != nil {
			return nil, err
		}

		log.Printf("Quota consumed: userID=%d category=%s amount=%d primary=%d extra=%d",
			quotaUserID, category, tokenCost, primaryUsed, extraUsed)

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
			if err == nil {
				refundID := user.ID
				if user.TelegramID != nil {
					refundID = *user.TelegramID
				}
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
					_ = s.quotaService.AddPrimaryQuota(refundID, category, primaryUsed)
					log.Printf("Refunded primary quota: userID=%d category=%s amount=%d", refundID, category, primaryUsed)
				}
				if extraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(refundID, category, extraUsed)
					log.Printf("Refunded extra quota: userID=%d category=%s amount=%d", refundID, category, extraUsed)
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
			input["image_input"] = s.reuploadImagesToKie(req.ImageURLs)
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
			input["image_input"] = s.reuploadImagesToKie(req.ImageURLs)
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
			input["image_urls"] = s.reuploadImagesToKie(req.ImageURLs)
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
			input["image_urls"] = s.reuploadImagesToKie(req.ImageURLs)
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
		// Veo 3.1 Fast: imageUrls (camelCase), максимум 2 изображения
		if len(req.ImageURLs) > 0 {
			urls := req.ImageURLs
			if len(urls) > 2 {
				urls = urls[:2]
			}
			input["imageUrls"] = s.reuploadImagesToKie(urls)
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
			input["image_urls"] = s.reuploadImagesToKie(req.ImageURLs)
		}
	}

	if model == "seedream/4.5-edit" {
		input["quality"] = "basic"
	}

	taskReq := kieapi.CreateTaskRequest{
		Model:       kieModelName(req.Model),
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

// kieModelName maps internal model IDs to the names KieAPI expects.
func kieModelName(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "google/nano-banana":
		return "google/nano-banana-edit"
	case "google/nano-banana-pro":
		return "nano-banana-pro"
	case "seedream/4.5-edit":
		return "seedream/4.5-edit"
	}
	return model
}

func isTextModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "gpt") ||
		strings.Contains(model, "gemini") ||
		model == "google/gemini-3-flash" ||
		model == "openai/gpt-5-mini" ||
		model == "openai/gpt-5-nano"
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
			if err == nil {
				refundID := user.ID
				if user.TelegramID != nil {
					refundID = *user.TelegramID
				}
				if req.PrimaryUsed > 0 {
					_ = s.quotaService.AddPrimaryQuota(refundID, models.QuotaCategoryMusic, req.PrimaryUsed)
				}
				if req.ExtraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(refundID, models.QuotaCategoryMusic, req.ExtraUsed)
				}
			}
		}
		return
	}

	// Если есть готовый audioURL, сразу завершаем с MediaOutput
	if audioURL != "" {
		title := "Музыка"
		if req.Prompt != "" {
			promptLength := min(50, len(req.Prompt))
			if promptLength > 0 {
				title = fmt.Sprintf("Музыка: %s", req.Prompt[:promptLength])
			}
		}

		mediaOutput := &MediaOutput{
			URL:      audioURL,
			Type:     "audio",
			Title:    title,
			MimeType: "audio/mpeg",
		}
		_ = s.completeGenerationWithMedia(genReq.ID, audioURL, mediaOutput)
		return
	}

	// Если только taskID, сохраняем его для обработки callback
	if taskID != "" {
		_ = s.updateExternalTaskID(genReq.ID, taskID)
		// Регистрируем задачу в боте чтобы он знал о ней когда придет callback
		if err := s.registerSunoTaskInBot(taskID, genReq.UserID); err != nil {
			log.Printf("ERROR registering Suno task in bot: %v", err)
		}
		log.Printf("Music generation started with taskID: %s", taskID)
		return
	}

	_ = s.updateStatus(genReq.ID, "failed", "No audio URL or task ID returned from Suno API")
	// Возвращаем квоту
	if s.quotaService != nil {
		user, err := s.quotaService.GetByID(genReq.UserID)
		if err == nil {
			refundID := user.ID
			if user.TelegramID != nil {
				refundID = *user.TelegramID
			}
			if req.PrimaryUsed > 0 {
				_ = s.quotaService.AddPrimaryQuota(refundID, models.QuotaCategoryMusic, req.PrimaryUsed)
			}
			if req.ExtraUsed > 0 {
				_ = s.quotaService.AddExtraQuota(refundID, models.QuotaCategoryMusic, req.ExtraUsed)
			}
		}
	}
}

func (s *WebGenerationService) processTextGeneration(genReq *GenerationRequest, req CreateGenerationRequest) {
	_ = s.updateStatus(genReq.ID, "processing", "")

	ctxKey := fmt.Sprintf("%d:%s", genReq.UserID, req.Model)

	// Если фронтенд прислал полную историю (2+ сообщений) — используем её и сохраняем как новый контекст.
	// Если прислано только 1 сообщение или ничего — подгружаем серверный контекст.
	var messages []map[string]string
	if len(req.Messages) >= 2 {
		// Фронтенд прислал историю — используем её как есть
		messages = req.Messages
		// Синхронизируем серверный контекст (без последнего user-сообщения, оно добавится после ответа)
		s.chatMu.Lock()
		s.chatContexts[ctxKey] = req.Messages[:len(req.Messages)-1]
		s.chatMu.Unlock()
	} else {
		// Загружаем серверный контекст и добавляем текущее сообщение
		s.chatMu.Lock()
		stored := s.chatContexts[ctxKey]
		s.chatMu.Unlock()

		userMsg := req.Prompt
		if len(req.Messages) == 1 {
			if c, ok := req.Messages[0]["content"]; ok && c != "" {
				userMsg = c
			}
		}
		messages = append(stored, map[string]string{"role": "user", "content": userMsg})
	}

	response, err := s.generateChat(req.Model, messages)
	if err != nil {
		log.Printf("Chat generation failed: %v", err)
		_ = s.updateStatus(genReq.ID, "failed", fmt.Sprintf("Text generation failed: %v", err))
		// Возвращаем квоту
		if s.quotaService != nil {
			user, err := s.quotaService.GetByID(genReq.UserID)
			if err == nil {
				refundID := user.ID
				if user.TelegramID != nil {
					refundID = *user.TelegramID
				}
				if req.PrimaryUsed > 0 {
					_ = s.quotaService.AddPrimaryQuota(refundID, models.QuotaCategoryText, req.PrimaryUsed)
				}
				if req.ExtraUsed > 0 {
					_ = s.quotaService.AddExtraQuota(refundID, models.QuotaCategoryText, req.ExtraUsed)
				}
			}
		}
		return
	}

	// Сохраняем контекст диалога (последние 20 сообщений = 10 ходов)
	s.chatMu.Lock()
	updated := append(messages, map[string]string{"role": "assistant", "content": response})
	if len(updated) > 20 {
		updated = updated[len(updated)-20:]
	}
	s.chatContexts[ctxKey] = updated
	s.chatMu.Unlock()

	_ = s.completeGeneration(genReq.ID, response)
}

func (s *WebGenerationService) completeGeneration(id int64, output string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE generation_requests
		SET status = 'completed', output = $1, completed_at = $2
		WHERE id = $3
	`, output, now, id)
	if err == nil {
		go s.notifyBotAboutCompletion(id)
	}
	return err
}

// notifyBotAboutCompletion sends the completed generation result to the bot's
// Telegram notification endpoint so the user receives it in their TG chat.
func (s *WebGenerationService) notifyBotAboutCompletion(genID int64) {
	if s.botWebhookURL == "" {
		return
	}

	// Load the generation request
	var userID int64
	var model, modelType, status string
	var output, errorMsg sql.NullString
	var tokensUsed, tokensPrimaryUsed, tokensExtraUsed int
	err := s.db.QueryRow(`
		SELECT user_id, model, model_type, status, output, error_msg,
		       tokens_used, COALESCE(tokens_primary_used,0), COALESCE(tokens_extra_used,0)
		FROM generation_requests WHERE id = $1
	`, genID).Scan(&userID, &model, &modelType, &status, &output, &errorMsg,
		&tokensUsed, &tokensPrimaryUsed, &tokensExtraUsed)
	if err != nil {
		log.Printf("notifyBot: failed to load gen %d: %v", genID, err)
		return
	}

	// Skip text generations — they are streaming chat responses shown on the web
	if modelType == "text" {
		return
	}

	// Look up user's telegram_id
	if s.quotaService == nil {
		return
	}
	user, err := s.quotaService.GetByID(userID)
	if err != nil || user.TelegramID == nil {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"telegram_id":         *user.TelegramID,
		"status":              status,
		"model":               model,
		"model_type":          modelType,
		"output":              output.String,
		"error_msg":           errorMsg.String,
		"tokens_used":         tokensUsed,
		"tokens_primary_used": tokensPrimaryUsed,
		"tokens_extra_used":   tokensExtraUsed,
	})

	url := strings.TrimRight(s.botWebhookURL, "/") + "/web/generation/notify"
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("notifyBot: POST failed for gen %d: %v", genID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("notifyBot: bot returned %d for gen %d", resp.StatusCode, genID)
	}
}

func (s *WebGenerationService) completeGenerationWithMedia(id int64, output string, mediaOutput *MediaOutput) error {
	now := time.Now()

	// Сериализуем MediaOutput в JSON
	mediaJSON := ""
	if mediaOutput != nil {
		mediaBytes, err := json.Marshal(mediaOutput)
		if err != nil {
			log.Printf("Failed to marshal media output: %v", err)
			return s.completeGeneration(id, output)
		}
		mediaJSON = string(mediaBytes)
	}

	_, err := s.db.Exec(`
		UPDATE generation_requests
		SET status = 'completed', output = $1, media_output = $2, completed_at = $3
		WHERE id = $4
	`, output, mediaJSON, now, id)
	if err == nil {
		go s.notifyBotAboutCompletion(id)
	}
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

	// Промежуточные статусы — игнорируем, ждём финального callback
	switch status {
	case "processing", "pending", "running", "queued", "queue", "in_progress", "submitted", "waiting":
		log.Printf("HandleCallback: interim status %q for task %s, skipping", status, taskID)
		return nil
	}

	// Явная ошибка
	if status == "failed" || status == "error" || status == "cancelled" || status == "canceled" {
		reason := payload.Msg
		if reason == "" || strings.ToLower(reason) == "success" {
			reason = fmt.Sprintf("status=%s", payload.StatusValue())
		}
		return s.updateStatus(genReq.ID, "failed", reason)
	}

	// Неизвестный статус — игнорируем (не помечаем как ошибку)
	if status != "success" && status != "completed" && status != "succeeded" {
		log.Printf("HandleCallback: unknown status %q for task %s, ignoring", status, taskID)
		return nil
	}

	resultURL := payload.ResultURL()
	if resultURL == "" {
		return s.updateStatus(genReq.ID, "failed", "callback missing result URL")
	}

	return s.completeRequest(genReq.ID, resultURL)
}

func (s *WebGenerationService) HandleSunoCallback(payload map[string]any) error {
	payloadJSON, _ := json.Marshal(payload)
	log.Printf("HandleSunoCallback received: %s", string(payloadJSON))

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

	// Проверяем статус: ошибка — завершаем сразу, не ждём 2 песни
	status := findStringByKeys(payload, "status", "state")
	log.Printf("Status from callback: %s", status)
	statusLower := strings.ToLower(status)
	// Промежуточные статусы — пропускаем
	switch statusLower {
	case "processing", "pending", "running", "queued", "queue", "in_progress", "submitted", "waiting":
		log.Printf("HandleSunoCallback: interim status %q for task %s, skipping", status, taskID)
		return nil
	}
	// Явная ошибка
	if statusLower == "failed" || statusLower == "error" || statusLower == "cancelled" || statusLower == "canceled" {
		reason := findStringByKeys(payload, "message", "msg", "error")
		if reason == "" || strings.ToLower(reason) == "success" {
			reason = fmt.Sprintf("status=%s", status)
		}
		log.Printf("Generation failed with status %s: %s", status, reason)
		s.sunoMu.Lock()
		delete(s.sunoPartials, genReq.ID)
		if t, ok := s.sunoTimers[genReq.ID]; ok {
			t.Stop()
			delete(s.sunoTimers, genReq.ID)
		}
		s.sunoMu.Unlock()
		return s.updateStatus(genReq.ID, "failed", reason)
	}

	// Если генерация уже завершена — игнорируем дублирующийся callback
	if genReq.Status == "completed" || genReq.Status == "failed" {
		log.Printf("Suno callback ignored: genID=%d already %s", genReq.ID, genReq.Status)
		return nil
	}

	// Извлекаем ВСЕ аудио-URL из payload (source_audio_url и другие поля)
	newURLs := findAllAudioURLsFromPayload(payload)
	if len(newURLs) == 0 {
		// Последний шанс — findFirstURL
		if u := findFirstURL(payload); u != "" {
			newURLs = []string{u}
		}
	}

	if len(newURLs) == 0 {
		log.Printf("ERROR: callback missing audio URL. Payload keys: %v", getMapKeys(payload))
		return s.updateStatus(genReq.ID, "failed", "callback missing audio URL")
	}

	log.Printf("Suno callback: genID=%d got %d audio URL(s): %v", genReq.ID, len(newURLs), newURLs)

	s.sunoMu.Lock()
	existing := s.sunoPartials[genReq.ID]
	// Добавляем только уникальные URL
	seen := make(map[string]bool)
	for _, u := range existing {
		seen[u] = true
	}
	for _, u := range newURLs {
		if !seen[u] {
			seen[u] = true
			existing = append(existing, u)
		}
	}
	s.sunoPartials[genReq.ID] = existing
	urls := existing

	if len(urls) >= 2 {
		// Получили обе песни — останавливаем таймер и завершаем
		if t, ok := s.sunoTimers[genReq.ID]; ok {
			t.Stop()
			delete(s.sunoTimers, genReq.ID)
		}
		delete(s.sunoPartials, genReq.ID)
		s.sunoMu.Unlock()
		log.Printf("Suno genID=%d: got %d songs, completing", genReq.ID, len(urls))
		return s.completeSunoGeneration(genReq.ID, genReq.Prompt, urls)
	}

	// Ещё не хватает песен — запускаем 6-минутный фоллбек если ещё не запущен
	if _, exists := s.sunoTimers[genReq.ID]; !exists {
		genID := genReq.ID
		genPrompt := genReq.Prompt
		t := time.AfterFunc(6*time.Minute, func() {
			s.sunoMu.Lock()
			partialURLs := s.sunoPartials[genID]
			delete(s.sunoPartials, genID)
			delete(s.sunoTimers, genID)
			s.sunoMu.Unlock()
			if len(partialURLs) > 0 {
				log.Printf("Suno genID=%d: 6-min timeout, completing with %d song(s)", genID, len(partialURLs))
				_ = s.completeSunoGeneration(genID, genPrompt, partialURLs)
			}
		})
		s.sunoTimers[genReq.ID] = t
		log.Printf("Suno genID=%d: got %d song(s) so far, waiting 6 min for more", genReq.ID, len(urls))
	}
	s.sunoMu.Unlock()
	return nil
}

// completeSunoGeneration завершает генерацию Suno с одной или двумя песнями
func (s *WebGenerationService) completeSunoGeneration(id int64, prompt string, urls []string) error {
	title := "Музыка"
	if prompt != "" {
		n := len(prompt)
		if n > 50 {
			n = 50
		}
		title = fmt.Sprintf("Музыка: %s", prompt[:n])
	}
	mediaOutput := &MediaOutput{
		URL:      urls[0],
		URLs:     urls,
		Type:     "audio",
		Title:    title,
		MimeType: "audio/mpeg",
	}
	return s.completeGenerationWithMedia(id, urls[0], mediaOutput)
}

func (s *WebGenerationService) GetByID(id int64) (*GenerationRequest, error) {
	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, media_output, prompt, error_msg, tokens_used, source, created_at, completed_at
		FROM generation_requests WHERE id = $1`

	genReq := &GenerationRequest{}
	var mediaOutputJSON *string
	err := s.db.QueryRow(query, id).Scan(
		&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
		&genReq.InputImage, &genReq.Output, &mediaOutputJSON, &genReq.Prompt, &genReq.ErrorMsg,
		&genReq.TokensUsed, &genReq.Source, &genReq.CreatedAt, &genReq.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Генерируем динамическое сообщение о статусе
	genReq.StatusMessage = s.getStatusMessage(genReq.Status, genReq.ModelType)

	// Десериализуем media_output если есть
	if mediaOutputJSON != nil && *mediaOutputJSON != "" {
		var mediaOutput MediaOutput
		if err := json.Unmarshal([]byte(*mediaOutputJSON), &mediaOutput); err == nil {
			genReq.MediaOutput = &mediaOutput
		}
	}

	return genReq, nil
}

func (s *WebGenerationService) GetByUserID(userID int64, limit, offset int) ([]*GenerationRequest, int, error) {
	// Сначала получаем общее количество
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM generation_requests WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, model_type, model, status, input_image, output, media_output, prompt, error_msg, tokens_used, source, created_at, completed_at
		FROM generation_requests 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var generations []*GenerationRequest
	for rows.Next() {
		genReq := &GenerationRequest{}
		var mediaOutputJSON *string
		if err := rows.Scan(
			&genReq.ID, &genReq.UserID, &genReq.ModelType, &genReq.Model, &genReq.Status,
			&genReq.InputImage, &genReq.Output, &mediaOutputJSON, &genReq.Prompt, &genReq.ErrorMsg,
			&genReq.TokensUsed, &genReq.Source, &genReq.CreatedAt, &genReq.CompletedAt,
		); err != nil {
			return nil, 0, err
		}

		// Генерируем динамическое сообщение о статусе
		genReq.StatusMessage = s.getStatusMessage(genReq.Status, genReq.ModelType)

		// Десериализуем media_output если есть
		if mediaOutputJSON != nil && *mediaOutputJSON != "" {
			var mediaOutput MediaOutput
			if err := json.Unmarshal([]byte(*mediaOutputJSON), &mediaOutput); err == nil {
				genReq.MediaOutput = &mediaOutput
			}
		}

		generations = append(generations, genReq)
	}

	return generations, total, rows.Err()
}

// GetPublicGallery returns gallery ideas for the public gallery.
// sort="all" → priority ASC NULLS LAST, created_at DESC; sort="new" (default) → created_at DESC.
func (s *WebGenerationService) GetPublicGallery(limit, offset int, sort string) ([]*GenerationRequest, int, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM gallery_ideas`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	orderClause := `created_at DESC`
	if sort == "all" {
		orderClause = `priority ASC NULLS LAST, MD5(id::text)`
	}

	rows, err := s.db.Query(`
		SELECT id, model, output, prompt, created_at
		FROM gallery_ideas
		ORDER BY `+orderClause+`
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*GenerationRequest
	for rows.Next() {
		g := &GenerationRequest{}
		g.ModelType = "image"
		g.Status = "completed"
		if err := rows.Scan(&g.ID, &g.Model, &g.Output, &g.Prompt, &g.CreatedAt); err != nil {
			continue
		}
		results = append(results, g)
	}
	return results, total, rows.Err()
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

// humanizeGenerationError переводит технические ошибки генерации в понятные пользователю сообщения
func humanizeGenerationError(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "playground task failed"),
		strings.Contains(lower, "task failed"),
		strings.Contains(lower, "generation failed"):
		return "Нейросеть не смогла обработать запрос. Попробуйте изменить промпт или загрузить другое фото."
	case strings.Contains(lower, "nsfw"), strings.Contains(lower, "safety"),
		strings.Contains(lower, "content policy"), strings.Contains(lower, "blocked"):
		return "Запрос заблокирован системой безопасности. Измените изображение или текст запроса."
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "timed out"):
		return "Время ожидания истекло. Попробуйте ещё раз."
	case strings.Contains(lower, "invalid image"), strings.Contains(lower, "image fetch failed"),
		strings.Contains(lower, "cannot process image"), strings.Contains(lower, "image error"):
		return "Не удалось обработать изображение. Убедитесь, что фото чёткое и попробуйте снова."
	case strings.Contains(lower, "insufficient quota"), strings.Contains(lower, "quota"):
		return "Недостаточно запросов. Пополните баланс."
	case strings.Contains(lower, "no audio url"), strings.Contains(lower, "missing result"):
		return "Нейросеть не вернула результат. Попробуйте ещё раз."
	case strings.Contains(lower, "status=failed"), strings.Contains(lower, "status=error"):
		return "Нейросеть не смогла обработать запрос. Попробуйте изменить промпт или фото."
	case strings.Contains(lower, "music generation failed"):
		return "Не удалось создать музыку. Попробуйте изменить описание."
	case strings.Contains(lower, "text generation failed"):
		return "Не удалось сгенерировать текст. Попробуйте изменить запрос."
	}
	return "Нейросеть не смогла обработать запрос. Попробуйте изменить промпт или фото."
}

func (s *WebGenerationService) updateStatus(id int64, status, errorMsg string) error {
	var query string
	if errorMsg != "" {
		query = `UPDATE generation_requests SET status = $2, error_msg = $3, completed_at = NOW() WHERE id = $1`
		_, err := s.db.Exec(query, id, status, humanizeGenerationError(errorMsg))
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

	// Получаем информацию о генерации для определения типа
	genReq, err := s.GetByID(id)
	if err != nil {
		log.Printf("ERROR getting generation request: %v", err)
		// Если не удалось получить, сохраняем как обычно
		_, err = s.db.Exec(`UPDATE generation_requests SET status = 'completed', output = $2, completed_at = NOW() WHERE id = $1`, id, resultURL)
		return err
	}

	// Создаем MediaOutput в зависимости от типа модели
	var mediaOutput *MediaOutput
	switch genReq.ModelType {
	case "video":
		title := "Видео"
		if genReq.Prompt != "" {
			promptLength := min(50, len(genReq.Prompt))
			if promptLength > 0 {
				title = fmt.Sprintf("Видео: %s", genReq.Prompt[:promptLength])
			}
		}
		mediaOutput = &MediaOutput{
			URL:       resultURL,
			Type:      "video",
			Title:     title,
			MimeType:  "video/mp4",
			Thumbnail: "", // Можно будет добавить генерацию thumbnail
		}
	case "image":
		title := "Изображение"
		if genReq.Prompt != "" {
			promptLength := min(50, len(genReq.Prompt))
			if promptLength > 0 {
				title = fmt.Sprintf("Изображение: %s", genReq.Prompt[:promptLength])
			}
		}
		mediaOutput = &MediaOutput{
			URL:      resultURL,
			Type:     "image",
			Title:    title,
			MimeType: "image/jpeg",
		}
	case "music":
		title := "Музыка"
		if genReq.Prompt != "" {
			promptLength := min(50, len(genReq.Prompt))
			if promptLength > 0 {
				title = fmt.Sprintf("Музыка: %s", genReq.Prompt[:promptLength])
			}
		}
		mediaOutput = &MediaOutput{
			URL:      resultURL,
			Type:     "audio",
			Title:    title,
			MimeType: "audio/mpeg",
		}
	}

	// Используем новый метод с MediaOutput
	err = s.completeGenerationWithMedia(id, resultURL, mediaOutput)
	if err != nil {
		log.Printf("ERROR completeRequest: %v", err)
	} else {
		log.Printf("completeRequest success with media output")
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

// findAllStringsByKey собирает ВСЕ значения указанного ключа из вложенной структуры
func findAllStringsByKey(v any, key string) []string {
	var results []string
	switch val := v.(type) {
	case map[string]any:
		if raw, ok := val[key]; ok {
			if s, ok2 := raw.(string); ok2 && s != "" {
				results = append(results, s)
			}
		}
		for k, vv := range val {
			if k == key {
				continue // уже обработали выше
			}
			results = append(results, findAllStringsByKey(vv, key)...)
		}
	case []any:
		for _, vv := range val {
			results = append(results, findAllStringsByKey(vv, key)...)
		}
	}
	return results
}

// findAllAudioURLsFromPayload извлекает все аудио-URL из callback-payload Suno
func findAllAudioURLsFromPayload(payload map[string]any) []string {
	audioKeys := []string{
		"source_audio_url", "audio_url", "audioUrl",
		"audioUrlHigh", "audio_url_high", "audio_high_url",
		"download_url", "downloadUrl",
		"result_url", "resultUrl",
	}
	seen := make(map[string]bool)
	var result []string
	for _, k := range audioKeys {
		for _, u := range findAllStringsByKey(payload, k) {
			if strings.HasPrefix(u, "http") && !seen[u] {
				seen[u] = true
				result = append(result, u)
			}
		}
	}
	return result
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
		"source_audio_url",
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
		if url == "" {
			continue
		}

		originalURL := url

		// Only modify imgur.com URLs that aren't already i.imgur.com
		if strings.Contains(url, "imgur.com") && !strings.Contains(url, "i.imgur.com") {
			url = strings.ReplaceAll(url, "imgur.com", "i.imgur.com")
			log.Printf("Normalized imgur URL: %s -> %s", originalURL, url)
		}

		result = append(result, url)
	}
	return result
}

// reuploadImagesToKie downloads each URL and re-uploads it to KieAPI storage.
// This avoids "Image fetch failed" errors caused by Imgur hotlink protection.
func (s *WebGenerationService) reuploadImagesToKie(urls []string) []string {
	if s.kieClient == nil {
		return urls
	}
	result := make([]string, 0, len(urls))
	for i, rawURL := range urls {
		if rawURL == "" {
			continue
		}
		resp, err := s.httpClient.Get(rawURL)
		if err != nil {
			log.Printf("reuploadImagesToKie: failed to download url[%d] %s: %v", i, rawURL, err)
			result = append(result, rawURL) // keep original as fallback
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode >= 400 {
			log.Printf("reuploadImagesToKie: error reading url[%d] status=%d err=%v", i, resp.StatusCode, err)
			result = append(result, rawURL)
			continue
		}
		filename := fmt.Sprintf("image%d.jpg", i)
		kieURL, err := s.kieClient.UploadFile(data, filename)
		if err != nil {
			log.Printf("reuploadImagesToKie: KieAPI upload failed for url[%d]: %v, keeping original", i, err)
			result = append(result, rawURL)
			continue
		}
		log.Printf("reuploadImagesToKie: %s -> %s", rawURL, kieURL)
		result = append(result, kieURL)
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

// SaveUploadedFile saves an uploaded image to local storage, records it in user_uploads, and returns its public URL.
func (s *WebGenerationService) SaveUploadedFile(file io.Reader, originalFilename, uploadDir, baseURL string, userID int64) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	ext := ".jpg"
	if idx := strings.LastIndex(originalFilename, "."); idx >= 0 {
		e := strings.ToLower(originalFilename[idx:])
		if e == ".jpg" || e == ".jpeg" || e == ".png" || e == ".webp" || e == ".gif" || e == ".heic" || e == ".heif" || e == ".avif" {
			ext = e
		}
	}

	uid, err := generateUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}
	filename := uid + ext
	fullPath := uploadDir + "/" + filename

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload dir: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/api/uploads/" + filename
	log.Printf("Image saved locally: %s -> %s", fullPath, url)

	if userID > 0 {
		const maxUploads = 10
		// Insert new record
		_, _ = s.db.Exec(`INSERT INTO user_uploads (user_id, url, filename) VALUES ($1, $2, $3)`, userID, url, filename)
		// Delete oldest records beyond the limit, also removing their files from disk
		rows, err := s.db.Query(`
			SELECT filename FROM user_uploads
			WHERE user_id = $1
			ORDER BY created_at DESC
			OFFSET $2
		`, userID, maxUploads)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var oldFilename string
				if err := rows.Scan(&oldFilename); err != nil {
					continue
				}
				_ = os.Remove(uploadDir + "/" + oldFilename)
				_, _ = s.db.Exec(`DELETE FROM user_uploads WHERE user_id = $1 AND filename = $2`, userID, oldFilename)
			}
		}
	}

	return url, nil
}

// GetUserUploads returns the most recent uploaded images for a user.
func (s *WebGenerationService) GetUserUploads(userID int64, limit int) ([]map[string]string, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT url, filename, created_at FROM user_uploads
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]string
	var staleFilenames []string
	for rows.Next() {
		var url, filename string
		var createdAt interface{}
		if err := rows.Scan(&url, &filename, &createdAt); err != nil {
			continue
		}
		// If we know the upload dir, verify the file still exists on disk
		if s.uploadDir != "" && filename != "" {
			if _, err := os.Stat(s.uploadDir + "/" + filename); os.IsNotExist(err) {
				staleFilenames = append(staleFilenames, filename)
				continue
			}
		}
		result = append(result, map[string]string{"url": url, "filename": filename})
	}
	// Clean up stale DB records
	for _, fn := range staleFilenames {
		_, _ = s.db.Exec(`DELETE FROM user_uploads WHERE filename = $1`, fn)
	}
	return result, nil
}

// runUploadCleanup periodically deletes uploaded files older than maxAge.
func (s *WebGenerationService) runUploadCleanup(uploadDir string, maxAge, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		entries, err := os.ReadDir(uploadDir)
		if err != nil {
			continue
		}
		cutoff := time.Now().Add(-maxAge)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				path := uploadDir + "/" + e.Name()
				if err := os.Remove(path); err == nil {
					log.Printf("Deleted expired upload: %s", path)
					_, _ = s.db.Exec(`DELETE FROM user_uploads WHERE filename = $1`, e.Name())
				}
			}
		}
	}
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// Gallery Ideas Management

type GalleryIdea struct {
	ID        int64     `json:"id"`
	Model     string    `json:"model"`
	Output    string    `json:"output"`
	Prompt    string    `json:"prompt"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *WebGenerationService) CreateGalleryIdea(model, output, prompt string) (*GalleryIdea, error) {
	idea := &GalleryIdea{}
	err := s.db.QueryRow(`
		INSERT INTO gallery_ideas (model, output, prompt, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, model, output, prompt, created_at, updated_at
	`, model, output, prompt).Scan(&idea.ID, &idea.Model, &idea.Output, &idea.Prompt, &idea.CreatedAt, &idea.UpdatedAt)
	return idea, err
}

func (s *WebGenerationService) GetGalleryIdeas(limit, offset int) ([]*GalleryIdea, int, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM gallery_ideas`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
		SELECT id, model, output, prompt, created_at, updated_at
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
		if err := rows.Scan(&idea.ID, &idea.Model, &idea.Output, &idea.Prompt, &idea.CreatedAt, &idea.UpdatedAt); err != nil {
			continue
		}
		ideas = append(ideas, idea)
	}
	return ideas, total, rows.Err()
}

func (s *WebGenerationService) UpdateGalleryIdea(id int64, model, output, prompt string) error {
	_, err := s.db.Exec(`
		UPDATE gallery_ideas
		SET model = $1, output = $2, prompt = $3, updated_at = NOW()
		WHERE id = $4
	`, model, output, prompt, id)
	return err
}

func (s *WebGenerationService) DeleteGalleryIdea(id int64) error {
	_, err := s.db.Exec(`DELETE FROM gallery_ideas WHERE id = $1`, id)
	return err
}
