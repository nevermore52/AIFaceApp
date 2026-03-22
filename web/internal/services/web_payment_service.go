package services

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WebPaymentService struct {
	db            *sql.DB
	shopID        string
	secretKey     string
	returnURL     string
	httpClient    *http.Client
}

func NewWebPaymentService(db *sql.DB, shopID, secretKey, returnURL string) *WebPaymentService {
	return &WebPaymentService{
		db:         db,
		shopID:     shopID,
		secretKey:  secretKey,
		returnURL:  returnURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type PackageInfo struct {
	Category string  `json:"category"`
	Qty      int     `json:"qty"`
	Price    float64 `json:"price"`
}

func (s *WebPaymentService) GetPackages() []PackageInfo {
	return []PackageInfo{
		{Category: "image", Qty: 10, Price: 99},
		{Category: "image", Qty: 50, Price: 399},
		{Category: "image", Qty: 100, Price: 699},
		{Category: "video", Qty: 5, Price: 199},
		{Category: "video", Qty: 20, Price: 699},
		{Category: "video", Qty: 50, Price: 1499},
		{Category: "music", Qty: 10, Price: 149},
		{Category: "music", Qty: 30, Price: 399},
	}
}

func (s *WebPaymentService) GetPrice(category string, qty int) (float64, bool) {
	for _, pkg := range s.GetPackages() {
		if pkg.Category == category && pkg.Qty == qty {
			return pkg.Price, true
		}
	}
	return 0, false
}

type CreatePaymentRequest struct {
	UserID    int64  `json:"user_id"`
	Category  string `json:"category"`
	Qty       int    `json:"qty"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type CreatePaymentResponse struct {
	PaymentID   string `json:"payment_id"`
	CheckoutURL string `json:"checkout_url"`
	Amount      float64 `json:"amount"`
}

func (s *WebPaymentService) IsConfigured() bool {
	return s.shopID != "" && s.secretKey != ""
}

func (s *WebPaymentService) CreatePayment(req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("YooKassa not configured")
	}

	price, ok := s.GetPrice(req.Category, req.Qty)
	if !ok {
		return nil, fmt.Errorf("invalid package: %s x %d", req.Category, req.Qty)
	}

	description := fmt.Sprintf("Покупка %d %s запросов", req.Qty, getCategoryName(req.Category))

	body := map[string]any{
		"amount": map[string]string{
			"value":    fmt.Sprintf("%.2f", price),
			"currency": "RUB",
		},
		"capture":     true,
		"description": description,
		"confirmation": map[string]string{
			"type":       "redirect",
			"return_url": s.returnURL,
		},
		"metadata": map[string]any{
			"user_id":    req.UserID,
			"category":   req.Category,
			"qty":        req.Qty,
			"username":   req.Username,
			"first_name": req.FirstName,
			"last_name":  req.LastName,
			"source":     "web",
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", "https://api.yookassa.ru/v3/payments", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", fmt.Sprintf("web-%d-%d", req.UserID, time.Now().UnixNano()))
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", s.shopID, s.secretKey)))
	httpReq.Header.Set("Authorization", "Basic "+auth)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("YooKassa error: %s", string(respBody))
	}

	var parsed struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Confirmation struct {
			ConfirmationURL string `json:"confirmation_url"`
		} `json:"confirmation"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse YooKassa response: %w", err)
	}

	if parsed.Confirmation.ConfirmationURL == "" || parsed.ID == "" {
		return nil, fmt.Errorf("YooKassa response incomplete")
	}

	return &CreatePaymentResponse{
		PaymentID:   parsed.ID,
		CheckoutURL: parsed.Confirmation.ConfirmationURL,
		Amount:      price,
	}, nil
}

type WebhookData struct {
	PaymentID string
	UserID    int64
	Category  string
	Qty       int
	Amount    float64
	Username  string
	FirstName string
	LastName  string
}

func (s *WebPaymentService) ParseWebhook(body []byte) (*WebhookData, error) {
	var payload struct {
		Event  string `json:"event"`
		Object struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Paid   bool   `json:"paid"`
			Amount struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"amount"`
			Metadata map[string]any `json:"metadata"`
		} `json:"object"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	if payload.Event != "payment.succeeded" && payload.Object.Status != "succeeded" && !payload.Object.Paid {
		return nil, fmt.Errorf("payment not succeeded: event=%s status=%s", payload.Event, payload.Object.Status)
	}

	md := payload.Object.Metadata
	if md == nil {
		return nil, fmt.Errorf("metadata missing")
	}

	userID, err := parseInt64(md["user_id"])
	if err != nil || userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	qty, err := parseInt64(md["qty"])
	if err != nil || qty <= 0 {
		return nil, fmt.Errorf("invalid qty")
	}

	category, _ := md["category"].(string)
	if category == "" {
		return nil, fmt.Errorf("category missing")
	}

	username, _ := md["username"].(string)
	firstName, _ := md["first_name"].(string)
	lastName, _ := md["last_name"].(string)

	amount := 0.0
	if v := strings.TrimSpace(payload.Object.Amount.Value); v != "" {
		amount, _ = strconv.ParseFloat(v, 64)
	}

	return &WebhookData{
		PaymentID: payload.Object.ID,
		UserID:    userID,
		Category:  category,
		Qty:       int(qty),
		Amount:    amount,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
	}, nil
}

func (s *WebPaymentService) AddQuota(userID int64, category string, qty int) error {
	var column string
	switch category {
	case "image":
		column = "image_extra"
	case "video":
		column = "video_extra"
	case "music":
		column = "music_extra"
	case "text":
		column = "text_extra"
	default:
		return fmt.Errorf("unknown category: %s", category)
	}

	query := fmt.Sprintf(`UPDATE user_quotas SET %s = %s + $2 WHERE user_id = $1`, column, column)
	result, err := s.db.Exec(query, userID, qty)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		insertQuery := fmt.Sprintf(`INSERT INTO user_quotas (user_id, %s) VALUES ($1, $2) ON CONFLICT (user_id) DO UPDATE SET %s = user_quotas.%s + $2`, column, column, column)
		_, err = s.db.Exec(insertQuery, userID, qty)
		return err
	}
	return nil
}

func (s *WebPaymentService) RecordPayment(data *WebhookData) error {
	query := `INSERT INTO payments (user_id, payment_id, category, qty, amount, status, created_at) VALUES ($1, $2, $3, $4, $5, 'completed', NOW())`
	_, err := s.db.Exec(query, data.UserID, data.PaymentID, data.Category, data.Qty, data.Amount)
	return err
}

func getCategoryName(category string) string {
	switch category {
	case "image":
		return "изображений"
	case "video":
		return "видео"
	case "music":
		return "музыки"
	case "text":
		return "текстовых"
	default:
		return category
	}
}

func parseInt64(v any) (int64, error) {
	switch val := v.(type) {
	case float64:
		return int64(val), nil
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case json.Number:
		return val.Int64()
	case string:
		return strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}
