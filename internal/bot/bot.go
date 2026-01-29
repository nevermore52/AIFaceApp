package bot

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"telegram-ai-face-bot/internal/config"
	"telegram-ai-face-bot/internal/defapi"
	"telegram-ai-face-bot/internal/models"
	"telegram-ai-face-bot/internal/openrouter"
	"telegram-ai-face-bot/internal/payments"
	"telegram-ai-face-bot/internal/redis"
	"telegram-ai-face-bot/internal/services"
	"telegram-ai-face-bot/pkg/telegram"
)

type Bot struct {
	tgBot             *telegram.Bot
	userService       *services.UserService
	generationService *services.GenerationService
	paymentService    *services.PaymentService
	server            *http.Server
	shutdownOnce      sync.Once
}

func NewBot(cfg *config.Config, db *sql.DB) (*Bot, error) {
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return nil, err
	}

	userService := services.NewUserService(db)
	openRouterClient := openrouter.NewClient(cfg.OpenRouter)
	generationService := services.NewGenerationService(db, openRouterClient)
	if cfg.DefAPI.APIKey != "" && cfg.DefAPI.BaseURL != "" {
		generationService.SetDefAPIClient(defapi.NewClient(cfg.DefAPI.APIKey, cfg.DefAPI.BaseURL))
	}
	paymentProvider := payments.NewPaymentProvider(cfg.Payment)
	paymentService := services.NewPaymentService(paymentProvider, userService)

	tgBot, err := telegram.NewBot(cfg.TelegramToken, userService, generationService, paymentService, redisClient, cfg)
	if err != nil {
		return nil, err
	}

	generationService.SetNotifier(func(chatID int64, req *models.GenerationRequest) {
		tgBot.NotifyGenerationStatus(chatID, req)
	})

	return &Bot{
		tgBot:             tgBot,
		userService:       userService,
		generationService: generationService,
		paymentService:    paymentService,
	}, nil
}

func (b *Bot) Start() error {
	go b.startWebhookServer()
	return b.tgBot.Start()
}

func (b *Bot) Stop() {
	b.shutdownOnce.Do(func() {
		if b.tgBot != nil {
			b.tgBot.Stop()
		}
		if b.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := b.server.Shutdown(ctx); err != nil {
				log.Printf("webhook server shutdown error: %v", err)
			}
		}
	})
}
func (b *Bot) startWebhookServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/yookassa/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Printf("yookassa webhook bad method: %s from %s", r.Method, r.RemoteAddr)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		auth := r.Header.Get("Authorization")
		if err := b.paymentService.HandleYooKassaWebhook(body, auth); err != nil {
			log.Printf("yookassa webhook error: %v", err)
			http.Error(w, "error", http.StatusBadRequest)
			return
		}
		log.Printf("yookassa webhook handled OK from %s", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/defapi/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Printf("defapi callback bad method: %s from %s", r.Method, r.RemoteAddr)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		log.Printf("defapi callback body: %s", string(body))

		var payload defapi.CallbackPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("defapi callback parse error: %v body=%s", err, truncateForLog(string(body), 300))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if payload.TaskID == "" {
			log.Printf("defapi callback missing task_id body=%s", truncateForLog(string(body), 300))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := b.generationService.HandleDefAPICallback(payload); err != nil {
			log.Printf("defapi callback error: %v", err)
			http.Error(w, "error", http.StatusBadRequest)
			return
		}
		log.Printf("defapi callback handled OK task=%s from %s", payload.TaskID, r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/suno/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Printf("suno callback bad method: %s from %s", r.Method, r.RemoteAddr)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		log.Printf("suno callback body: %s", string(body))

		taskID, audioURLs, errMsg, callbackType, code, parseErr := parseSunoCallback(body)
		if parseErr != nil {
			log.Printf("suno callback parse error: %v body=%s", parseErr, truncateForLog(string(body), 300))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if taskID == "" {
			log.Printf("suno callback missing taskID body=%s", truncateForLog(string(body), 300))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(audioURLs) == 0 {
			if callbackType == "text" && code >= 200 && code < 300 {
				log.Printf("suno callback pending text task=%s msg=%s", taskID, truncateForLog(errMsg, 200))
				w.WriteHeader(http.StatusOK)
				return
			}
			if errMsg != "" {
				b.tgBot.HandleSunoError(taskID, errMsg)
				log.Printf("suno callback error task=%s msg=%s", taskID, truncateForLog(errMsg, 200))
				w.WriteHeader(http.StatusOK)
				return
			}
			log.Printf("suno callback skip: empty audio_url for task=%s body=%s", taskID, truncateForLog(string(body), 300))
			w.WriteHeader(http.StatusOK)
			return
		}

		b.tgBot.HandleSunoCallback(taskID, audioURLs)
		log.Printf("suno callback handled OK task=%s from %s", taskID, r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	})

	addr := b.tgBot.Config().Server.Host + ":" + b.tgBot.Config().Server.Port
	server := &http.Server{Addr: addr, Handler: mux}
	b.server = server
	log.Printf("Starting webhook server on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("webhook server error: %v", err)
	}
}

func parseSunoCallback(body []byte) (string, []string, string, string, int, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, "", "", 0, err
	}
	taskID := findStringByKeys(payload, "taskId", "task_id")
	audioURLs := findAllStringsByKeys(payload,
		"audio_url", "audioUrl", "audio",
		"audio_high_url", "audioHighUrl",
		"download_url", "downloadUrl",
		"result_url", "resultUrl",
		"streaming_url", "streamingUrl",
		"preview_url", "previewUrl",
		"url",
	)
	errMsg := findStringByKeys(payload, "msg", "message", "error")
	callbackType := findStringByKeys(payload, "callbackType", "callback_type", "type")
	code := findIntByKeys(payload, "code", "status")
	return taskID, audioURLs, errMsg, callbackType, code, nil
}

func findIntByKeys(v any, keys ...string) int {
	switch val := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				switch n := raw.(type) {
				case float64:
					return int(n)
				case int:
					return n
				case int64:
					return int(n)
				case json.Number:
					if parsed, err := n.Int64(); err == nil {
						return int(parsed)
					}
				case string:
					if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
						return parsed
					}
				}
			}
		}
		for _, vv := range val {
			if res := findIntByKeys(vv, keys...); res != 0 {
				return res
			}
		}
	case []any:
		for _, vv := range val {
			if res := findIntByKeys(vv, keys...); res != 0 {
				return res
			}
		}
	}
	return 0
}

func findStringByKeys(v any, keys ...string) string {
	switch val := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if s, ok2 := raw.(string); ok2 && s != "" {
					return s
				}
			}
		}
		for _, vv := range val {
			if res := findStringByKeys(vv, keys...); res != "" {
				return res
			}
		}
	case []any:
		for _, vv := range val {
			if res := findStringByKeys(vv, keys...); res != "" {
				return res
			}
		}
	}
	return ""
}

func findAllStringsByKeys(v any, keys ...string) []string {
	var res []string
	switch val := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if s, ok2 := raw.(string); ok2 && s != "" {
					res = append(res, s)
				}
			}
		}
		for _, vv := range val {
			res = append(res, findAllStringsByKeys(vv, keys...)...)
		}
	case []any:
		for _, vv := range val {
			res = append(res, findAllStringsByKeys(vv, keys...)...)
		}
	}
	return uniqueStrings(res)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func findFirstHTTP(v any) string {
	switch val := v.(type) {
	case map[string]any:
		for _, vv := range val {
			if u := findFirstHTTP(vv); u != "" {
				return u
			}
		}
	case []any:
		for _, vv := range val {
			if u := findFirstHTTP(vv); u != "" {
				return u
			}
		}
	case string:
		if strings.HasPrefix(val, "http") {
			return val
		}
	}
	return ""
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
