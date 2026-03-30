package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"telegram-ai-face-bot/web/internal/kieapi"
	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type GenerationHandler struct {
	generationService    *services.GenerationService
	webGenerationService *services.WebGenerationService
}

func NewGenerationHandler(generationService *services.GenerationService, webGenerationService *services.WebGenerationService) *GenerationHandler {
	return &GenerationHandler{
		generationService:    generationService,
		webGenerationService: webGenerationService,
	}
}

func (h *GenerationHandler) GetUserGenerations(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)
	if u.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram account not linked"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	generations, total, err := h.generationService.GetByUserID(*u.TelegramID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   generations,
		"total":  total,
		"limit":  limit,
		"offset": offset,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем подписку для текстовых моделей
	textModels := map[string]bool{
		"google/gemini-3-flash": true,
		"openai/gpt-5-mini":     true,
		"openai/gpt-5-nano":     true,
		"chat-gpt-4.1mini":      true,
	}

	if textModels[req.Model] {
		if u.SubscriptionType == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Subscription required for text models"})
			return
		}
		// Проверяем, доступна ли модель для подписки
		allowedModels := map[string][]string{
			"google/gemini-3-flash": {"start", "pro"},
			"openai/gpt-5-mini":     {"mini", "start", "pro"},
			"openai/gpt-5-nano":     {"mini", "start", "pro"},
			"chat-gpt-4.1mini":      {"start", "pro"},
		}

		if allowed, ok := allowedModels[req.Model]; ok {
			found := false
			for _, sub := range allowed {
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
