package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
	DatabaseURL   string
	Redis         RedisConfig
	OpenRouter    OpenRouterConfig
	DefAPI        DefAPIConfig
	Payment       PaymentConfig
	AdminIDs      []int64
	Server        ServerConfig
	DebugLogging  bool
}

type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

type OpenRouterConfig struct {
	APIKey       string
	BaseURL      string
	Model        string
	DebugLogging bool
}

type DefAPIConfig struct {
	APIKey  string
	BaseURL string
}

type PaymentConfig struct {
	Provider      string
	APIKey        string
	WebhookSecret string
	YooKassaShop  string
	YooKassaKey   string
	YooReturnURL  string
}

type ServerConfig struct {
	Port string
	Host string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.TelegramToken = getEnv("TELEGRAM_BOT_TOKEN", "")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "postgres://aifacebot_user:aifacebot_password@localhost:5432/aifacebot?sslmode=disable")
	cfg.Redis = RedisConfig{
		URL:      getEnv("REDIS_URL", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getEnvInt("REDIS_DB", 0),
	}

	cfg.OpenRouter = OpenRouterConfig{
		APIKey:       getEnv("PIAPI_API_KEY", ""),
		BaseURL:      getEnv("PIAPI_BASE_URL", "https://api.piapi.ai/api/v1"),
		Model:        getEnv("OPENROUTER_IMAGE_MODEL", "gemini"),
		DebugLogging: getEnvBool("DEBUG_LOGGING", true),
	}

	cfg.DefAPI = DefAPIConfig{
		APIKey:  getEnv("DEF_API_KEY", ""),
		BaseURL: getEnv("DEF_BASE_URL", ""),
	}

	cfg.Payment = PaymentConfig{
		Provider:      getEnv("PAYMENT_PROVIDER", "yookassa"),
		APIKey:        getEnv("PAYMENT_API_KEY", ""),
		WebhookSecret: getEnv("PAYMENT_WEBHOOK_SECRET", ""),
		YooKassaShop:  getEnv("YOOKASSA_SHOP_ID", ""),
		YooKassaKey:   getEnv("YOOKASSA_SECRET_KEY", ""),
		YooReturnURL:  getEnv("YOOKASSA_RETURN_URL", "https://t.me/AIFaceApps"),
	}

	adminIDsStr := getEnv("WHITELIST_TELEGRAM_IDS", "")
	if adminIDsStr != "" {
		ids := strings.Split(adminIDsStr, ",")
		for _, id := range ids {
			if parsedID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64); err == nil {
				cfg.AdminIDs = append(cfg.AdminIDs, parsedID)
			}
		}
	}

	cfg.Server = ServerConfig{
		Port: getEnv("SERVER_PORT", "8080"),
		Host: getEnv("SERVER_HOST", "localhost"),
	}

	cfg.DebugLogging = getEnvBool("DEBUG_LOGGING", true)
	if cfg.TelegramToken == "" {
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
