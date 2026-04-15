package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServerHost  string
	ServerPort  string
	DatabaseURL string
	RedisURL    string
	RedisPass   string
	RedisDB     int

	JWTSecret      string
	JWTExpireHours int

	TelegramBotToken string
	TelegramBotName  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	FrontendURL string
	Environment string
	WebBaseURL  string
	UploadDir   string

	// KieAPI for image/video generation
	KieAPIKey      string
	KieAPIBaseURL  string
	KieCallbackURL string

	// DefAPI for text generation
	DefAPIKey     string
	DefAPIBaseURL string

	// YooKassa payments
	YooKassaShopID    string
	YooKassaSecretKey string
	YooKassaReturnURL string

}

func Load() (*Config, error) {
	cfg := &Config{
		ServerHost:  getEnv("WEB_SERVER_HOST", "0.0.0.0"),
		ServerPort:  getEnv("WEB_SERVER_PORT", "3000"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://aifacebot_user:aifacebot_password@postgres:5432/aifacebot?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:     getEnvInt("REDIS_DB", 0),

		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production-very-secret-key"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 168), // 7 days

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramBotName:  getEnv("TELEGRAM_BOT_NAME", ""),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		Environment: getEnv("ENVIRONMENT", "development"),
		WebBaseURL:  getEnv("WEB_BASE_URL", ""),
		UploadDir:   getEnv("UPLOAD_DIR", "/app/uploads"),

		KieAPIKey:      getEnv("KIEAPI_API_KEY", ""),
		KieAPIBaseURL:  getEnv("KIEAPI_BASE_URL", "https://api.kie.ai"),
		KieCallbackURL: getEnv("WEB_KIEAPI_CALLBACK_URL", ""),

		DefAPIKey:     getEnv("DEF_API_KEY", ""),
		DefAPIBaseURL: getEnv("DEF_BASE_URL", "https://api.defapi.org"),

		YooKassaShopID:    getEnv("YOOKASSA_SHOP_ID", ""),
		YooKassaSecretKey: getEnv("YOOKASSA_SECRET_KEY", ""),
		YooKassaReturnURL: getEnv("YOOKASSA_RETURN_URL", ""),
	}

	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
