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
	db          *sql.DB
	client      *openrouter.Client
	defAPI      *defapi.Client
	kieAPI      *kieapi.Client
	http        *http.Client
	notify      func(chatID int64, req *models.GenerationRequest)
	userService *UserService

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
	VideoURLs          []string // Reference video URLs (for motion-control)
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
	PhotoResolution    string
	GoogleSearch       string
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
	// nano-banana-2 and gpt-image-2 support text-only generation (empty images allowed)
	modelLower := strings.ToLower(strings.TrimSpace(opts.Model))
	if len(images) == 0 && modelLower != "nano-banana-2" && modelLower != "gpt-image-2" {
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

	// nano-banana-2 — KieAPI task with resolution and google_search parameters
	if strings.EqualFold(strings.TrimSpace(opts.Model), "nano-banana-2") {
		taskID, err := s.createNanoBanana2Task(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration NanoBanana2 FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration NanoBanana2 task created: requestID=%d taskID=%s", req.ID, taskID)
		// Start timeout checker: 3 minutes for photo models
		s.startKieAPITimeoutChecker(req.ID, taskID, 3*time.Minute, opts.ChatID)
		return
	}

	// gpt-image-2 — KieAPI task; photo optional (text-to-image or image-to-image variant)
	if strings.EqualFold(strings.TrimSpace(opts.Model), "gpt-image-2") {
		taskID, err := s.createGPTImage2Task(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration GPTImage2 FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration GPTImage2 task created: requestID=%d taskID=%s", req.ID, taskID)
		s.startKieAPITimeoutChecker(req.ID, taskID, 3*time.Minute, opts.ChatID)
		return
	}

	// Nano Banana must never go to OpenRouter/PiAPI. Only DefAPI or KieAPI.
	if strings.EqualFold(strings.TrimSpace(opts.Model), "google/nano-banana") || strings.EqualFold(strings.TrimSpace(opts.Model), "google/nano-banana-pro") || strings.EqualFold(strings.TrimSpace(opts.Model), "seedream/4.5-edit") {
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
		// Start timeout checker: 3 minutes for photo models
		s.startKieAPITimeoutChecker(req.ID, taskID, 3*time.Minute, opts.ChatID)
		return
	}

	// veo3_fast — KieAPI video task with top-level payload
	if strings.EqualFold(strings.TrimSpace(opts.Model), "veo3_fast") {
		taskID, err := s.createKieAPIVideoTask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration KieAPI Video FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration KieAPI Video task created: requestID=%d taskID=%s", req.ID, taskID)
		return
	}

	// wan/2-6-image-to-video — KieAPI task with duration parameter
	if strings.EqualFold(strings.TrimSpace(opts.Model), "wan/2-6-image-to-video") {
		taskID, err := s.createWanVideoTask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration Wan Video FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration Wan Video task created: requestID=%d taskID=%s", req.ID, taskID)
		// Start timeout checker: 6 minutes for video models
		s.startKieAPITimeoutChecker(req.ID, taskID, 6*time.Minute, opts.ChatID)
		return
	}

	// kling-2.6/image-to-video — KieAPI task with duration and sound parameters
	if strings.EqualFold(strings.TrimSpace(opts.Model), "kling-2.6/image-to-video") {
		taskID, err := s.createKlingVideoTask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration Kling Video FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration Kling Video task created: requestID=%d taskID=%s", req.ID, taskID)
		// Start timeout checker: 6 minutes for kling video models
		s.startKieAPITimeoutChecker(req.ID, taskID, 6*time.Minute, opts.ChatID)
		return
	}

	// kling-2.6/motion-control — KieAPI task with input_urls, video_urls, mode, duration
	if strings.EqualFold(strings.TrimSpace(opts.Model), "kling-2.6/motion-control") {
		taskID, err := s.createKlingMotionTask(req.ID, opts, images)
		if err != nil {
			debugLog("processGeneration Kling Motion FAILED: requestID=%d, error=%v", req.ID, err)
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
		debugLog("processGeneration Kling Motion task created: requestID=%d taskID=%s", req.ID, taskID)
		s.startKieAPITimeoutChecker(req.ID, taskID, 6*time.Minute, opts.ChatID)
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
		// Start timeout checker: 3 minutes for photo models
		s.startKieAPITimeoutChecker(req.ID, taskID, 3*time.Minute, opts.ChatID)
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

func (s *GenerationService) SetUserService(us *UserService) {
	s.userService = us
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
	return model == "kie/nano-banana-edit" || model == "kie/nano-banana-pro" || model == "nano-banana-pro" || model == "seedream/4.5-edit" || model == "veo3_fast"
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
	case "seedream/4.5-edit":
		return "seedream/4.5-edit"
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
	if modelID == "seedream/4.5-edit" {
		input["quality"] = "basic"
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

func (s *GenerationService) createNanoBanana2Task(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	input := map[string]any{
		"prompt": opts.Prompt,
	}
	if opts.AspectRatio != "" {
		input["aspect_ratio"] = opts.AspectRatio
	}
	if len(images) > 0 {
		input["image_input"] = images
	} else {
		input["image_input"] = []string{}
	}

	// Resolution: 1K, 2K, 4K
	resolution := strings.TrimSpace(opts.PhotoResolution)
	if resolution == "" {
		resolution = "1K"
	}
	input["resolution"] = resolution

	// Google Search: true/false
	googleSearch := false
	if strings.TrimSpace(opts.GoogleSearch) == "true" {
		googleSearch = true
	}
	input["google_search"] = googleSearch

	payload := kieapi.CreateTaskRequest{
		Model:       "nano-banana-2",
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

func (s *GenerationService) createGPTImage2Task(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	input := map[string]any{
		"prompt":       opts.Prompt,
		"nsfw_checker": false,
	}

	aspect := strings.TrimSpace(opts.AspectRatio)
	switch aspect {
	case "1:1", "16:9", "9:16":
		input["aspect_ratio"] = aspect
	default:
		input["aspect_ratio"] = "1:1"
	}

	apiModel := "gpt-image-2-text-to-image"
	if len(images) > 0 {
		apiModel = "gpt-image-2-image-to-image"
		input["input_urls"] = images
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

func (s *GenerationService) createKieAPIVideoTask(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	payload := map[string]any{
		"prompt":            opts.Prompt,
		"imageUrls":         images,
		"model":             "veo3_fast",
		"watermark":         "",
		"callBackUrl":       callbackURL,
		"seeds":             12345,
		"enableFallback":    false,
		"enableTranslation": true,
		"generationType":    "REFERENCE_2_VIDEO",
	}
	// aspect_ratio: отправляем только если явно выбран (16:9, 9:16). "auto" = не отправляем.
	if opts.AspectRatio != "" && opts.AspectRatio != "auto" {
		payload["aspect_ratio"] = opts.AspectRatio
	}

	taskID, err := s.kieAPI.CreateVeoTask(payload)
	if err != nil {
		return "", err
	}
	if err := s.updateExternalTaskID(requestID, taskID); err != nil {
		return "", err
	}
	return taskID, nil
}

func (s *GenerationService) createKlingVideoTask(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	// Extract duration and sound from opts
	// Duration is passed via AspectRatio field
	// Sound is passed via NanoBananaProvider field temporarily
	duration := "5"
	sound := false

	if opts.AspectRatio != "" {
		duration = opts.AspectRatio
	}
	if opts.NanoBananaProvider == "true" {
		sound = true
	}

	input := map[string]any{
		"duration": duration,
		"sound":    sound,
	}
	if len(images) > 0 {
		input["image_urls"] = images
	}
	if opts.Prompt != "" {
		input["prompt"] = opts.Prompt
	}

	payload := kieapi.CreateTaskRequest{
		Model:       "kling-2.6/image-to-video",
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

func (s *GenerationService) createKlingMotionTask(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	// Duration via AspectRatio field, mode (720p/1080p) via NanoBananaProvider field
	duration := "5"
	mode := "720p"

	if opts.AspectRatio != "" {
		duration = opts.AspectRatio
	}
	if opts.NanoBananaProvider != "" {
		mode = opts.NanoBananaProvider
	}

	input := map[string]any{
		"duration":              duration,
		"mode":                  mode,
		"character_orientation": "video",
	}
	if len(images) > 0 {
		input["input_urls"] = images
	}
	if len(opts.VideoURLs) > 0 {
		input["video_urls"] = opts.VideoURLs
	}
	if opts.Prompt != "" {
		input["prompt"] = opts.Prompt
	}

	payload := kieapi.CreateTaskRequest{
		Model:       "kling-2.6/motion-control",
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

func (s *GenerationService) createWanVideoTask(requestID int64, opts GenerationOptions, images []string) (string, error) {
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

	// Extract duration and resolution from opts
	// Duration is passed via AspectRatio field
	// Resolution is passed via NanoBananaProvider field temporarily
	duration := "5"
	resolution := "720p"

	if opts.AspectRatio != "" {
		duration = opts.AspectRatio
	}
	if opts.NanoBananaProvider != "" {
		resolution = opts.NanoBananaProvider
	}

	input := map[string]any{
		"duration":   duration,
		"resolution": resolution,
	}
	if len(images) > 0 {
		input["image_urls"] = images
	}
	if opts.Prompt != "" {
		input["prompt"] = opts.Prompt
	}

	payload := kieapi.CreateTaskRequest{
		Model:       "wan/2-6-image-to-video",
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

	// Получаем telegram_id из таблицы users по user_id
	var telegramID int64
	if err := s.db.QueryRow(`SELECT telegram_id FROM users WHERE id = $1`, req.UserID).Scan(&telegramID); err != nil {
		debugLog("HandleKieAPICallback: failed to get telegram_id for user_id=%d: %v", req.UserID, err)
		// Продолжаем без уведомления, но обновляем статус
		telegramID = 0
	}

	status := strings.ToLower(strings.TrimSpace(payload.StatusValue()))
	if status != "success" && status != "completed" && status != "succeeded" {
		reason := strings.TrimSpace(payload.Msg)
		if reason == "" {
			reason = fmt.Sprintf("kieapi status=%s", payload.StatusValue())
		}
		_ = s.updateRequestStatus(req.ID, "failed", reason)

		// Возврат токенов пользователю при ошибке
		if req.TokensUsed > 0 && req.UserID != 0 && s.userService != nil {
			category := models.QuotaCategoryImage
			if req.ModelType == "video" {
				category = models.QuotaCategoryVideo
			}
			if err := s.userService.RefundQuota(req.UserID, category, req.TokensPrimaryUsed, req.TokensExtraUsed); err != nil {
				debugLog("HandleKieAPICallback: refund quota error for user_id=%d: %v", req.UserID, err)
			}
			if err := s.ResetRequestTokensUsed(req.ID); err != nil {
				debugLog("HandleKieAPICallback: reset tokens error for request_id=%d: %v", req.ID, err)
			}
		}

		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil && telegramID != 0 {
			s.notify(telegramID, req)
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

		// Возврат токенов пользователю при ошибке
		if req.TokensUsed > 0 && req.UserID != 0 && s.userService != nil {
			category := models.QuotaCategoryImage
			if req.ModelType == "video" {
				category = models.QuotaCategoryVideo
			}
			if err := s.userService.RefundQuota(req.UserID, category, req.TokensPrimaryUsed, req.TokensExtraUsed); err != nil {
				debugLog("HandleKieAPICallback: refund quota error for user_id=%d: %v", req.UserID, err)
			}
			if err := s.ResetRequestTokensUsed(req.ID); err != nil {
				debugLog("HandleKieAPICallback: reset tokens error for request_id=%d: %v", req.ID, err)
			}
		}

		req.Status = "failed"
		req.ErrorMsg = &reason
		req.Output = nil
		req.CompletedAt = nil
		if s.notify != nil && telegramID != 0 {
			s.notify(telegramID, req)
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
	if s.notify != nil && telegramID != 0 {
		s.notify(telegramID, req)
	}
	s.markDone(req.ID)
	return nil
}

// startKieAPITimeoutChecker starts a goroutine that waits for the specified timeout
// and then polls the KieAPI for task status if the request is still pending.
// If task is still processing after first poll, waits another timeout period and retries once.
// timeout: 3 minutes for photo models, 6 minutes for video models (kling)
func (s *GenerationService) startKieAPITimeoutChecker(requestID int64, taskID string, timeout time.Duration, chatID int64) {
	go func() {
		for attempt := 1; attempt <= 2; attempt++ {
			time.Sleep(timeout)

			// Check if request is still pending/processing
			req, err := s.GetGenerationRequest(requestID)
			if err != nil {
				debugLog("KieAPI timeout checker [attempt %d]: failed to get request %d: %v", attempt, requestID, err)
				return
			}

			// If already completed or failed, do nothing
			if req.Status == "completed" || req.Status == "failed" {
				debugLog("KieAPI timeout checker [attempt %d]: request %d already %s, skipping", attempt, requestID, req.Status)
				return
			}

			debugLog("KieAPI timeout checker [attempt %d]: request %d still %s after %v, polling recordInfo for task %s", attempt, requestID, req.Status, timeout, taskID)

			// Poll KieAPI for task status
			if s.kieAPI == nil {
				debugLog("KieAPI timeout checker [attempt %d]: kieAPI client is nil", attempt)
				return
			}

			info, err := s.kieAPI.GetRecordInfo(taskID)
			if err != nil {
				debugLog("KieAPI timeout checker [attempt %d]: GetRecordInfo failed for task %s: %v", attempt, taskID, err)
				// Don't fail the request, callback might still come or we retry
				continue
			}

			// Convert to callback payload and check status
			payload := info.ToCallbackPayload()
			status := strings.ToLower(strings.TrimSpace(payload.StatusValue()))
			debugLog("KieAPI timeout checker [attempt %d]: got recordInfo for task %s, status=%s", attempt, taskID, status)

			// If still processing, continue to next attempt (will wait another timeout period)
			if status == "processing" || status == "pending" || status == "queued" || status == "" {
				debugLog("KieAPI timeout checker [attempt %d]: task %s still processing, will retry after %v", attempt, taskID, timeout)
				continue
			}

			// Process the response using existing callback handler logic
			if err := s.HandleKieAPICallback(payload); err != nil {
				debugLog("KieAPI timeout checker [attempt %d]: HandleKieAPICallback failed for task %s: %v", attempt, taskID, err)
			}
			return
		}
		debugLog("KieAPI timeout checker: exhausted all attempts for request %d, task %s", requestID, taskID)
	}()
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
	return model == "google/nano-banana" || model == "google/nano-banana-pro" || model == "seedream/4.5-edit"
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

func (s *GenerationService) ResetRequestTokensUsed(requestID int64) error {
	_, err := s.db.Exec(`
		UPDATE generation_requests
		SET tokens_used = 0,
			tokens_primary_used = 0,
			tokens_extra_used = 0
		WHERE id = $1
	`, requestID)
	return err
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

type TopUserStats struct {
	UserID           int64
	Username         string
	FirstName        string
	TotalGenerations int
	PhotoGenerations int
	VideoGenerations int
	MusicGenerations int
	TextGenerations  int
	TokensSpent      int
	PhotoTokensSpent int
	PhotoRubles      float64
}

func (s *GenerationService) GetTopUsersByDailyGenerations(limit int) ([]TopUserStats, error) {
	from := time.Now().UTC().Add(-24 * time.Hour)

	query := `
		SELECT 
			gr.user_id,
			COALESCE(gr.username, '') as username,
			COALESCE(u.first_name, '') as first_name,
			COUNT(*) as total_generations,
			COUNT(CASE WHEN gr.model_type = 'photo' THEN 1 END) as photo_generations,
			COUNT(CASE WHEN gr.model_type = 'video' THEN 1 END) as video_generations,
			COUNT(CASE WHEN gr.model_type = 'music' THEN 1 END) as music_generations,
			COUNT(CASE WHEN gr.model_type = 'chat' THEN 1 END) as text_generations,
			COALESCE(SUM(gr.tokens_used), 0) as tokens_spent,
			COALESCE(SUM(CASE WHEN gr.model_type = 'photo' THEN gr.tokens_used ELSE 0 END), 0) as photo_tokens_spent
		FROM generation_requests gr
		LEFT JOIN users u ON gr.user_id = u.telegram_id
		WHERE gr.created_at >= $1
		GROUP BY gr.user_id, gr.username, u.first_name
		ORDER BY total_generations DESC
		LIMIT $2`

	rows, err := s.db.Query(query, from, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TopUserStats
	for rows.Next() {
		var stat TopUserStats
		err := rows.Scan(
			&stat.UserID,
			&stat.Username,
			&stat.FirstName,
			&stat.TotalGenerations,
			&stat.PhotoGenerations,
			&stat.VideoGenerations,
			&stat.MusicGenerations,
			&stat.TextGenerations,
			&stat.TokensSpent,
			&stat.PhotoTokensSpent,
		)
		if err != nil {
			return nil, err
		}

		// Convert photo tokens to rubles (1 token = 1.5 rubles)
		stat.PhotoRubles = float64(stat.PhotoTokensSpent) * 1.5

		results = append(results, stat)
	}

	return results, rows.Err()
}
