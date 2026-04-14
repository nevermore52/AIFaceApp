package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"telegram-ai-face-bot/web/internal/kieapi"
	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type GenerationHandler struct {
	generationService    *services.GenerationService
	webGenerationService *services.WebGenerationService
	uploadDir            string
	webBaseURL           string
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewGenerationHandler(generationService *services.GenerationService, webGenerationService *services.WebGenerationService, uploadDir, webBaseURL string) *GenerationHandler {
	return &GenerationHandler{
		generationService:    generationService,
		webGenerationService: webGenerationService,
		uploadDir:            uploadDir,
		webBaseURL:           webBaseURL,
	}
}

func (h *GenerationHandler) GetUserGenerations(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	// Используем WebGenerationService для получения генераций с динамическими статусами
	if h.webGenerationService != nil {
		generations, total, err := h.webGenerationService.GetByUserID(u.ID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"generations": generations,
			"total":       total,
			"limit":       limit,
			"offset":      offset,
		})
		return
	}

	// Fallback на основной сервис если веб-сервис недоступен
	generations, total, err := h.generationService.GetByUserID(*u.TelegramID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"generations": generations,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

func (h *GenerationHandler) GetGeneration(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Используем WebGenerationService для получения генерации с динамическим статусом
	if h.webGenerationService != nil {
		generation, err := h.webGenerationService.GetByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Generation not found"})
			return
		}

		u := user.(*models.User)
		if generation.UserID != u.ID {
			isAdmin, _ := c.Get("is_admin")
			if isAdmin != true {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}
		}

		c.JSON(http.StatusOK, generation)
		return
	}

	// Fallback на основной сервис если веб-сервис недоступен
	generation, err := h.generationService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Generation not found"})
		return
	}

	u := user.(*models.User)
	if generation.UserID != u.ID {
		isAdmin, _ := c.Get("is_admin")
		if isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	c.JSON(http.StatusOK, generation)
}

func (h *GenerationHandler) GetUserHistory(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	generations, total, err := h.generationService.GetByUserID(u.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   generations,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetPublicGallery returns gallery ideas (public, no auth).
// Query params: sort=all (priority first) or sort=new (newest first, default).
func (h *GenerationHandler) GetPublicGallery(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	sort := c.DefaultQuery("sort", "new")
	if limit > 100 {
		limit = 100
	}

	generations, total, err := h.webGenerationService.GetPublicGallery(limit, offset, sort)
	if err != nil {
		log.Printf("GetPublicGallery error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get gallery"})
		return
	}

	type galleryItem struct {
		ID       int64   `json:"id"`
		Model    string  `json:"model"`
		Output   *string `json:"output"`
		Prompt   string  `json:"prompt"`
	}
	items := make([]galleryItem, 0, len(generations))
	for _, g := range generations {
		items = append(items, galleryItem{ID: g.ID, Model: g.Model, Output: g.Output, Prompt: g.Prompt})
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// GetPublicTrends returns trends (public, no auth).
func (h *GenerationHandler) GetPublicTrends(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	trends, total, err := h.generationService.GetPublicTrends(limit, offset)
	if err != nil {
		log.Printf("GetPublicTrends error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trends"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": trends, "total": total})
}

func (h *GenerationHandler) GetModels(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}
	models := h.webGenerationService.GetAvailableModels()
	c.JSON(http.StatusOK, models)
}

func (h *GenerationHandler) CreateGeneration(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	var req services.CreateGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateGeneration bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Валидация запроса
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model is required"})
		return
	}
	if req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt is required"})
		return
	}

	// Логируем информацию о запросе для отладки
	log.Printf("CreateGeneration request: model=%s, prompt_len=%d, image_urls_count=%d",
		req.Model, len(req.Prompt), len(req.ImageURLs))

	// Проверяем валидность URL изображений
	for i, imgURL := range req.ImageURLs {
		if imgURL == "" {
			log.Printf("Empty image URL at index %d", i)
			continue
		}
		// Проверяем что это base64 или URL
		if len(imgURL) > 100 && (strings.HasPrefix(imgURL, "data:image/") ||
			strings.HasPrefix(imgURL, "/9j/") || strings.HasPrefix(imgURL, "iVBORw0")) {
			log.Printf("Base64 image detected at index %d, length: %d", i, len(imgURL))
		} else {
			log.Printf("Image URL at index %d: %s", i, imgURL[:min(100, len(imgURL))])
		}
	}

	// Проверяем подписку для текстовых моделей
	// Все текстовые модели видны, но доступны по подпискам
	textModelsRequireSubscription := map[string][]string{
		"google/gemini-3-flash": {"start", "pro"},
		"openai/gpt-5-nano":     {"mini", "start", "pro"},
	}

	if allowedSubs, requiresSubscription := textModelsRequireSubscription[req.Model]; requiresSubscription {
		if u.SubscriptionType == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Subscription required for this model"})
			return
		}
		// Проверяем, доступна ли модель для подписки
		found := false
		for _, sub := range allowedSubs {
			if sub == u.SubscriptionType {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "This model is not available for your subscription"})
			return
		}
	}

	genReq, err := h.webGenerationService.CreateGeneration(u.ID, u.Username, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, genReq)
}

func (h *GenerationHandler) GetGenerationStatus(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	genReq, err := h.webGenerationService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Generation not found"})
		return
	}

	c.JSON(http.StatusOK, genReq)
}

func (h *GenerationHandler) HandleKieAPICallback(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	var payload kieapi.CallbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		// Try to parse raw body
		c.Request.Body = io.NopCloser(io.Reader(nil))
	}

	// Re-parse from body
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid callback payload"})
		return
	}

	if err := h.webGenerationService.HandleCallback(payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *GenerationHandler) HandleSunoCallback(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generation service not available"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid callback payload"})
		return
	}

	if err := h.webGenerationService.HandleSunoCallback(payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UploadImage handles temporary image uploads
func (h *GenerationHandler) UploadImage(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	// Get file from request
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Validate file size (max 10MB)
	if header.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 10MB)"})
		return
	}

	// Validate file type via magic bytes; fallback to multipart Content-Type header
	// (needed for HEIC/AVIF/etc. that http.DetectContentType doesn't recognise)
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	file.Seek(0, 0)

	contentType := http.DetectContentType(buffer[:n])
	if !strings.HasPrefix(contentType, "image/") {
		// Fallback: trust the Content-Type sent by the browser in the multipart part
		if ct := header.Header.Get("Content-Type"); strings.HasPrefix(ct, "image/") {
			contentType = ct
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
			return
		}
	}

	tempImageURL, err := h.webGenerationService.SaveUploadedFile(file, header.Filename, h.uploadDir, h.webBaseURL, u.ID)
	if err != nil {
		log.Printf("Error saving image for user %d: %v", u.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":  tempImageURL,
		"size": header.Size,
	})
}

// UploadVideo handles temporary video uploads (reference videos for motion-control)
func (h *GenerationHandler) UploadVideo(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	file, header, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// 100MB limit for videos
	if header.Size > 100*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 100MB)"})
		return
	}

	// Validate file type via magic bytes
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	file.Seek(0, 0)

	contentType := http.DetectContentType(buffer[:n])
	if !strings.HasPrefix(contentType, "video/") {
		if ct := header.Header.Get("Content-Type"); strings.HasPrefix(ct, "video/") {
			contentType = ct
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type, expected video"})
			return
		}
	}

	url, err := h.webGenerationService.SaveUploadedFile(file, header.Filename, h.uploadDir, h.webBaseURL, u.ID)
	if err != nil {
		log.Printf("Error saving video for user %d: %v", u.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload video"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":  url,
		"size": header.Size,
	})
}

// AdminUploadImage is the same as UploadImage but uses userID=0, so the file
// is saved permanently without being subject to the per-user 10-file cleanup.
// Used for gallery-idea images that must persist indefinitely.
func (h *GenerationHandler) AdminUploadImage(c *gin.Context) {
	if h.webGenerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service not available"})
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	if header.Size > 20*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 20MB)"})
		return
	}

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	file.Seek(0, 0)
	contentType := http.DetectContentType(buffer[:n])
	if !strings.HasPrefix(contentType, "image/") {
		if ct := header.Header.Get("Content-Type"); strings.HasPrefix(ct, "image/") {
			contentType = ct
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
			return
		}
	}
	_ = contentType

	// userID=0 → skips user_uploads tracking → file never gets auto-deleted
	url, err := h.webGenerationService.SaveUploadedFile(file, header.Filename, h.uploadDir, h.webBaseURL, 0)
	if err != nil {
		log.Printf("AdminUploadImage error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url, "size": header.Size})
}

// GetUserUploads returns previously uploaded images for the current user.
func (h *GenerationHandler) GetUserUploads(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	u := user.(*models.User)

	uploads, err := h.webGenerationService.GetUserUploads(u.ID, 50)
	if err != nil {
		log.Printf("Error getting uploads for user %d: %v", u.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get uploads"})
		return
	}

	if uploads == nil {
		uploads = []map[string]string{}
	}
	c.JSON(http.StatusOK, gin.H{"uploads": uploads})
}
