package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type DefAPIClientImpl struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewDefAPIClient(apiKey, baseURL string) DefAPIClient {
	return &DefAPIClientImpl{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * 1e9}, // 60 seconds
	}
}

func (c *DefAPIClientImpl) CreateChatCompletion(model string, messages []map[string]string) (string, error) {
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
		return "", fmt.Errorf("marshal defapi payload: %w", err)
	}

	url := c.baseURL + "/api/v1/chat/completions"
	fmt.Printf("[DEFAPI WEB] chat request url=%s model=%s\n", url, model)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create defapi request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("defapi request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read defapi response: %w", err)
	}

	fmt.Printf("[DEFAPI WEB] chat response status=%d body=%s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("defapi api error: %s", string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse defapi response: %w", err)
	}

	// Extract text from response
	if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unexpected defapi response format: %s", string(respBody))
}
