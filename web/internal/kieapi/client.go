package kieapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
		Info       struct {
			ResultUrls []string `json:"resultUrls"`
			ResultURLs []string `json:"result_urls"`
		} `json:"info"`
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
	if st == "" && p.Code == 200 {
		return "success"
	}
	return st
}

func (p CallbackPayload) ResultURL() string {
	if u := findFirstHTTP(p.Data.Result); u != "" {
		return u
	}
	if len(p.Data.Info.ResultUrls) > 0 {
		return strings.TrimSpace(p.Data.Info.ResultUrls[0])
	}
	if len(p.Data.Info.ResultURLs) > 0 {
		return strings.TrimSpace(p.Data.Info.ResultURLs[0])
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
		return "", fmt.Errorf("kieapi status %d: %s", resp.StatusCode, truncate(string(raw), 512))
	}

	var parsed CreateTaskResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse kieapi response: %w", err)
	}

	if parsed.Code != 0 && parsed.Code != 200 {
		msg := parsed.Message
		if msg == "" {
			msg = parsed.Msg
		}
		return "", fmt.Errorf("kieapi error: code=%d message=%s", parsed.Code, msg)
	}

	taskID := strings.TrimSpace(parsed.Data.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(parsed.Data.TaskIDAlt)
	}
	if taskID == "" {
		return "", fmt.Errorf("kieapi response missing taskId")
	}
	return taskID, nil
}

// CreateVeoTask creates a task using the Veo-specific endpoint.
// Veo API expects a flat request body (all fields at top level, not nested in "input").
func (c *Client) CreateVeoTask(req CreateTaskRequest) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kieapi client is nil")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("KIEAPI_API_KEY is not set")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("KIEAPI_BASE_URL is not set")
	}

	// Veo API expects flat body: merge input fields with top-level model/callBackUrl
	flat := make(map[string]any)
	for k, v := range req.Input {
		flat[k] = v
	}
	flat["model"] = req.Model
	flat["callBackUrl"] = req.CallBackURL

	body, err := json.Marshal(flat)
	if err != nil {
		return "", fmt.Errorf("marshal kieapi veo payload: %w", err)
	}

	// Veo uses a different endpoint
	url := c.baseURL + "/api/v1/veo/generate"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create kieapi veo request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("kieapi veo request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read kieapi veo response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("kieapi veo status %d: %s", resp.StatusCode, truncate(string(raw), 512))
	}

	var parsed CreateTaskResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse kieapi veo response: %w", err)
	}

	if parsed.Code != 0 && parsed.Code != 200 {
		msg := parsed.Message
		if msg == "" {
			msg = parsed.Msg
		}
		return "", fmt.Errorf("kieapi veo error: code=%d message=%s", parsed.Code, msg)
	}

	taskID := strings.TrimSpace(parsed.Data.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(parsed.Data.TaskIDAlt)
	}
	if taskID == "" {
		return "", fmt.Errorf("kieapi veo response missing taskId")
	}
	return taskID, nil
}

// UploadFile uploads image bytes to KieAPI's file storage and returns the hosted URL.
// This avoids "Image fetch failed" errors when passing third-party image URLs.
func (c *Client) UploadFile(data []byte, filename string) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", fmt.Errorf("kieapi client not configured")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err = fw.Write(data); err != nil {
		return "", fmt.Errorf("write form file: %w", err)
	}
	mw.Close()

	url := c.baseURL + "/api/v1/jobs/upload"
	httpReq, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read upload response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("kieapi upload error %d: %s", resp.StatusCode, truncate(string(raw), 256))
	}

	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse upload response: %w", err)
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		msg := parsed.Message
		if msg == "" {
			msg = parsed.Msg
		}
		return "", fmt.Errorf("kieapi upload error: code=%d message=%s", parsed.Code, msg)
	}
	if parsed.Data.URL == "" {
		return "", fmt.Errorf("kieapi upload returned empty URL, raw=%s", truncate(string(raw), 256))
	}
	return parsed.Data.URL, nil
}

func (c *Client) GetRecordInfo(taskID string) (*CallbackPayload, error) {
	if c == nil || c.apiKey == "" || c.baseURL == "" {
		return nil, fmt.Errorf("kieapi client not configured")
	}

	url := c.baseURL + "/api/v1/jobs/recordInfo?taskId=" + taskID
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed CallbackPayload
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
