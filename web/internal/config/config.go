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

	JWTSecret     string
	JWTExpireHours int

	TelegramBotToken  string
	TelegramBotName   string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	FrontendURL string
	Environment string
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
