package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"telegram-ai-face-bot/web/internal/repository"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	userService       *services.UserService
	generationService *services.GenerationService
	paymentService    *services.PaymentService
	webPaymentService *services.WebPaymentService
	settingsRepo      *repository.SettingsRepository
	botWebhookURL     string
}

func NewAdminHandler(
	userService *services.UserService,
	generationService *services.GenerationService,
	paymentService *services.PaymentService,
	webPaymentService *services.WebPaymentService,
	settingsRepo *repository.SettingsRepository,
) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		generationService: generationService,
		paymentService:    paymentService,
		webPaymentService: webPaymentService,
		settingsRepo:      settingsRepo,
		botWebhookURL:     os.Getenv("BOT_WEBHOOK_URL"),
	}
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	period := c.DefaultQuery("period", "all")

	var since time.Time
	now := time.Now().UTC()
	switch period {
	case "day":
		since = now.Add(-24 * time.Hour)
	case "week":
		since = now.Add(-7 * 24 * time.Hour)
	case "month":
		since = now.Add(-30 * 24 * time.Hour)
	}

	genStats, err := h.generationService.GetStatsSince(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generation stats"})
		return
	}

	userStats, err := h.userService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user stats"})
		return
	}

	paymentStats, err := h.paymentService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payment stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"period":      period,
		"generations": genStats,
		"users":       userStats,
		"payments":    paymentStats,
	})
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	search := c.DefaultQuery("search", "")

	users, total, err := h.userService.GetAll(limit, offset, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		IsAdmin   *bool `json:"is_admin"`
		IsBlocked *bool `json:"is_blocked"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	if req.IsBlocked != nil {
		user.IsBlocked = *req.IsBlocked
	}
	if err := h.userService.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) SetUserSubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req struct {
		Plan string `json:"plan"`
		Days int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Plan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request, need plan and days"})
		return
	}
	plan := strings.ToLower(req.Plan)
	if plan != "mini" && plan != "start" && plan != "pro" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plan must be mini, start, or pro"})
		return
	}
	days := req.Days
	if days <= 0 {
		days = 30
	}

	if h.webPaymentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Payment service not configured"})
		return
	}
	if err := h.webPaymentService.ApplySubscription(id, plan, days); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to apply subscription: %v", err)})
		return
	}

	user, _ := h.userService.GetByID(id)
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": user})
}

func (h *AdminHandler) RemoveUserSubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.userService.RemoveSubscription(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove subscription"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) GetTopUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	users, err := h.generationService.GetTopUsers(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (h *AdminHandler) Broadcast(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	if h.botWebhookURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Bot webhook URL not configured"})
		return
	}

	payload, _ := json.Marshal(map[string]string{"text": req.Text})
	url := strings.TrimRight(h.botWebhookURL, "/") + "/admin/broadcast"
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reach bot: %v", err)})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) GetGenerations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}
	generations, total, err := h.generationService.GetAll(limit, offset)
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

func (h *AdminHandler) GetPayments(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}
	payments, total, err := h.paymentService.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   payments,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) GetCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Category management via web is not yet implemented"})
}

func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Category management via web is not yet implemented"})
}

// Gallery Ideas Management

func (h *AdminHandler) GetGalleryIdeas(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}
	ideas, total, err := h.generationService.GetGalleryIdeas(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get gallery ideas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   ideas,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) CreateGalleryIdea(c *gin.Context) {
	var req struct {
		Model    string `json:"model" binding:"required"`
		Output   string `json:"output" binding:"required"`
		Prompt   string `json:"prompt" binding:"required"`
		Priority *int   `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idea, err := h.generationService.CreateGalleryIdea(req.Model, req.Output, req.Prompt, req.Priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gallery idea"})
		return
	}
	c.JSON(http.StatusOK, idea)
}

func (h *AdminHandler) UpdateGalleryIdea(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var req struct {
		Model    string `json:"model" binding:"required"`
		Output   string `json:"output" binding:"required"`
		Prompt   string `json:"prompt" binding:"required"`
		Priority *int   `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.generationService.UpdateGalleryIdea(id, req.Model, req.Output, req.Prompt, req.Priority); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update gallery idea"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gallery idea updated"})
}

func (h *AdminHandler) DeleteGalleryIdea(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.generationService.DeleteGalleryIdea(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete gallery idea"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gallery idea deleted"})
}

// GetGalleryIdeaPriorities returns taken priority slots (optionally excluding one idea by id).
func (h *AdminHandler) GetGalleryIdeaPriorities(c *gin.Context) {
	excludeID, _ := strconv.ParseInt(c.DefaultQuery("exclude_id", "0"), 10, 64)
	taken, err := h.generationService.GetOccupiedPriorities(excludeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get priorities"})
		return
	}
	if taken == nil {
		taken = []int{}
	}
	c.JSON(http.StatusOK, gin.H{"taken": taken})
}

// ─── Trends ──────────────────────────────────────────────────────────────────

func (h *AdminHandler) GetTrends(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 200 {
		limit = 200
	}
	list, total, err := h.generationService.GetTrends(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trends"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "limit": limit, "offset": offset})
}

func (h *AdminHandler) CreateTrend(c *gin.Context) {
	var req struct {
		Title      string `json:"title"`
		Output     string `json:"output" binding:"required"`
		InputVideo string `json:"input_video"`
		Prompt     string `json:"prompt"`
		Model      string `json:"model"`
		IsPopular  bool   `json:"is_popular"`
		Priority   *int   `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.generationService.CreateTrend(req.Title, req.Output, req.Prompt, req.Model, req.IsPopular, req.Priority, req.InputVideo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trend"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *AdminHandler) UpdateTrend(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var req struct {
		Title      string `json:"title"`
		Output     string `json:"output" binding:"required"`
		InputVideo string `json:"input_video"`
		Prompt     string `json:"prompt"`
		Model      string `json:"model"`
		IsPopular  bool   `json:"is_popular"`
		Priority   *int   `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.generationService.UpdateTrend(id, req.Title, req.Output, req.Prompt, req.Model, req.IsPopular, req.Priority, req.InputVideo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trend"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trend updated"})
}

func (h *AdminHandler) DeleteTrend(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.generationService.DeleteTrend(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete trend"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trend deleted"})
}

func (h *AdminHandler) GetTrendPriorities(c *gin.Context) {
	excludeID, _ := strconv.ParseInt(c.DefaultQuery("exclude_id", "0"), 10, 64)
	taken, err := h.generationService.GetOccupiedTrendPriorities(excludeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get priorities"})
		return
	}
	if taken == nil {
		taken = []int{}
	}
	c.JSON(http.StatusOK, gin.H{"taken": taken})
}

// GetMaintenanceStatus возвращает текущий статус режима техработ
func (h *AdminHandler) GetMaintenanceStatus(c *gin.Context) {
	enabled, err := h.settingsRepo.IsMaintenanceMode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get maintenance status"})
		return
	}

	message, err := h.settingsRepo.GetMaintenanceMessage()
	if err != nil {
		message = "Сервис временно недоступен. Ведутся технические работы."
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
		"message": message,
	})
}

// SetMaintenanceMode включает/выключает режим техработ
func (h *AdminHandler) SetMaintenanceMode(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Message string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.settingsRepo.SetMaintenanceMode(req.Enabled, req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set maintenance mode"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Maintenance mode updated"})
}

// GetAPIBalances возвращает балансы всех API
func (h *AdminHandler) GetAPIBalances(c *gin.Context) {
	result := gin.H{}

	// Suno API
	if sunoKey := os.Getenv("SUNO_API_KEY"); sunoKey != "" {
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", "https://api.sunoapi.org/api/v1/generate/credit", nil)
		req.Header.Set("Authorization", "Bearer "+sunoKey)
		req.Header.Set("Accept", "application/json")
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Suno API response: status=%d body=%s", resp.StatusCode, string(body))

			var payload interface{}
			balanceStr := ""
			if json.Unmarshal(body, &payload) == nil {
				if num, ok := findNumberByKeys(payload, "data", "credit", "credits", "balance", "available_credits"); ok {
					balanceStr = fmt.Sprintf("%.0f", num)
				} else if s, ok := findStringByKeys(payload, "data", "credit", "credits", "balance", "available_credits"); ok {
					balanceStr = s
				}
			}

			if balanceStr != "" {
				result["suno"] = gin.H{
					"balance": balanceStr,
					"status":  fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			} else {
				result["suno"] = gin.H{
					"raw":    string(body),
					"status": fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			}
		} else {
			log.Printf("Suno API error: %v", err)
			result["suno"] = gin.H{"error": err.Error()}
		}
	} else {
		log.Printf("Suno API key not set")
		result["suno"] = gin.H{"error": "API key not set"}
	}

	// DefAPI
	if defKey := os.Getenv("DEFAPI_API_KEY"); defKey != "" {
		defURL := os.Getenv("DEFAPI_BASE_URL")
		if defURL == "" {
			defURL = "https://api.defapi.com"
		}
		defURL = strings.TrimRight(defURL, "/")
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", defURL+"/api/user", nil)
		req.Header.Set("Authorization", "Bearer "+defKey)
		req.Header.Set("Accept", "application/json")
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			log.Printf("DefAPI response: status=%d body=%s", resp.StatusCode, string(body))

			var payload interface{}
			balanceStr := ""
			if json.Unmarshal(body, &payload) == nil {
				if s, ok := findStringByKeys(payload, "credit"); ok {
					balanceStr = s
				} else if num, ok := findNumberByKeys(payload, "credit"); ok {
					balanceStr = fmt.Sprintf("%.8f", num)
				}
			}

			if balanceStr != "" {
				result["defapi"] = gin.H{
					"balance": balanceStr,
					"status":  fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			} else {
				result["defapi"] = gin.H{
					"raw":    string(body),
					"status": fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			}
		} else {
			log.Printf("DefAPI error: %v", err)
			result["defapi"] = gin.H{"error": err.Error()}
		}
	} else {
		log.Printf("DefAPI key not set")
		result["defapi"] = gin.H{"error": "API key not set"}
	}

	// KieAPI
	if kieKey := os.Getenv("KIEAPI_API_KEY"); kieKey != "" {
		kieURL := os.Getenv("KIEAPI_BASE_URL")
		if kieURL == "" {
			kieURL = "https://api.klingai.com"
		}
		kieURL = strings.TrimRight(kieURL, "/")
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", kieURL+"/api/v1/chat/credit", nil)
		req.Header.Set("Authorization", "Bearer "+kieKey)
		req.Header.Set("Accept", "application/json")
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			log.Printf("KieAPI response: status=%d body=%s", resp.StatusCode, string(body))

			var payload interface{}
			balanceStr := ""
			if json.Unmarshal(body, &payload) == nil {
				if s, ok := findStringByKeys(payload, "data", "credit", "credits", "balance", "available_credits", "available"); ok {
					balanceStr = s
				} else if num, ok := findNumberByKeys(payload, "data", "credit", "credits", "balance", "available_credits", "available"); ok {
					balanceStr = fmt.Sprintf("%.2f", num)
				}
			}

			if balanceStr != "" {
				result["kieapi"] = gin.H{
					"balance": balanceStr,
					"status":  fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			} else {
				result["kieapi"] = gin.H{
					"raw":    string(body),
					"status": fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			}
		} else {
			log.Printf("KieAPI error: %v", err)
			result["kieapi"] = gin.H{"error": err.Error()}
		}
	} else {
		log.Printf("KieAPI key not set")
		result["kieapi"] = gin.H{"error": "API key not set"}
	}

	c.JSON(http.StatusOK, result)
}

// findStringByKeys ищет первое строковое значение по ключам
func findStringByKeys(v interface{}, keys ...string) (string, bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if s, ok2 := raw.(string); ok2 && s != "" {
					return s, true
				}
			}
		}
		for _, vv := range val {
			if res, ok := findStringByKeys(vv, keys...); ok {
				return res, true
			}
		}
	case []interface{}:
		for _, vv := range val {
			if res, ok := findStringByKeys(vv, keys...); ok {
				return res, true
			}
		}
	}
	return "", false
}

// findNumberByKeys ищет первое числовое значение по ключам (float64/числовая строка)
func findNumberByKeys(v interface{}, keys ...string) (float64, bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if n, okNum := toFloat(raw); okNum {
					return n, true
				}
			}
		}
		for _, vv := range val {
			if n, ok := findNumberByKeys(vv, keys...); ok {
				return n, true
			}
		}
	case []interface{}:
		for _, vv := range val {
			if n, ok := findNumberByKeys(vv, keys...); ok {
				return n, true
			}
		}
	}
	return 0, false
}

// toFloat конвертирует interface{} в float64
func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
