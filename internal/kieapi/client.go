package kieapi

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

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

type CreateTaskRequest struct {
	Model       string         `json:"model"`
	CallBackURL string         `json:"callBackUrl"`
	Input       map[string]any `json:"input"`
}

type CreateTaskResponse struct {
	RawBody string `json:"-"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Msg     string `json:"msg"`
	Data    struct {
		TaskID    string `json:"taskId"`
		TaskIDAlt string `json:"task_id"`
	} `json:"data"`
}

type CallbackPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID     string `json:"taskId"`
		TaskIDAlt  string `json:"task_id"`
		State      string `json:"state"`
		Status     string `json:"status"`
		Result     any    `json:"result"`
		ResultJson string `json:"resultJson"`
	} `json:"data"`
}

func (p CallbackPayload) TaskIDValue() string {
	id := strings.TrimSpace(p.Data.TaskID)
	if id == "" {
		id = strings.TrimSpace(p.Data.TaskIDAlt)
	}
	return id
}

func (p CallbackPayload) StatusValue() string {
	st := strings.TrimSpace(p.Data.State)
	if st == "" {
		st = strings.TrimSpace(p.Data.Status)
	}
	return st
}

func (p CallbackPayload) ResultURL() string {
	if u := findFirstHTTP(p.Data.Result); u != "" {
		return u
	}
	raw := strings.TrimSpace(p.Data.ResultJson)
	if raw == "" {
		return ""
	}
	var parsed struct {
		ResultUrls []string `json:"resultUrls"`
		ResultURLs []string `json:"result_urls"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ""
	}
	if len(parsed.ResultUrls) > 0 {
		return strings.TrimSpace(parsed.ResultUrls[0])
	}
	if len(parsed.ResultURLs) > 0 {
		return strings.TrimSpace(parsed.ResultURLs[0])
	}
	return ""
}

func (c *Client) CreateTask(req CreateTaskRequest) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kieapi client is nil")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("KIEAPI_API_KEY is not set")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("KIEAPI_BASE_URL is not set")
	}
	if strings.TrimSpace(req.Model) == "" {
		return "", fmt.Errorf("kieapi model is empty")
	}
	if strings.TrimSpace(req.CallBackURL) == "" {
		return "", fmt.Errorf("kieapi callback url is empty")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal kieapi payload: %w", err)
	}

	url := c.baseURL + "/api/v1/jobs/createTask"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create kieapi request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("kieapi request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read kieapi response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := raw
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("kieapi status %d: %s", resp.StatusCode, string(limited))
	}

	var parsed CreateTaskResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse kieapi response: %w", err)
	}
	parsed.RawBody = string(raw)

	msg := strings.TrimSpace(parsed.Message)
	if msg == "" {
		msg = strings.TrimSpace(parsed.Msg)
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		limited := parsed.RawBody
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("kieapi error: code=%d message=%s raw=%s", parsed.Code, msg, limited)
	}
	if strings.TrimSpace(parsed.Data.TaskID) == "" {
		parsed.Data.TaskID = strings.TrimSpace(parsed.Data.TaskIDAlt)
	}
	if strings.TrimSpace(parsed.Data.TaskID) == "" {
		limited := parsed.RawBody
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("kieapi response missing taskId: raw=%s", limited)
	}
	return parsed.Data.TaskID, nil
}

// CreateVeoTask отправляет запрос на /api/v1/veo/generate с плоским payload.
func (c *Client) CreateVeoTask(payload map[string]any) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kieapi client is nil")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("KIEAPI_API_KEY is not set")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("KIEAPI_BASE_URL is not set")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal veo payload: %w", err)
	}

	url := c.baseURL + "/api/v1/veo/generate"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create veo request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("veo request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read veo response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := raw
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("veo status %d: %s", resp.StatusCode, string(limited))
	}

	var parsed CreateTaskResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse veo response: %w", err)
	}
	parsed.RawBody = string(raw)

	msg := strings.TrimSpace(parsed.Message)
	if msg == "" {
		msg = strings.TrimSpace(parsed.Msg)
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		limited := parsed.RawBody
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("veo error: code=%d message=%s raw=%s", parsed.Code, msg, limited)
	}
	if strings.TrimSpace(parsed.Data.TaskID) == "" {
		parsed.Data.TaskID = strings.TrimSpace(parsed.Data.TaskIDAlt)
	}
	if strings.TrimSpace(parsed.Data.TaskID) == "" {
		limited := parsed.RawBody
		if len(limited) > 2048 {
			limited = limited[:2048]
		}
		return "", fmt.Errorf("veo response missing taskId: raw=%s", limited)
	}
	return parsed.Data.TaskID, nil
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
