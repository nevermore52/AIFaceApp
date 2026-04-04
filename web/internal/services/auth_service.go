package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"telegram-ai-face-bot/web/internal/config"
	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthService struct {
	cfg         *config.Config
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
}

func NewAuthService(cfg *config.Config, userRepo *repository.UserRepository, sessionRepo *repository.SessionRepository) *AuthService {
	return &AuthService{
		cfg:         cfg,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

type TelegramAuthData struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

type JWTClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *AuthService) ValidateTelegramAuth(data TelegramAuthData) error {
	if time.Now().Unix()-data.AuthDate > 86400 {
		return fmt.Errorf("auth data expired")
	}

	checkString := fmt.Sprintf("auth_date=%d\nfirst_name=%s\nid=%d",
		data.AuthDate, data.FirstName, data.ID)
	if data.LastName != "" {
		checkString = fmt.Sprintf("auth_date=%d\nfirst_name=%s\nid=%d\nlast_name=%s",
			data.AuthDate, data.FirstName, data.ID, data.LastName)
	}
	if data.PhotoURL != "" {
		checkString += fmt.Sprintf("\nphoto_url=%s", data.PhotoURL)
	}
	if data.Username != "" {
		checkString += fmt.Sprintf("\nusername=%s", data.Username)
	}

	secretKey := sha256.Sum256([]byte(s.cfg.TelegramBotToken))
	h := hmac.New(sha256.New, secretKey[:])
	h.Write([]byte(checkString))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	if expectedHash != data.Hash {
		return fmt.Errorf("invalid hash")
	}

	return nil
}

func (s *AuthService) ValidateTelegramMiniApp(initData string) (*TelegramAuthData, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("invalid init data format")
	}

	hash := values.Get("hash")
	if hash == "" {
		return nil, fmt.Errorf("hash is missing")
	}

	values.Del("hash")

	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataCheckString strings.Builder
	for i, k := range keys {
		if i > 0 {
			dataCheckString.WriteString("\n")
		}
		dataCheckString.WriteString(k)
		dataCheckString.WriteString("=")
		dataCheckString.WriteString(values.Get(k))
	}

	secretKey := hmac.New(sha256.New, []byte("WebAppData"))
	secretKey.Write([]byte(s.cfg.TelegramBotToken))

	h := hmac.New(sha256.New, secretKey.Sum(nil))
	h.Write([]byte(dataCheckString.String()))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	if expectedHash != hash {
		return nil, fmt.Errorf("invalid hash")
	}

	authDateStr := values.Get("auth_date")
	authDate, _ := strconv.ParseInt(authDateStr, 10, 64)
	if time.Now().Unix()-authDate > 86400 {
		return nil, fmt.Errorf("auth data expired")
	}

	userJSON := values.Get("user")
	if userJSON == "" {
		return nil, fmt.Errorf("user data is missing")
	}

	var userData struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
		PhotoURL  string `json:"photo_url"`
	}
	if err := json.Unmarshal([]byte(userJSON), &userData); err != nil {
		return nil, fmt.Errorf("invalid user data: %w", err)
	}

	return &TelegramAuthData{
		ID:        userData.ID,
		FirstName: userData.FirstName,
		LastName:  userData.LastName,
		Username:  userData.Username,
		PhotoURL:  userData.PhotoURL,
		AuthDate:  authDate,
		Hash:      hash,
	}, nil
}

func (s *AuthService) LoginWithTelegram(data TelegramAuthData, userAgent, ipAddress string) (*models.User, string, string, error) {
	user, err := s.userRepo.GetByTelegramID(data.ID)
	if err == sql.ErrNoRows {
		var avatarURL *string
		if data.PhotoURL != "" {
			avatarURL = &data.PhotoURL
		}
		user, err = s.userRepo.CreateFromTelegram(data.ID, data.Username, data.FirstName, data.LastName, "", avatarURL)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, "", "", fmt.Errorf("failed to get user: %w", err)
	}

	accessToken, refreshToken, err := s.createSession(user.ID, userAgent, ipAddress)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

func (s *AuthService) LoginWithGoogle(email, firstName, lastName string, avatarURL *string, userAgent, ipAddress string) (*models.User, string, string, error) {
	user, err := s.userRepo.GetByEmail(email)
	if err == sql.ErrNoRows {
		user, err = s.userRepo.CreateFromGoogle(email, firstName, lastName, avatarURL)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, "", "", fmt.Errorf("failed to get user: %w", err)
	}

	accessToken, refreshToken, err := s.createSession(user.ID, userAgent, ipAddress)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

func (s *AuthService) createSession(userID int64, userAgent, ipAddress string) (string, string, error) {
	expiresAt := time.Now().Add(time.Duration(s.cfg.JWTExpireHours) * time.Hour)

	// Попытка создания сессии с повторами при конфликте токенов
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		accessToken, err := s.generateJWT(userID, expiresAt)
		if err != nil {
			return "", "", err
		}

		refreshToken, err := s.generateJWT(userID, time.Now().Add(30*24*time.Hour))
		if err != nil {
			return "", "", err
		}

		session := &models.WebSession{
			UserID:       userID,
			Token:        accessToken,
			RefreshToken: refreshToken,
			UserAgent:    userAgent,
			IPAddress:    ipAddress,
			ExpiresAt:    expiresAt,
		}

		err = s.sessionRepo.Create(session)
		if err == nil {
			return accessToken, refreshToken, nil
		}

		// Если ошибка дубликата токена, пробуем еще раз
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "web_sessions_token_key") {
			if attempt < maxRetries-1 {
				time.Sleep(time.Millisecond * 10) // Небольшая задержка перед повтором
				continue
			}
		}

		return "", "", fmt.Errorf("failed to create session: %w", err)
	}

	return "", "", fmt.Errorf("failed to create session after %d attempts", maxRetries)
}

func (s *AuthService) generateJWT(userID int64, expiresAt time.Time) (string, error) {
	// Генерируем уникальный ID для токена, чтобы избежать дубликатов
	jti := uuid.New().String()

	claims := JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	session, err := s.sessionRepo.GetByToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}

	user, err := s.userRepo.GetByID(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.ID != claims.UserID {
		return nil, fmt.Errorf("token user mismatch")
	}

	return user, nil
}

func (s *AuthService) RefreshToken(refreshToken, userAgent, ipAddress string) (string, string, error) {
	session, err := s.sessionRepo.GetByRefreshToken(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("invalid refresh token")
	}

	s.sessionRepo.Delete(session.Token)

	return s.createSession(session.UserID, userAgent, ipAddress)
}

func (s *AuthService) Logout(token string) error {
	return s.sessionRepo.Delete(token)
}

func (s *AuthService) GetUserByID(userID int64) (*models.User, error) {
	return s.userRepo.GetByID(userID)
}

func (s *AuthService) IsAdmin(userID int64) (bool, error) {
	return s.userRepo.IsAdmin(userID)
}

func (s *AuthService) LinkTelegramAccount(userID int64, telegramID int64, username, firstName, lastName string) error {
	// Load current user to check their state
	currentUser, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("current user not found: %w", err)
	}
	// Refuse if current account already has a Telegram linked
	if currentUser.TelegramID != nil {
		return fmt.Errorf("account already linked to telegram")
	}

	// Check if an account with this telegram_id already exists
	existingUser, err := s.userRepo.GetByTelegramID(telegramID)
	if err == nil && existingUser.ID != userID {
		// Refuse if the TG account is already linked to a Google account — two full accounts, no merge
		if existingUser.Email != nil {
			return fmt.Errorf("telegram account already linked to another google account")
		}
		// TG-only account exists — merge: keep current (Google) user, transfer TG data over
		if err := s.userRepo.MergeAccounts(userID, existingUser.ID); err != nil {
			return fmt.Errorf("failed to merge accounts: %w", err)
		}
	} else if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing telegram account: %w", err)
	}

	if err := s.userRepo.LinkTelegramID(userID, telegramID, username, firstName, lastName); err != nil {
		return fmt.Errorf("failed to link telegram account: %w", err)
	}

	return nil
}

func (s *AuthService) LinkGoogleAccount(userID int64, email string) error {
	// Load current user to check their state
	currentUser, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("current user not found: %w", err)
	}
	// Refuse if current account already has a Google account linked
	if currentUser.Email != nil {
		return fmt.Errorf("account already linked to google")
	}

	// Check if an account with this email already exists
	existingUser, err := s.userRepo.GetByEmail(email)
	if err == nil && existingUser.ID != userID {
		// Refuse if the Google account is already linked to a Telegram account — two full accounts, no merge
		if existingUser.TelegramID != nil {
			return fmt.Errorf("google account already linked to another telegram account")
		}
		// Google-only account exists — merge: keep current (TG) user, transfer Google data over
		if err := s.userRepo.MergeAccounts(userID, existingUser.ID); err != nil {
			return fmt.Errorf("failed to merge accounts: %w", err)
		}
	} else if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing google account: %w", err)
	}

	if err := s.userRepo.LinkEmail(userID, email); err != nil {
		return fmt.Errorf("failed to link google account: %w", err)
	}

	return nil
}
