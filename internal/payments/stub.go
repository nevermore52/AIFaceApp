package payments

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"telegram-ai-face-bot/internal/config"
)

type PaymentProvider struct {
	cfg        config.PaymentConfig
	httpClient *http.Client
}

type PaymentRequest struct {
	UserID   int64          `json:"user_id"`
	Amount   float64        `json:"amount"`
	Currency string         `json:"currency"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type PaymentResponse struct {
	Success      bool   `json:"success"`
	PaymentID    string `json:"payment_id"`
	CheckoutURL  string `json:"checkout_url,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCancelled PaymentStatus = "cancelled"
)

func NewPaymentProvider(cfg config.PaymentConfig) *PaymentProvider {
	return &PaymentProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

func (p *PaymentProvider) CreatePayment(req PaymentRequest) (*PaymentResponse, error) {
	if p.cfg.Provider == "youkassa" || p.cfg.Provider == "yookassa" {
		return p.createYooKassaPayment(req)
	}

	return nil, fmt.Errorf("unsupported payment provider: %s", p.cfg.Provider)
}

func (p *PaymentProvider) GetPaymentStatus(paymentID string) (PaymentStatus, error) {
	return "", fmt.Errorf("get payment status not implemented for provider %s", p.cfg.Provider)
}

func (p *PaymentProvider) ProcessWebhook(payload []byte, signature string) error {
	return fmt.Errorf("webhook processing not implemented for provider %s", p.cfg.Provider)
}

func (p *PaymentProvider) RefundPayment(paymentID string, amount float64) error {
	return fmt.Errorf("refund not implemented for provider %s", p.cfg.Provider)
}

func (p *PaymentProvider) GetSupportedCurrencies() []string {
	return []string{"RUB"}
}

func (p *PaymentProvider) ValidatePaymentAmount(amount float64, currency string) bool {
	if currency != "RUB" {
		return false
	}
	minAmount := 5.0
	maxAmount := 100000.0
	return amount >= minAmount && amount <= maxAmount
}

func (p *PaymentProvider) createYooKassaPayment(req PaymentRequest) (*PaymentResponse, error) {
	if p.cfg.YooKassaShop == "" || p.cfg.YooKassaKey == "" {
		return nil, fmt.Errorf("yookassa credentials are not configured")
	}

	body := map[string]any{
		"amount": map[string]string{
			"value":    fmt.Sprintf("%.2f", req.Amount),
			"currency": "RUB",
		},
		"capture":     true,
		"description": "Покупка доп. запросов",
		"confirmation": map[string]string{
			"type":       "redirect",
			"return_url": p.cfg.YooReturnURL,
		},
		"metadata": req.Metadata,
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
	httpReq.Header.Set("Idempotence-Key", fmt.Sprintf("yk-%d", time.Now().UnixNano()))
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", p.cfg.YooKassaShop, p.cfg.YooKassaKey)))
	httpReq.Header.Set("Authorization", "Basic "+auth)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Confirmation struct {
			Type            string `json:"type"`
			ConfirmationURL string `json:"confirmation_url"`
		} `json:"confirmation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode yookassa response: %w", err)
	}
	if parsed.Confirmation.ConfirmationURL == "" || parsed.ID == "" {
		return nil, fmt.Errorf("yookassa response incomplete")
	}

	return &PaymentResponse{
		Success:     true,
		PaymentID:   parsed.ID,
		CheckoutURL: parsed.Confirmation.ConfirmationURL,
	}, nil
}

type YooWebhookData struct {
	PaymentID string
	UserID    int64
	Category  string
	Qty       int
}

func (p *PaymentProvider) ParseYooKassaWebhook(body []byte, authHeader string) (*YooWebhookData, error) {
	if p.cfg.Provider != "youkassa" && p.cfg.Provider != "yookassa" {
		return nil, fmt.Errorf("yookassa provider disabled")
	}
	if p.cfg.YooKassaShop == "" || p.cfg.YooKassaKey == "" {
		return nil, fmt.Errorf("yookassa credentials missing")
	}

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", p.cfg.YooKassaShop, p.cfg.YooKassaKey)))
	if strings.TrimSpace(authHeader) != "" && strings.TrimSpace(authHeader) != expected {
		return nil, fmt.Errorf("unauthorized webhook")
	}

	var payload struct {
		Event  string `json:"event"`
		Object struct {
			ID       string         `json:"id"`
			Status   string         `json:"status"`
			Paid     bool           `json:"paid"`
			Metadata map[string]any `json:"metadata"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	if payload.Event != "payment.succeeded" && payload.Object.Status != "succeeded" && !payload.Object.Paid {
		return nil, fmt.Errorf("payment not succeeded")
	}

	md := payload.Object.Metadata
	if md == nil {
		return nil, fmt.Errorf("metadata missing")
	}

	uid, ok := md["user_id"]
	if !ok {
		return nil, fmt.Errorf("user_id missing")
	}
	userID, err := parseInt64(uid)
	if err != nil || userID <= 0 {
		return nil, fmt.Errorf("invalid user_id: %v", err)
	}

	qtyVal, ok := md["qty"]
	if !ok {
		return nil, fmt.Errorf("qty missing")
	}
	qty, err := parseInt64(qtyVal)
	if err != nil || qty <= 0 {
		return nil, fmt.Errorf("invalid qty: %v", err)
	}

	cat, _ := md["category"].(string)
	cat = strings.ToLower(cat)
	if cat == "" {
		return nil, fmt.Errorf("category missing")
	}

	return &YooWebhookData{
		PaymentID: payload.Object.ID,
		UserID:    userID,
		Category:  cat,
		Qty:       int(qty),
	}, nil
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
