package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type PromoHandler struct {
	promoRepo   *repository.PromoRepository
	userService *services.UserService
}

func NewPromoHandler(promoRepo *repository.PromoRepository, userService *services.UserService) *PromoHandler {
	return &PromoHandler{promoRepo: promoRepo, userService: userService}
}

// ──────────────────────────────────────────
// Admin endpoints
// ──────────────────────────────────────────

func (h *PromoHandler) AdminList(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 200 {
		limit = 200
	}
	list, total, err := h.promoRepo.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get promo codes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total})
}

func (h *PromoHandler) AdminCreate(c *gin.Context) {
	var req struct {
		Code           string     `json:"code" binding:"required"`
		Description    string     `json:"description"`
		ImageTokens    int        `json:"image_tokens"`
		VideoTokens    int        `json:"video_tokens"`
		TextTokens     int        `json:"text_tokens"`
		MusicTokens    int        `json:"music_tokens"`
		MaxActivations *int       `json:"max_activations"`
		ExpiresAt      *time.Time `json:"expires_at"`
		IsActive       *bool      `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	p := &repository.PromoCode{
		Code:           req.Code,
		Description:    req.Description,
		ImageTokens:    req.ImageTokens,
		VideoTokens:    req.VideoTokens,
		TextTokens:     req.TextTokens,
		MusicTokens:    req.MusicTokens,
		MaxActivations: req.MaxActivations,
		ExpiresAt:      req.ExpiresAt,
		IsActive:       isActive,
	}
	out, err := h.promoRepo.Create(p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create promo code: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *PromoHandler) AdminUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req struct {
		Description    string     `json:"description"`
		ImageTokens    int        `json:"image_tokens"`
		VideoTokens    int        `json:"video_tokens"`
		TextTokens     int        `json:"text_tokens"`
		MusicTokens    int        `json:"music_tokens"`
		MaxActivations *int       `json:"max_activations"`
		ExpiresAt      *time.Time `json:"expires_at"`
		IsActive       bool       `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := &repository.PromoCode{
		ID:             id,
		Description:    req.Description,
		ImageTokens:    req.ImageTokens,
		VideoTokens:    req.VideoTokens,
		TextTokens:     req.TextTokens,
		MusicTokens:    req.MusicTokens,
		MaxActivations: req.MaxActivations,
		ExpiresAt:      req.ExpiresAt,
		IsActive:       req.IsActive,
	}
	if err := h.promoRepo.Update(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update promo code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PromoHandler) AdminDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.promoRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete promo code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ──────────────────────────────────────────
// User endpoint
// ──────────────────────────────────────────

func (h *PromoHandler) Activate(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	u := user.(*models.User)

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите промокод"})
		return
	}

	promo, err := h.promoRepo.GetByCode(req.Code)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Промокод не найден"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при проверке промокода"})
		return
	}

	if !promo.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Промокод неактивен"})
		return
	}
	if promo.ExpiresAt != nil && time.Now().After(*promo.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Срок действия промокода истёк"})
		return
	}
	if promo.MaxActivations != nil && promo.ActivationsCount >= *promo.MaxActivations {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Лимит активаций промокода исчерпан"})
		return
	}

	alreadyUsed, err := h.promoRepo.HasUserActivated(promo.ID, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при проверке активации"})
		return
	}
	if alreadyUsed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Вы уже использовали этот промокод"})
		return
	}

	// Record activation first
	if err := h.promoRepo.Activate(promo.ID, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при активации промокода"})
		return
	}

	// Use telegram_id for quota; fallback to internal ID (same pattern as web gen service)
	quotaUserID := u.ID
	if u.TelegramID != nil {
		quotaUserID = *u.TelegramID
	}

	if promo.ImageTokens > 0 {
		_ = h.userService.AddExtraQuota(quotaUserID, models.QuotaCategoryImage, promo.ImageTokens)
	}
	if promo.VideoTokens > 0 {
		_ = h.userService.AddExtraQuota(quotaUserID, models.QuotaCategoryVideo, promo.VideoTokens)
	}
	if promo.TextTokens > 0 {
		_ = h.userService.AddExtraQuota(quotaUserID, models.QuotaCategoryText, promo.TextTokens)
	}
	if promo.MusicTokens > 0 {
		_ = h.userService.AddExtraQuota(quotaUserID, models.QuotaCategoryMusic, promo.MusicTokens)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Промокод успешно активирован!",
		"image_tokens": promo.ImageTokens,
		"video_tokens": promo.VideoTokens,
		"text_tokens":  promo.TextTokens,
		"music_tokens": promo.MusicTokens,
	})
}
