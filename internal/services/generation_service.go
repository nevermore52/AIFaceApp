package services

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"telegram-ai-face-bot/internal/defapi"
	"telegram-ai-face-bot/internal/kieapi"
	"telegram-ai-face-bot/internal/models"
	"telegram-ai-face-bot/internal/openrouter"
)

var debugLogging = os.Getenv("DEBUG_LOGGING") != "false"

func debugLog(format string, v ...any) {
	if debugLogging {
		log.Printf("[API GENERATION] "+format, v...)
	}
}

type GenerationService struct {
	db     *sql.DB
	client *openrouter.Client
	defAPI *defapi.Client
	kieAPI *kieapi.Client
	http   *http.Client
	notify func(chatID int64, req *models.GenerationRequest)

	inFlightMu      sync.Mutex
	inFlightByReqID map[int64]struct{}
	inFlightWg      sync.WaitGroup
}

func (s *GenerationService) ensureGenerationRequestsColumns() error {
	stmts := []string{
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS username VARCHAR(255);`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS model_type VARCHAR(50);`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS model VARCHAR(255);`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS external_task_id TEXT;`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS tokens_primary_used INTEGER DEFAULT 0;`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS tokens_extra_used INTEGER DEFAULT 0;`,
		`ALTER TABLE generation_requests ADD COLUMN IF NOT EXISTS output TEXT;`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

type LogRequestOptions struct {
	Username          string
	ModelType         string
	Model             string
	Prompt            string
	ExternalTaskID    string
	Status            string
	Output            string
	ErrorMsg          string
	TokensUsed        int
	TokensPrimaryUsed int
	TokensExtraUsed   int
}

type GenerationOptions struct {
	InputImage         string
	InputImages        []string
	Prompt             string
	TokensCost         int
	TokensPrimaryUsed  int
	TokensExtraUsed    int
	ChatID             int64
	Model              string
	ModelType          string
	Username           string
	UseDefAPI          bool
	NanoBananaProvider string
	AspectRatio        string
}

func NewGenerationService(db *sql.DB, client *openrouter.Client) *GenerationService {
	return &GenerationService{
		db:              db,
		client:          client,
		http:            &http.Client{Timeout: 30 * time.Second},
		inFlightByReqID: make(map[int64]struct{}),
	}
}

func (s *GenerationService) markInFlight(reqID int64) {
	if reqID == 0 {
		return
	}
	s.inFlightMu.Lock()
	_, exists := s.inFlightByReqID[reqID]
	if !exists {
		s.inFlightByReqID[reqID] = struct{}{}
		s.inFlightWg.Add(1)
	}
	s.inFlightMu.Unlock()
}

func (s *GenerationService) markDone(reqID int64) {
	if reqID == 0 {
		return
	}
	s.inFlightMu.Lock()
	_, exists := s.inFlightByReqID[reqID]
	if exists {
		delete(s.inFlightByReqID, reqID)
		s.inFlightWg.Done()
	}
	s.inFlightMu.Unlock()
}

// WaitInFlight blocks until all StartGeneration background tasks are finished.
// This is intended for graceful shutdown.
func (s *GenerationService) WaitInFlight() {
	s.inFlightWg.Wait()
}

func (s *GenerationService) SetDefAPIClient(client *defapi.Client) {
	s.defAPI = client
}

func (s *GenerationService) SetKieAPIClient(client *kieapi.Client) {
	s.kieAPI = client
}

func (s *GenerationService) StartGeneration(userID int64, opts GenerationOptions) (*models.GenerationRequest, error) {
	debugLog("StartGeneration: userID=%d, prompt=%s", userID, opts.Prompt)
	query := `
		INSERT INTO generation_requests (user_id, username, model_type, model, status, input_image, prompt, tokens_used, tokens_primary_used, tokens_extra_used)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9)
		RETURNING id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, created_at, completed_at`

	req := &models.GenerationRequest{}
	inputForStore := opts.InputImage
	if len(opts.InputImages) > 0 {
		inputForStore = strings.Join(opts.InputImages, ",")
	}

	err := s.db.QueryRow(query, userID, opts.Username, opts.ModelType, opts.Model, inputForStore, opts.Prompt, opts.TokensCost, opts.TokensPrimaryUsed, opts.TokensExtraUsed).Scan(
		&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
		&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
	)

	if err != nil {
		debugLog("StartGeneration ERROR: failed to create DB record: %v", err)
		return nil, fmt.Errorf("failed to create generation request: %w", err)
	}

	s.markInFlight(req.ID)

	debugLog("StartGeneration: DB record created with ID=%d, starting background processing...", req.ID)
	go s.processGeneration(req, opts)

	return req, nil
}

func (s *GenerationService) processGeneration(req *models.GenerationRequest, opts GenerationOptions) {
	defer func() {
		// For DefAPI/KieAPI async tasks we finish in callbacks.
		if !(useDefAPIModel(opts.Model) && opts.UseDefAPI) && !useKieAPIModel(opts.Model) {
			s.markDone(req.ID)
		}
	}()

	startTime := time.Now()
	debugLog("processGeneration START: requestID=%d, prompt=%s, model=%s", req.ID, opts.Prompt, opts.Model)

	images := opts.InputImages
	if len(images) == 0 && opts.InputImage != "" {
		images = []string{opts.InputImage}
	}
	if len(images) == 0 {
		debugLog("processGeneration ERROR: no input images provided")
		_ = s.updateRequestStatus(req.ID, "failed", "no input images provided")
		if s.notify != nil {
			req.Status = "failed"
			msg := "no input images provided"
			req.ErrorMsg = &msg
			s.notify(opts.ChatID, req)
		}
		return
	}
	if len(images) > 4 {
		images = images[:4]
	}
	debugLog("Input images count: %d", len(images))

	if err := s.updateRequestStatus(req.ID, "processing", ""); err != nil {
		debugLog("processGeneration ERROR: failed to update status to processing: %v", err)
	}
	req.Status = "processing"
	req.ErrorMsg = nil
	req.Output = nil
	req.CompletedAt = nil
	debugLog("Status updated to 'processing'")

	var resultURL string
	var err error

	if useDefAPIModel(opts.Model) && opts.UseDefAPI {
		taskID, err := s.createDefAPITask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration DefAPI FAILED: requestID=%d, error=%v", req.ID, err)
			_ = s.updateRequestStatus(req.ID, "failed", err.Error())
			req.Status = "failed"
			errMsg := err.Error()
			req.ErrorMsg = &errMsg
			if s.notify != nil {
				s.notify(opts.ChatID, req)
			}
			s.markDone(req.ID)
			return
		}
		debugLog("processGeneration DefAPI task created: requestID=%d taskID=%s", req.ID, taskID)
		return
	}

	// Nano Banana must never go to OpenRouter/PiAPI. Only DefAPI or KieAPI.
	if strings.EqualFold(strings.TrimSpace(opts.Model), "google/nano-banana") || strings.EqualFold(strings.TrimSpace(opts.Model), "google/nano-banana-pro") {
		provider := strings.ToLower(strings.TrimSpace(opts.NanoBananaProvider))
		if provider == "defapi" {
			taskID, err := s.createDefAPITask(req.ID, opts, images)
			if err != nil {
				debugLog("processGeneration DefAPI FAILED: requestID=%d, error=%v", req.ID, err)
				_ = s.updateRequestStatus(req.ID, "failed", err.Error())
				req.Status = "failed"
				errMsg := err.Error()
				req.ErrorMsg = &errMsg
				if s.notify != nil {
					s.notify(opts.ChatID, req)
				}
				s.markDone(req.ID)
				return
			}
			debugLog("processGeneration DefAPI task created: requestID=%d taskID=%s", req.ID, taskID)
			return
		}

		// default provider is kieapi
		taskID, err := s.createKieAPITask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration KieAPI FAILED: requestID=%d, error=%v", req.ID, err)
			_ = s.updateRequestStatus(req.ID, "failed", err.Error())
			req.Status = "failed"
			errMsg := err.Error()
			req.ErrorMsg = &errMsg
			if s.notify != nil {
				s.notify(opts.ChatID, req)
			}
			s.markDone(req.ID)
			return
		}
		debugLog("processGeneration KieAPI task created: requestID=%d taskID=%s", req.ID, taskID)
		return
	}

	if useKieAPIModel(opts.Model) {
		taskID, err := s.createKieAPITask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration KieAPI FAILED: requestID=%d, error=%v", req.ID, err)
			_ = s.updateRequestStatus(req.ID, "failed", err.Error())
			req.Status = "failed"
			errMsg := err.Error()
			req.ErrorMsg = &errMsg
			if s.notify != nil {
				s.notify(opts.ChatID, req)
			}
			s.markDone(req.ID)
			return
		}
		debugLog("processGeneration KieAPI task created: requestID=%d taskID=%s", req.ID, taskID)
		return
	}

	debugLog("Calling OpenRouter API")
	resultURL, err = s.client.ChangeImage(opts.Model, images, opts.Prompt, opts.AspectRatio)

	if err != nil {
		debugLog("processGeneration FAILED: requestID=%d, error=%v, duration=%v", req.ID, err, time.Since(startTime))
		_ = s.updateRequestStatus(req.ID, "failed", err.Error())
		req.Status = "failed"
		errMsg := err.Error()
		req.ErrorMsg = &errMsg
		if s.notify != nil {
			s.notify(opts.ChatID, req)
		}
		return
	}

	debugLog("processGeneration SUCCESS: requestID=%d, resultURL=%s, duration=%v", req.ID, resultURL, time.Since(startTime))

	if err := s.completeRequest(req.ID, resultURL); err != nil {
		debugLog("processGeneration ERROR: failed to complete request: %v", err)
	}
	debugLog("Request completed and saved to DB")

	req.Status = "completed"
	req.ErrorMsg = nil
	req.Output = &resultURL
	now := time.Now()
	req.CompletedAt = &now

	if s.notify != nil {
		s.notify(opts.ChatID, req)
	}
}

func (s *GenerationService) SetNotifier(fn func(chatID int64, req *models.GenerationRequest)) {
	s.notify = fn
}
func (s *GenerationService) HandleDefAPICallback(payload defapi.CallbackPayload) error {
	if payload.TaskID == "" {
		return fmt.Errorf("defapi callback missing task_id")
	}

	req, err := s.getGenerationRequestByExternalTaskID(payload.TaskID)
	if err != nil {
		return err
	}

	if payload.Status != "success" {
		reason := fmt.Sprintf("defapi status=%s", payload.Status)
		if msg, ok := payload.StatusReason["message"].(string); ok && strings.TrimSpace(msg) != "" {
			reason = msg
		}
		_ = s.updateRequestStatus(req.ID, "failed", reason)
		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil {
			s.notify(req.UserID, req)
		}
		s.markDone(req.ID)
		return nil
	}

	url := payload.ResultURL()
	if url == "" {
		reason := "defapi callback missing result url"
		if msg, ok := payload.StatusReason["message"].(string); ok {
			msg = strings.TrimSpace(msg)
			if msg != "" {
				reason = msg
			}
			if msg == "Request successful, but the official returned empty content. Since the official platform will still charge us for this, we have no choice but to bill this request as well." {
				reason = "Произошла ошибка возможно вы нарушили правила бота."
			}
		}

		_ = s.updateRequestStatus(req.ID, "failed", reason)
		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil {
			s.notify(req.UserID, req)
		}
		s.markDone(req.ID)
		return nil
	}

	if req.Status == "completed" {
		s.markDone(req.ID)
		return nil
	}
	if err := s.completeRequest(req.ID, url); err != nil {
		return err
	}

	req.Status = "completed"
	req.ErrorMsg = nil
	req.Output = &url
	now := time.Now()
	req.CompletedAt = &now
	if s.notify != nil {
		s.notify(req.UserID, req)
	}
	s.markDone(req.ID)
	return nil
}

func useKieAPIModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return model == "kie/nano-banana-edit" || model == "kie/nano-banana-pro" || model == "nano-banana-pro"
}

func mapKieAPIModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch model {
	case "google/nano-banana":
		return "google/nano-banana-edit"
	case "google/nano-banana-pro":
		return "nano-banana-pro"
	case "kie/nano-banana-edit":
		return "google/nano-banana-edit"
	case "kie/nano-banana-pro":
		return "nano-banana-pro"
	case "nano-banana-pro":
		return "nano-banana-pro"
	default:
		return model
	}
}

func (s *GenerationService) createKieAPITask(requestID int64, opts GenerationOptions, images []string) (string, error) {
	if s.kieAPI == nil {
		return "", fmt.Errorf("kieapi client is not configured")
	}
	callbackURL := strings.TrimSpace(os.Getenv("KIEAPI_CALLBACK_URL"))
	if callbackURL == "" {
		callbackURL = strings.TrimSpace(os.Getenv("KIE_CALLBACK_URL"))
	}
	if callbackURL == "" {
		return "", fmt.Errorf("KIEAPI_CALLBACK_URL is not set")
	}

	apiModel := mapKieAPIModel(opts.Model)
	input := map[string]any{
		"prompt":       opts.Prompt,
		"aspect_ratio": opts.AspectRatio,
	}
	modelID := strings.ToLower(strings.TrimSpace(opts.Model))
	if modelID == "kie/nano-banana-pro" || modelID == "nano-banana-pro" || modelID == "google/nano-banana-pro" {
		if len(images) > 0 {
			input["image_input"] = images
		} else {
			input["image_input"] = []string{}
		}
	} else {
		input["image_urls"] = images
	}
	payload := kieapi.CreateTaskRequest{
		Model:       apiModel,
		CallBackURL: callbackURL,
		Input:       input,
	}

	taskID, err := s.kieAPI.CreateTask(payload)
	if err != nil {
		return "", err
	}
	if err := s.updateExternalTaskID(requestID, taskID); err != nil {
		return "", err
	}
	return taskID, nil
}

func (s *GenerationService) HandleKieAPICallback(payload kieapi.CallbackPayload) error {
	taskID := strings.TrimSpace(payload.TaskIDValue())
	if taskID == "" {
		return fmt.Errorf("kieapi callback missing taskId")
	}

	req, err := s.getGenerationRequestByExternalTaskID(taskID)
	if err != nil {
		return err
	}

	status := strings.ToLower(strings.TrimSpace(payload.StatusValue()))
	if status != "success" && status != "completed" && status != "succeeded" {
		reason := strings.TrimSpace(payload.Msg)
		if reason == "" {
			reason = fmt.Sprintf("kieapi status=%s", payload.StatusValue())
		}
		_ = s.updateRequestStatus(req.ID, "failed", reason)
		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil {
			s.notify(req.UserID, req)
		}
		s.markDone(req.ID)
		return nil
	}

	url := strings.TrimSpace(payload.ResultURL())
	if url == "" {
		reason := strings.TrimSpace(payload.Msg)
		if reason == "" {
			reason = "kieapi callback missing result url"
		}
		_ = s.updateRequestStatus(req.ID, "failed", reason)
		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil {
			s.notify(req.UserID, req)
		}
		s.markDone(req.ID)
		return nil
	}

	if req.Status == "completed" {
		s.markDone(req.ID)
		return nil
	}

	if err := s.completeRequest(req.ID, url); err != nil {
		return err
	}

	req.Status = "completed"
	req.ErrorMsg = nil
	req.Output = &url
	now := time.Now()
	req.CompletedAt = &now
	if s.notify != nil {
		s.notify(req.UserID, req)
	}
	s.markDone(req.ID)
	return nil
}
func (s *GenerationService) GenerateAudio(text, model, taskType, stylePrompt string) (string, error) {
	return s.client.GenerateAudio(model, text, taskType, stylePrompt)
}
func (s *GenerationService) GenerateVideoFromImage(imageURL, model, taskType string) (string, error) {
	return s.client.GenerateVideo(imageURL, model, taskType)
}
func (s *GenerationService) GenerateChat(model string, messages []map[string]string) (string, error) {
	if useDefAPIChatModel(model) {
		if s.defAPI == nil {
			return "", fmt.Errorf("defapi client is not configured")
		}
		return s.defAPI.CreateChatCompletion(model, messages)
	}
	return s.client.GenerateChat(model, messages)
}

func (s *GenerationService) GenerateMusicSuno(prompt, vocalGender string, instrumental bool) (string, string, error) {
	apiKey := os.Getenv("SUNO_API_KEY")
	if apiKey == "" {
		return "", "", fmt.Errorf("SUNO_API_KEY is not set")
	}
	callbackURL := os.Getenv("SUNO_CALLBACK_URL")
	if callbackURL == "" {
		return "", "", fmt.Errorf("SUNO_CALLBACK_URL is not set")
	}

	gender := strings.ToLower(vocalGender)
	if gender != "f" {
		gender = "m"
	}

	payload := map[string]any{
		"customMode":          false,
		"instrumental":        instrumental,
		"model":               "V5",
		"callBackUrl":         callbackURL,
		"prompt":              prompt,
		"vocalGender":         gender,
		"styleWeight":         0.65,
		"weirdnessConstraint": 0.65,
		"audioWeight":         0.75,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal suno payload: %w", err)
	}

	debugLog("Suno request payload: %s", truncate(string(body), 300))

	req, err := http.NewRequest("POST", "https://api.sunoapi.org/api/v1/generate", strings.NewReader(string(body)))
	if err != nil {
		return "", "", fmt.Errorf("create suno request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		debugLog("Suno request failed: %v", err)
		return "", "", fmt.Errorf("suno request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		debugLog("Suno response error: status=%d body=%s", resp.StatusCode, truncate(string(raw), 300))
		return "", "", fmt.Errorf("suno api error: %s", string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read suno response: %w", err)
	}
	debugLog("Suno response raw: %s", truncate(string(raw), 400))

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", "", fmt.Errorf("parse suno response: %w, raw=%s", err, string(raw))
	}

	taskID := findStringByKeys(generic, "taskId", "task_id")
	audio := findStringByKeys(
		generic,
		"audio_url", "audioUrl", "audio",
		"audioUrlHigh", "audio_url_high", "audio_high", "audio_high_url",
		"download_url", "downloadUrl",
		"result_url", "resultUrl",
		"streaming_url", "streamingUrl",
		"preview_url", "previewUrl",
		"url",
	)
	if audio == "" {
		audio = findFirstURL(generic)
	}

	debugLog("Suno parsed taskID=%s audioCandidate=%s", taskID, truncate(audio, 200))

	if audio != "" {
		return audio, taskID, nil
	}
	if taskID != "" {
		return "", taskID, nil
	}

	return "", "", fmt.Errorf("suno response missing audio url and taskId: %s", string(raw))
}

func (s *GenerationService) HealthCheckPiAPI() error {
	return s.client.HealthCheck()
}

func useDefAPIModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return model == "google/nano-banana" || model == "google/nano-banana-pro"
}

func useDefAPIChatModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return model == "google/gemini-3-flash" || model == "openai/gpt-5-mini" || model == "openai/gpt-5-nano"
}

func (s *GenerationService) createDefAPITask(requestID int64, opts GenerationOptions, images []string) (string, error) {
	if s.defAPI == nil {
		return "", fmt.Errorf("defapi client is not configured")
	}
	callbackURL := os.Getenv("DEF_CALLBACK_URL")
	if callbackURL == "" {
		return "", fmt.Errorf("DEF_CALLBACK_URL is not set")
	}

	taskID, err := s.defAPI.CreateImageTask(opts.Model, opts.Prompt, images, callbackURL, opts.AspectRatio)
	if err != nil {
		return "", err
	}

	if err := s.updateExternalTaskID(requestID, taskID); err != nil {
		return "", err
	}

	return taskID, nil
}

func (s *GenerationService) updateExternalTaskID(requestID int64, taskID string) error {
	_, err := s.db.Exec(`UPDATE generation_requests SET external_task_id = $2 WHERE id = $1`, requestID, taskID)
	return err
}

func (s *GenerationService) getGenerationRequestByExternalTaskID(taskID string) (*models.GenerationRequest, error) {
	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, created_at, completed_at
		FROM generation_requests WHERE external_task_id = $1`

	req := &models.GenerationRequest{}
	err := s.db.QueryRow(query, taskID).Scan(
		&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
		&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
	)

	return req, err
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

func findFirstURL(v any) string {
	switch val := v.(type) {
	case map[string]any:
		for _, vv := range val {
			if u := findFirstURL(vv); u != "" {
				return u
			}
		}
	case []any:
		for _, vv := range val {
			if u := findFirstURL(vv); u != "" {
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *GenerationService) updateRequestStatus(requestID int64, status, errorMsg string) error {
	query := `UPDATE generation_requests SET status = $2, error_msg = $3 WHERE id = $1`
	_, err := s.db.Exec(query, requestID, status, errorMsg)
	return err
}

func (s *GenerationService) completeRequest(requestID int64, outputImage string) error {
	query := `
		UPDATE generation_requests
		SET status = 'completed', output = $2, completed_at = CURRENT_TIMESTAMP
		WHERE id = $1`
	_, err := s.db.Exec(query, requestID, outputImage)
	return err
}

func (s *GenerationService) LogRequest(userID int64, opts LogRequestOptions) (*models.GenerationRequest, error) {
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "processing"
	}

	query := `
		INSERT INTO generation_requests (user_id, username, model_type, model, status, input_image, output, external_task_id, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, completed_at)
		VALUES ($1, $2, $3, $4, $5::text, '', $6, $7, $8, $9, $10, $11, $12,
			CASE WHEN $5::text = 'completed' THEN CURRENT_TIMESTAMP ELSE NULL::timestamptz END)
		RETURNING id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, created_at, completed_at`

	req := &models.GenerationRequest{}
	row := func() *sql.Row {
		return s.db.QueryRow(
			query,
			userID,
			opts.Username,
			opts.ModelType,
			opts.Model,
			status,
			opts.Output,
			opts.ExternalTaskID,
			opts.Prompt,
			opts.ErrorMsg,
			opts.TokensUsed,
			opts.TokensPrimaryUsed,
			opts.TokensExtraUsed,
		)
	}

	err := row().Scan(
		&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
		&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
	)
	if err != nil {
		if ensureErr := s.ensureGenerationRequestsColumns(); ensureErr == nil {
			err = row().Scan(
				&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
				&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
			)
		}
		return nil, err
	}
	return req, nil
}

func (s *GenerationService) CompleteRequest(requestID int64, output string) error {
	return s.completeRequest(requestID, output)
}

func (s *GenerationService) FailRequest(requestID int64, reason string) error {
	return s.updateRequestStatus(requestID, "failed", reason)
}

func (s *GenerationService) CompleteRequestByExternalTaskID(taskID string, output string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("external task id is empty")
	}
	_, err := s.db.Exec(`UPDATE generation_requests SET status = 'completed', output = $2, completed_at = CURRENT_TIMESTAMP WHERE external_task_id = $1`, taskID, output)
	return err
}

func (s *GenerationService) FailRequestByExternalTaskID(taskID string, reason string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("external task id is empty")
	}
	_, err := s.db.Exec(`UPDATE generation_requests SET status = 'failed', error_msg = $2 WHERE external_task_id = $1`, taskID, reason)
	return err
}

func (s *GenerationService) GetGenerationRequest(requestID int64) (*models.GenerationRequest, error) {
	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, created_at, completed_at
		FROM generation_requests WHERE id = $1`

	req := &models.GenerationRequest{}
	err := s.db.QueryRow(query, requestID).Scan(
		&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
		&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
	)

	return req, err
}

func (s *GenerationService) GetUserGenerationRequests(userID int64, limit, offset int) ([]*models.GenerationRequest, error) {
	query := `
		SELECT id, user_id, username, model_type, model, status, input_image, output, prompt, error_msg, tokens_used, tokens_primary_used, tokens_extra_used, created_at, completed_at
		FROM generation_requests
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*models.GenerationRequest
	for rows.Next() {
		req := &models.GenerationRequest{}
		err := rows.Scan(
			&req.ID, &req.UserID, &req.Username, &req.ModelType, &req.Model, &req.Status, &req.InputImage, &req.Output,
			&req.Prompt, &req.ErrorMsg, &req.TokensUsed, &req.TokensPrimaryUsed, &req.TokensExtraUsed, &req.CreatedAt, &req.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}

	return requests, rows.Err()
}

func (s *GenerationService) ValidateImage(imageData []byte) (string, error) {
	if len(imageData) > 10*1024*1024 {
		return "", fmt.Errorf("image too large: maximum size is 10MB")
	}
	if len(imageData) < 4 {
		return "", fmt.Errorf("invalid image data")
	}
	contentType := ""
	switch {
	case imageData[0] == 0xFF && imageData[1] == 0xD8 && imageData[2] == 0xFF:
		contentType = "image/jpeg"
	case imageData[0] == 0x89 && imageData[1] == 0x50 && imageData[2] == 0x4E && imageData[3] == 0x47:
		contentType = "image/png"
	case imageData[0] == 0x47 && imageData[1] == 0x49 && imageData[2] == 0x46:
		contentType = "image/gif"
	case imageData[0] == 0x42 && imageData[1] == 0x4D:
		contentType = "image/bmp"
	case imageData[0] == 0x52 && imageData[1] == 0x49 && imageData[2] == 0x46 && imageData[3] == 0x46:
		contentType = "image/webp"
	default:
		return "", fmt.Errorf("unsupported image format")
	}

	base64Data := base64.StdEncoding.EncodeToString(imageData)

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

func (s *GenerationService) GetGenerationStats() (map[string]any, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_requests,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_requests,
			COUNT(CASE WHEN status = 'processing' THEN 1 END) as processing_requests,
			AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) FILTER (WHERE status = 'completed' AND completed_at IS NOT NULL) as avg_processing_time_seconds
		FROM generation_requests`

	var total, completed, failed, processing int
	var avgTime *float64

	err := s.db.QueryRow(query).Scan(&total, &completed, &failed, &processing, &avgTime)
	if err != nil {
		return nil, err
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	stats := map[string]any{
		"total_requests":      total,
		"completed_requests":  completed,
		"failed_requests":     failed,
		"processing_requests": processing,
		"success_rate":        successRate,
	}

	if avgTime != nil {
		stats["avg_processing_time_seconds"] = *avgTime
	}

	return stats, nil
}

func (s *GenerationService) GetGenerationStatsSince(from time.Time) (map[string]any, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_requests,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_requests,
			COUNT(CASE WHEN status = 'processing' THEN 1 END) as processing_requests,
			AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) FILTER (WHERE status = 'completed' AND completed_at IS NOT NULL) as avg_processing_time_seconds
		FROM generation_requests
		WHERE created_at >= $1`

	var total, completed, failed, processing int
	var avgTime *float64

	err := s.db.QueryRow(query, from).Scan(&total, &completed, &failed, &processing, &avgTime)
	if err != nil {
		return nil, err
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	stats := map[string]any{
		"total_requests":      total,
		"completed_requests":  completed,
		"failed_requests":     failed,
		"processing_requests": processing,
		"success_rate":        successRate,
	}

	if avgTime != nil {
		stats["avg_processing_time_seconds"] = *avgTime
	}

	return stats, nil
}
