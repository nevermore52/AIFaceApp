package handlers

import (
	"net/http"
	"strconv"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type GenerationHandler struct {
	generationService *services.GenerationService
}

func NewGenerationHandler(generationService *services.GenerationService) *GenerationHandler {
	return &GenerationHandler{generationService: generationService}
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
	if u.TelegramID == nil || generation.UserID != *u.TelegramID {
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
	if u.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram account not linked"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	generations, total, err := h.generationService.GetByUserID(*u.TelegramID, limit, offset)
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
