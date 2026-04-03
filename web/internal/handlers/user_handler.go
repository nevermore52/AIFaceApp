package handlers

import (
	"net/http"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService  *services.UserService
	botToken     string
}

func NewUserHandler(userService *services.UserService, botToken string) *UserHandler {
	return &UserHandler{userService: userService, botToken: botToken}
}

func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	// Перезагружаем пользователя из БД чтобы получить актуальную информацию о подписке
	freshUser, err := h.userService.GetUserByID(u.ID)
	if err != nil {
		// Если ошибка при загрузке, возвращаем кэшированного пользователя
		c.JSON(http.StatusOK, user)
		return
	}

	c.JSON(http.StatusOK, freshUser)
}

type UpdateProfileRequest struct {
	Username     string `json:"username"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	LanguageCode string `json:"language_code"`
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	u := user.(*models.User)
	if req.Username != "" {
		u.Username = req.Username
	}
	if req.FirstName != "" {
		u.FirstName = req.FirstName
	}
	if req.LastName != "" {
		u.LastName = req.LastName
	}
	if req.LanguageCode != "" {
		u.LanguageCode = &req.LanguageCode
	}

	if err := h.userService.Update(u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) ClaimChannelBonus(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	u := user.(*models.User)

	subscribed, alreadyClaimed, err := h.userService.ClaimChannelBonus(u, h.botToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"subscribed":     subscribed,
		"already_claimed": alreadyClaimed,
	})
}

func (h *UserHandler) GetQuota(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)
	quotaUserID := u.ID
	if u.TelegramID != nil {
		quotaUserID = *u.TelegramID
	}

	quota, err := h.userService.GetQuota(quotaUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get quota"})
		return
	}

	c.JSON(http.StatusOK, quota)
}
