package telegram

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// codeBlockRe matches fenced code blocks: ```...``` with optional language tag.
var codeBlockRe = regexp.MustCompile("(?s)```[^\n`]*\n?(.*?)```")

// extractCodeBlock returns the trimmed content of the first ``` ``` block in text.
func extractCodeBlock(text string) string {
	m := codeBlockRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// modelKeywords maps lowercase substrings found in channel post text to model IDs.
var modelKeywords = []struct {
	substr  string
	modelID string
}{
	{"nano banana pro", "google/nano-banana-pro"},
	{"nano banana 2", "nano-banana-2"},
	{"nano banana", "google/nano-banana"},
	{"kling motion", "kling-2.6/motion-control"},
	{"motion control", "kling-2.6/motion-control"},
	{"veo3", "veo3_fast"},
	{"veo 3", "veo3_fast"},
	{"seedream", "seedream/4.5-edit"},
	{"seed dream", "seedream/4.5-edit"},
}

// detectModel tries to infer a model ID from a channel post body (e.g. footer "__Сделано с помощью Nano Banana 2__").
// Falls back to "google/nano-banana" when nothing matches.
func detectModel(text string) string {
	lower := strings.ToLower(text)
	for _, kw := range modelKeywords {
		if strings.Contains(lower, kw.substr) {
			return kw.modelID
		}
	}
	return "google/nano-banana"
}

// handleChannelPost is called for every ChannelPost update.
// It extracts the prompt (from a ``` ``` block) and the largest attached photo,
// saves the photo to the shared uploads directory, then inserts a gallery_idea row.
func (b *Bot) handleChannelPost(msg *tgmodels.Message) {
	if b.cfg.ChannelSyncID == 0 {
		return
	}
	if msg.Chat.ID != b.cfg.ChannelSyncID {
		return
	}

	// Prefer caption (photo post), fall back to text (text post)
	text := msg.Caption
	if text == "" {
		text = msg.Text
	}
	if text == "" {
		return
	}

	prompt := extractCodeBlock(text)
	if prompt == "" {
		// No code block → not a template post, skip silently
		return
	}

	model := detectModel(text)
	sourceID := fmt.Sprintf("tg_channel_%d", msg.ID)

	if b.db == nil {
		log.Printf("[channel_sync] db not configured, skipping message_id=%d", msg.ID)
		return
	}

	// Deduplication: skip if already imported
	var exists bool
	_ = b.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM gallery_ideas WHERE source_id = $1)`, sourceID).Scan(&exists)
	if exists {
		log.Printf("[channel_sync] duplicate, skipping message_id=%d", msg.ID)
		return
	}

	// Require a photo
	if len(msg.Photo) == 0 {
		log.Printf("[channel_sync] message_id=%d has no photo, skipping", msg.ID)
		return
	}

	// Pick the largest resolution variant
	largest := msg.Photo[len(msg.Photo)-1]
	photoURL, err := b.downloadAndSaveTelegramFile(largest.FileID, msg.ID)
	if err != nil {
		log.Printf("[channel_sync] photo download failed for message_id=%d: %v", msg.ID, err)
		return
	}

	_, err = b.db.Exec(`
		INSERT INTO gallery_ideas (model, output, prompt, source_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (source_id) DO NOTHING
	`, model, photoURL, prompt, sourceID)
	if err != nil {
		log.Printf("[channel_sync] insert error for message_id=%d: %v", msg.ID, err)
		return
	}

	log.Printf("[channel_sync] saved gallery idea from message_id=%d model=%s url=%s", msg.ID, model, photoURL)
}

// downloadAndSaveTelegramFile downloads a Telegram file by fileID,
// saves it to b.uploadDir, and returns the public URL.
func (b *Bot) downloadAndSaveTelegramFile(fileID string, msgID int) (string, error) {
	if b.uploadDir == "" {
		return "", fmt.Errorf("UPLOAD_DIR not configured")
	}
	if b.webBaseURL == "" {
		return "", fmt.Errorf("WEB_BASE_URL not configured")
	}

	tgFile, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("getFile: %w", err)
	}

	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.cfg.TelegramToken, tgFile.FilePath)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("http.Get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("telegram file server returned %d", resp.StatusCode)
	}

	ext := filepath.Ext(tgFile.FilePath)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("channel_%d_%d%s", msgID, time.Now().UnixNano(), ext)
	dst := filepath.Join(b.uploadDir, filename)

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("os.Create: %w", err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("io.Copy: %w", err)
	}

	base := strings.TrimRight(b.webBaseURL, "/")
	return base + "/api/uploads/" + filename, nil
}
