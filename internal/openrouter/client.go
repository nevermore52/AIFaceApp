package openrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"telegram-ai-face-bot/internal/config"
)

type Client struct {
	apiKey       string
	baseURL      string
	model        string
	client       *http.Client
	debugLogging bool
}

func (c *Client) debugLog(format string, v ...any) {
	if c.debugLogging {
		log.Printf("[API DEBUG] "+format, v...)
	}
}

type GenerationRequest struct {
	Model    string         `json:"model"`
	TaskType string         `json:"task_type"`
	Input    map[string]any `json:"input"`
	Params   map[string]any `json:"params,omitempty"`
}

type GenerationResponse struct {
	RawBody string `json:"-"`
}

type MessageContent struct {
	Type        string    `json:"type"`
	Text        string    `json:"text,omitempty"`
	ImageURL    *ImageURL `json:"image_url,omitempty"`
	Data        string    `json:"data,omitempty"`
	ImageBase64 string    `json:"image_base64,omitempty"`
	MimeType    string    `json:"mime_type,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type MessageContentList []MessageContent

func (m *MessageContentList) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*m = []MessageContent{{Type: "text", Text: s}}
		return nil
	}

	var parts []MessageContent
	if err := json.Unmarshal(b, &parts); err != nil {
		return err
	}
	*m = parts
	return nil
}

func NewClient(cfg config.OpenRouterConfig) *Client {
	model := cfg.Model
	if model == "" {
		model = "gemini"
	} else if strings.Contains(model, "flash") {
		model = "gemini"
	}
	c := &Client{
		apiKey:       cfg.APIKey,
		baseURL:      cfg.BaseURL,
		model:        model,
		debugLogging: cfg.DebugLogging,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	c.debugLog("Client initialized: baseURL=%s, model=%s, apiKey=%s...", cfg.BaseURL, model, cfg.APIKey[:min(10, len(cfg.APIKey))])
	return c
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) ChangeImage(model string, inputImageURLs []string, hairDescription string, aspectRatio string) (string, error) {
	if model == "" || model == "Gemini" {
		model = c.model
	}

	if len(inputImageURLs) == 0 {
		return "", fmt.Errorf("no input images provided")
	}
	if len(inputImageURLs) > 4 {
		inputImageURLs = inputImageURLs[:4]
	}

	taskType := model
	switch model {
	case "nano-banana-pro", "google/nano-banana-pro":
		taskType = "nano-banana-pro"
		model = "gemini"
	case "gemini-2.5-flash-image", "Nano Banana", "nano-banana", "google/nano-banana":
		taskType = "gemini-2.5-flash-image"
		model = "gemini"
	}

	firstURL := inputImageURLs[0]
	c.debugLog("ChangeImage started: description=%s, firstImage=%s, model=%s, taskType=%s", hairDescription, firstURL, model, taskType)
	joinedURLs := strings.Join(inputImageURLs, ",")
	c.debugLog("ChangeImage input URLs: %s", truncate(joinedURLs, 200))

	input := map[string]any{
		"prompt":        hairDescription,
		"image_urls":    inputImageURLs,
		"output_format": "png",
	}
	if aspectRatio != "" {
		input["aspect_ratio"] = aspectRatio
	}

	req := GenerationRequest{
		Model:    model,
		TaskType: taskType,
		Input:    input,
	}

	taskData, raw, err := c.makeRequest("/task", req)
	if err != nil {
		c.debugLog("ChangeImage ERROR: %v", err)
		return "", fmt.Errorf("failed to change image: %w", err)
	}

	result, err := c.waitForResult(taskData, raw)
	if err != nil {
		c.debugLog("ChangeImage waitForResult ERROR: %v", err)
		return "", err
	}

	c.debugLog("ChangeImage SUCCESS: result=%s", truncate(result, 100))
	return result, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (c *Client) GenerateAudio(model, text, taskType, stylePrompt string) (string, error) {
	if model == "" {
		model = c.model
	}
	if taskType == "" {
		taskType = "txt2audio-base"
	}
	if stylePrompt == "" {
		stylePrompt = "pop"
	}

	c.debugLog("GenerateAudio started: text_len=%d, model=%s, taskType=%s", len(text), model, taskType)

	var input map[string]any
	if taskType == "music" {
		input = map[string]any{
			"gpt_description_prompt": text,
		}
	} else {
		input = map[string]any{
			"prompt":        text,
			"lyrics":        text,
			"style_prompt":  stylePrompt,
			"output_format": "mp3",
		}
	}

	req := GenerationRequest{
		Model:    model,
		TaskType: taskType,
		Input:    input,
	}

	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt-1) * time.Second
			if backoff > 3*time.Second {
				backoff = 3 * time.Second
			}
			time.Sleep(backoff)
		}

		taskData, raw, err := c.makeRequest("/task", req)
		if err != nil {
			lastErr = err
			c.debugLog("GenerateAudio attempt %d makeRequest ERROR: %v", attempt, err)
			continue
		}

		att, delay := waitParamsForModel(model, taskType)
		result, err := c.waitForResultWithLimit(taskData, raw, att, delay)
		if err == nil {
			c.debugLog("GenerateAudio SUCCESS (attempt %d): result=%s", attempt, truncate(result, 120))
			return result, nil
		}

		lastErr = err
		c.debugLog("GenerateAudio attempt %d waitForResult ERROR: %v", attempt, err)
	}

	return "", fmt.Errorf("failed to generate audio after %d attempts: %w", maxAttempts, lastErr)
}

func (c *Client) GenerateVideo(imageURL, model, taskType string) (string, error) {
	if model == "" {
		model = c.model
	}
	if taskType == "" {
		taskType = "image_to_video"
	}

	c.debugLog("GenerateVideo started: model=%s, taskType=%s, imageURL=%s", model, taskType, truncate(imageURL, 160))

	req := GenerationRequest{
		Model:    model,
		TaskType: taskType,
		Input: map[string]any{
			"image_url": imageURL,
		},
	}

	taskData, raw, err := c.makeRequest("/task", req)
	if err != nil {
		c.debugLog("GenerateVideo ERROR: %v", err)
		return "", fmt.Errorf("failed to generate video: %w", err)
	}

	att, delay := waitParamsForModel(model, taskType)
	result, err := c.waitForResultWithLimit(taskData, raw, att, delay)
	if err != nil {
		c.debugLog("GenerateVideo waitForResult ERROR: %v", err)
		return "", err
	}

	c.debugLog("GenerateVideo SUCCESS: result=%s", truncate(result, 120))
	return result, nil
}

func (c *Client) GenerateChat(model string, messages []map[string]string) (string, error) {
	if model == "" {
		model = c.model
	}

	c.debugLog("GenerateChat started: model=%s, messages=%d", model, len(messages))

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}

	url := c.chatCompletionURL()
	start := time.Now()

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to do chat request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read chat response: %w", err)
	}

	c.debugLog("GenerateChat response: status=%d, duration=%v, body(first 400)=%s", resp.StatusCode, time.Since(start), truncate(string(body), 400))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		if content, serr := parseChatStream(body); serr == nil && content != "" {
			return content, nil
		}
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		if content, serr := parseChatStream(body); serr == nil && content != "" {
			return content, nil
		}
		return "", fmt.Errorf("no choices in chat response")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		if content, serr := parseChatStream(body); serr == nil && content != "" {
			return content, nil
		}
		return "", fmt.Errorf("empty content in chat response")
	}
	return content, nil
}

func contentToString(content MessageContentList) string {
	var b strings.Builder
	for i, part := range content {
		if i > 0 {
			b.WriteString(" | ")
		}
		switch part.Type {
		case "text":
			b.WriteString(part.Text)
		case "image_url":
			if part.ImageURL != nil {
				b.WriteString("image_url:")
				b.WriteString(part.ImageURL.URL)
			}
		default:
			if part.Text != "" {
				b.WriteString(part.Text)
			} else if part.Data != "" {
				b.WriteString("data:(len=")
				b.WriteString(fmt.Sprintf("%d", len(part.Data)))
				b.WriteString(")")
			}
		}
	}
	return b.String()
}

func (c *Client) extractImageURLFromPiAPI(resp map[string]any, raw string) (string, error) {
	if videoURL, ok := findVideoURL(resp); ok {
		return videoURL, nil
	}

	if dataMap, ok := resp["data"].(map[string]any); ok {
		if output, ok := dataMap["output"].(map[string]any); ok {
			if url, ok := firstStringFromArrays(output, "image_urls", "output_urls", "images", "result", "results"); ok {
				return url, nil
			}
			if url, ok := output["output_url"].(string); ok && url != "" {
				return url, nil
			}
		}
		if url, ok := firstStringFromArrays(dataMap, "output_urls", "images", "result", "image_urls", "results"); ok {
			return url, nil
		}
		if url, ok := dataMap["output_url"].(string); ok && url != "" {
			return url, nil
		}
	}

	if url, ok := firstStringFromArrays(resp, "output_urls", "images", "result", "image_urls", "results"); ok {
		return url, nil
	}
	if url, ok := resp["output_url"].(string); ok && url != "" {
		return url, nil
	}

	if raw != "" {
		if candidate, ok := findVideoURLInRaw(raw); ok {
			return candidate, nil
		}
		if candidate, ok := findImageInRaw(raw); ok {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no image url or data found in response")
}

func firstStringFromArrays(m map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch arr := v.(type) {
			case []any:
				for _, item := range arr {
					if s, ok := item.(string); ok && s != "" {
						return s, true
					}
				}
			case []string:
				for _, s := range arr {
					if s != "" {
						return s, true
					}
				}
			}
		}
	}
	return "", false
}

func findImageInRaw(raw string) (string, bool) {
	if idx := strings.Index(raw, "data:image"); idx != -1 {
		end := idx
		for end < len(raw) && raw[end] != '"' && raw[end] != '\\' && raw[end] != ' ' && raw[end] != '\n' {
			end++
		}
		return raw[idx:end], true
	}
	if idx := strings.Index(raw, "http"); idx != -1 {
		end := idx
		for end < len(raw) && raw[end] != '"' && raw[end] != '\\' && raw[end] != ' ' && raw[end] != '\n' {
			end++
		}
		return raw[idx:end], true
	}
	return "", false
}

func findVideoURL(resp map[string]any) (string, bool) {
	if dataMap, ok := resp["data"].(map[string]any); ok {
		if output, ok := dataMap["output"].(map[string]any); ok {
			if url, ok := output["video_url"].(string); ok && url != "" {
				return url, true
			}
		}
		if url, ok := dataMap["video_url"].(string); ok && url != "" {
			return url, true
		}
	}
	if url, ok := resp["video_url"].(string); ok && url != "" {
		return url, true
	}
	return "", false
}

func findVideoURLInRaw(raw string) (string, bool) {
	key := `"video_url":"`
	if idx := strings.Index(raw, key); idx >= 0 {
		start := idx + len(key)
		end := strings.Index(raw[start:], `"`)
		if end > 0 {
			return raw[start : start+end], true
		}
	}
	return "", false
}

func (c *Client) makeRequest(endpoint string, payload any) (map[string]any, string, error) {
	return c.makeRequestWithMethod("POST", endpoint, payload)
}

func (c *Client) makeRequestWithMethod(method, endpoint string, payload any) (map[string]any, string, error) {
	startTime := time.Now()
	fullURL := c.baseURL + endpoint
	c.debugLog("makeRequest START: URL=%s", fullURL)

	var bodyReader *bytes.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			c.debugLog("makeRequest ERROR: failed to marshal: %v", err)
			return nil, "", fmt.Errorf("failed to marshal request: %w", err)
		}
		c.debugLog("Request body size: %d bytes", len(jsonData))
		bodyReader = bytes.NewReader(jsonData)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		c.debugLog("makeRequest ERROR: failed to create request: %v", err)
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.debugLog("Executing HTTP request...")
	resp, err := c.client.Do(req)
	if err != nil {
		c.debugLog("makeRequest ERROR: HTTP request failed after %v: %v", time.Since(startTime), err)
		return nil, "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	c.debugLog("Response received: status=%d, duration=%v", resp.StatusCode, time.Since(startTime))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.debugLog("makeRequest ERROR: failed to read body: %v", err)
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}

	c.debugLog("Response body size: %d bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		c.debugLog("makeRequest ERROR: status %d, body: %s", resp.StatusCode, truncate(string(body), 1000))
		return nil, string(body), fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		c.debugLog("makeRequest ERROR: failed to decode JSON: %v, body: %s", err, truncate(string(body), 500))
		return nil, string(body), fmt.Errorf("failed to decode response: %w", err)
	}

	c.debugLog("makeRequest SUCCESS: duration=%v", time.Since(startTime))
	c.debugLog("Response body (first 1000 chars): %s", truncate(string(body), 1000))

	return response, string(body), nil
}

func (c *Client) waitForResultWithLimit(initial map[string]any, raw string, maxAttempts int, delay time.Duration) (string, error) {
	taskID := ""
	if dataMap, ok := initial["data"].(map[string]any); ok {
		if id, ok := dataMap["task_id"].(string); ok {
			taskID = id
		}
		status := getStatus(dataMap)
		if status == "completed" {
			return c.extractImageURLFromPiAPI(initial, raw)
		}
		if status == "failed" {
			return "", fmt.Errorf("task failed: %s", extractErrorMessage(dataMap))
		}
	}

	if taskID == "" {
		return "", fmt.Errorf("task_id not found in response")
	}

	for i := 0; i < maxAttempts; i++ {
		time.Sleep(delay)
		data, rawResp, err := c.makeRequestWithMethod("GET", "/task/"+taskID, nil)
		if err != nil {
			return "", err
		}
		if dataMap, ok := data["data"].(map[string]any); ok {
			status := getStatus(dataMap)
			switch status {
			case "completed":
				return c.extractImageURLFromPiAPI(data, rawResp)
			case "failed":
				return "", fmt.Errorf("task failed: %s", extractErrorMessage(dataMap))
			}
		}
	}

	return "", fmt.Errorf("task %s did not complete in time", taskID)
}

func (c *Client) waitForResult(initial map[string]any, raw string) (string, error) {
	return c.waitForResultWithLimit(initial, raw, 250, 5*time.Second)
}

func (c *Client) HealthCheck() error {
	url := strings.TrimRight(c.baseURL, "/") + "/status"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("healthcheck bad status: %d body: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return nil
}

func waitParamsForModel(model, taskType string) (attempts int, delay time.Duration) {
	attempts, delay = 60, 2*time.Second

	lowerModel := strings.ToLower(model)
	lowerTask := strings.ToLower(taskType)

	if lowerTask == "music" || strings.Contains(lowerModel, "music") || strings.Contains(lowerModel, "audio") {
		return 75, 20 * time.Second
	}

	if lowerTask == "image_to_video" || strings.Contains(lowerModel, "video") {
		return 75, 20 * time.Second
	}

	return
}

func (c *Client) chatCompletionURL() string {
	base := strings.TrimRight(c.baseURL, "/")
	base = strings.Replace(base, "/api/", "/", 1)
	return base + "/chat/completions"
}

func parseChatStream(body []byte) (string, error) {
	lines := strings.Split(string(body), "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		choices, ok := chunk["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}
		first, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}
		delta, ok := first["delta"].(map[string]any)
		if !ok {
			continue
		}
		if content, ok := delta["content"].(string); ok {
			b.WriteString(content)
		}
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return "", fmt.Errorf("empty chat stream")
	}
	return result, nil
}

func getStatus(data map[string]any) string {
	if status, ok := data["status"].(string); ok {
		return status
	}
	return ""
}

func extractErrorMessage(data map[string]any) string {
	if errObj, ok := data["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
		if raw, ok := errObj["raw_message"].(string); ok && raw != "" {
			return raw
		}
	}
	if logs, ok := data["logs"].([]any); ok {
		if len(logs) > 0 {
			if msg, ok := logs[len(logs)-1].(string); ok && msg != "" {
				return msg
			}
		}
	}
	return "unknown error"
}
