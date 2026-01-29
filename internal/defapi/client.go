package defapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if attempt == 1 {
		return 700 * time.Millisecond
	}
	return 1500 * time.Millisecond
}

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) CreateChatCompletion(model string, messages []map[string]string) (string, error) {
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
	if len(messages) == 0 {
		return "", fmt.Errorf("defapi messages are empty")
	}

	payload := map[string]any{
		"model":             model,
		"messages":          messages,
		"stream":            false,
		"temperature":       0.7,
		"top_p":             1,
		"frequency_penalty": 0,
		"presence_penalty":  0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal defapi chat payload: %w", err)
	}

	base := strings.TrimRight(c.baseURL, "/")
	url := base + "/api/v1/chat/completions"
	log.Printf("[DEFAPI] chat request url=%s payload=%s", url, string(body))

	for attempt := 0; attempt < 3; attempt++ {
		if d := retryDelay(attempt); d > 0 {
			time.Sleep(d)
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create defapi chat request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			log.Printf("[DEFAPI] chat transport error attempt=%d err=%v", attempt+1, err)
			if attempt < 2 {
				continue
			}
			return "", fmt.Errorf("defapi chat request failed: %w", err)
		}

		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt < 2 {
				continue
			}
			return "", fmt.Errorf("read defapi chat response: %w", readErr)
		}
		log.Printf("[DEFAPI] chat response status=%d body=%s", resp.StatusCode, string(raw))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if isRetryableStatus(resp.StatusCode) && attempt < 2 {
				continue
			}
			return "", fmt.Errorf("defapi chat status %d: %s", resp.StatusCode, string(raw))
		}

		var parsed chatCompletionResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", fmt.Errorf("parse defapi chat response: %w", err)
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("defapi chat response has no choices")
		}
		content := strings.TrimSpace(parsed.Choices[0].Message.Content)
		if content == "" {
			return "", fmt.Errorf("defapi chat response empty content")
		}
		return content, nil
	}

	return "", fmt.Errorf("defapi chat request failed after retries")
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
	log.Printf("[DEFAPI] request url=%s payload=%s", url, string(body))

	for attempt := 0; attempt < 3; attempt++ {
		if d := retryDelay(attempt); d > 0 {
			time.Sleep(d)
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create defapi request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			log.Printf("[DEFAPI] transport error attempt=%d err=%v", attempt+1, err)
			if attempt < 2 {
				continue
			}
			return "", fmt.Errorf("defapi request failed: %w", err)
		}

		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt < 2 {
				continue
			}
			return "", fmt.Errorf("read defapi response: %w", readErr)
		}
		log.Printf("[DEFAPI] response status=%d body=%s", resp.StatusCode, string(raw))

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if isRetryableStatus(resp.StatusCode) && attempt < 2 {
				continue
			}
			limited := raw
			if len(limited) > 2048 {
				limited = limited[:2048]
			}
			return "", fmt.Errorf("defapi status %d: %s", resp.StatusCode, string(limited))
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

	return "", fmt.Errorf("defapi request failed after retries")
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
