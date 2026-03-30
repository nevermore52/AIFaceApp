package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"telegram-ai-face-bot/web/internal/config"
	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthHandler struct {
	authService   *services.AuthService
	cfg           *config.Config
	googleOAuth   *oauth2.Config
	authTokenRepo *repository.AuthTokenRepository
}

func NewAuthHandler(authService *services.AuthService, cfg *config.Config, authTokenRepo *repository.AuthTokenRepository) *AuthHandler {
	var googleOAuth *oauth2.Config
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		googleOAuth = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}

	return &AuthHandler{
		authService:   authService,
		cfg:           cfg,
		googleOAuth:   googleOAuth,
		authTokenRepo: authTokenRepo,
	}
}

type TelegramLoginRequest struct {
	ID        int64  `json:"id" binding:"required"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date" binding:"required"`
	Hash      string `json:"hash" binding:"required"`
}

type MiniAppLoginRequest struct {
	InitData string `json:"init_data" binding:"required"`
}

type AuthResponse struct {
	User         interface{} `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
}

func (h *AuthHandler) TelegramLogin(c *gin.Context) {
	var req TelegramLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	authData := services.TelegramAuthData{
		ID:        req.ID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Username:  req.Username,
		PhotoURL:  req.PhotoURL,
		AuthDate:  req.AuthDate,
		Hash:      req.Hash,
	}

	if err := h.authService.ValidateTelegramAuth(authData); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Telegram auth data"})
		return
	}

	user, accessToken, refreshToken, err := h.authService.LoginWithTelegram(
		authData,
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *AuthHandler) TelegramMiniAppLogin(c *gin.Context) {
	var req MiniAppLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	authData, err := h.authService.ValidateTelegramMiniApp(req.InitData)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Mini App data"})
		return
	}

	user, accessToken, refreshToken, err := h.authService.LoginWithTelegram(
		*authData,
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	if h.googleOAuth == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Google OAuth not configured"})
		return
	}

	// Use simple state for now - linking will be handled via separate flow
	url := h.googleOAuth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if h.googleOAuth == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Google OAuth not configured"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code is required"})
		return
	}

	token, err := h.googleOAuth.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to exchange token"})
		return
	}

	client := h.googleOAuth.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user info"})
		return
	}

	var avatarURL *string
	if userInfo.Picture != "" {
		avatarURL = &userInfo.Picture
	}

	user, accessToken, refreshToken, err := h.authService.LoginWithGoogle(
		userInfo.Email,
		userInfo.GivenName,
		userInfo.FamilyName,
		avatarURL,
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed", "details": err.Error()})
		return
	}

	redirectURL := h.cfg.FrontendURL + "/auth/callback?token=" + accessToken + "&refresh=" + refreshToken
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)

	_ = user
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	accessToken, refreshToken, err := h.authService.RefreshToken(
		req.RefreshToken,
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token found"})
		return
	}

	if err := h.authService.Logout(token.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// CreateWebToken generates a one-time auth token for Telegram deep-link login
func (h *AuthHandler) CreateWebToken(c *gin.Context) {
	// Cleanup old expired tokens
	_ = h.authTokenRepo.CleanupExpired()

	// Check if user is authenticated (for linking)
	var userID *int64
	actionType := "auth"

	if user, exists := c.Get("user"); exists {
		u := user.(*models.User)
		userID = &u.ID
		actionType = "link"
	}

	t, err := h.authTokenRepo.CreateWithAction(actionType, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create auth token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": t.Token,
	})
}

// GetWebTokenStatus checks if the auth token has been confirmed by the bot
func (h *AuthHandler) GetWebTokenStatus(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	t, err := h.authTokenRepo.GetByToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	if h.authTokenRepo.IsExpired(t) {
		c.JSON(http.StatusOK, gin.H{"status": "expired"})
		return
	}

	if t.Status == "confirmed" && t.AccessToken != nil && t.RefreshToken != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":        "confirmed",
			"access_token":  *t.AccessToken,
			"refresh_token": *t.RefreshToken,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "pending"})
}

// ConfirmWebToken is called by the Telegram bot to confirm login
type ConfirmWebTokenRequest struct {
	Token      string `json:"token" binding:"required"`
	TelegramID int64  `json:"telegram_id" binding:"required"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	PhotoURL   string `json:"photo_url"`
}

func (h *AuthHandler) ConfirmWebToken(c *gin.Context) {
	// Verify internal secret
	secret := c.GetHeader("X-Bot-Secret")
	if secret == "" || secret != h.cfg.TelegramBotToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req ConfirmWebTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Check token exists and is pending
	t, err := h.authTokenRepo.GetByToken(req.Token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	if h.authTokenRepo.IsExpired(t) {
		c.JSON(http.StatusGone, gin.H{"error": "Token expired"})
		return
	}

	if t.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Token already used"})
		return
	}

	// Check if this is a linking action
	if t.ActionType == "link" && t.UserID != nil {
		// Link Telegram account to existing user
		if err := h.authService.LinkTelegramAccount(*t.UserID, req.TelegramID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link account", "details": err.Error()})
			return
		}

		// Mark token as confirmed without creating new session
		if err := h.authTokenRepo.Confirm(req.Token, req.TelegramID, "", ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "linked",
			"message": "Telegram account linked successfully",
		})
		return
	}

	// Regular auth flow - create user + session
	authData := services.TelegramAuthData{
		ID:        req.TelegramID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Username:  req.Username,
		PhotoURL:  req.PhotoURL,
		AuthDate:  0,  // not validated for bot-confirmed flow
		Hash:      "", // not validated for bot-confirmed flow
	}

	user, accessToken, refreshToken, err := h.authService.LoginWithTelegram(
		authData,
		"telegram-bot",
		"bot-confirmed",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	// Store tokens in the auth token record
	if err := h.authTokenRepo.Confirm(req.Token, req.TelegramID, accessToken, refreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "confirmed",
		"user":   user,
	})
}

// LinkGoogleAccount links a Google account to the current authenticated user
func (h *AuthHandler) LinkGoogleAccount(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)

	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.authService.LinkGoogleAccount(u.ID, req.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link Google account", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Google account linked successfully",
	})
}
