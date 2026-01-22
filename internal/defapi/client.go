package defapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type CreateImageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type CallbackPayload struct {
	TaskID       string         `json:"task_id"`
	Status       string         `json:"status"`
	Result       any            `json:"result"`
	StatusReason map[string]any `json:"status_reason"`
}

func (p CallbackPayload) ResultURL() string {
	if p.Result == nil {
		return ""
	}
	return findFirstHTTP(p.Result)
}

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimSpace(baseURL),
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (c *Client) CreateImageTask(model, prompt string, images []string, callbackURL string, aspectRatio string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("defapi client is nil")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("DEF_API_KEY is not set")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("DEF_BASE_URL is not set")
	}
	if model == "" {
		return "", fmt.Errorf("defapi model is empty")
	}
	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
	}
	if prompt == "" {
		prompt = " "
	}

	payload := map[string]any{
		"model":        model,
		"prompt":       prompt,
		"images":       images,
		"callback_url": callbackURL,
	}
	if aspectRatio != "" {
		payload["aspect_ratio"] = aspectRatio
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal defapi payload: %w", err)
	}

	url := normalizeEndpoint(c.baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create defapi request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("defapi request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("defapi status %d: %s", resp.StatusCode, string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read defapi response: %w", err)
	}

	var parsed CreateImageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse defapi response: %w", err)
	}
	if parsed.Code != 0 {
		return "", fmt.Errorf("defapi error: %s", strings.TrimSpace(parsed.Message))
	}
	if parsed.Data.TaskID == "" {
		return "", fmt.Errorf("defapi response missing task_id")
	}
	return parsed.Data.TaskID, nil
}

func normalizeEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/api/image/gen") || strings.HasSuffix(lower, "/image/generate") || strings.HasSuffix(lower, "/image-generation") || strings.HasSuffix(lower, "/generate") {
		return trimmed
	}
	return trimmed + "/api/image/gen"
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
