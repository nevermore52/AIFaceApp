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
	"unicode/utf16"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// codeBlockRe is a fallback for when no "pre" entity is present.
var codeBlockRe = regexp.MustCompile("(?s)```[^\n`]*\n?(.*?)```")

// extractCodeBlockFromMessage extracts the first "pre" (code block) entity from a Telegram
// message, using CaptionEntities (photo) or Entities (text). Offset/Length are UTF-16
// code units per the Telegram API spec.
// Falls back to regex on the raw text if no entity is found.
func extractCodeBlockFromMessage(text string, entities []tgmodels.MessageEntity) string {
	// Primary: use the "pre" entity to locate exact byte range
	for _, e := range entities {
		if e.Type != tgmodels.MessageEntityTypePre {
			continue
		}
		// Convert string → UTF-16 to work with Telegram's offset/length
		utf16Text := utf16.Encode([]rune(text))
		start := e.Offset
		end := e.Offset + e.Length
		if start < 0 || end > len(utf16Text) || start >= end {
			continue
		}
		extracted := string(utf16.Decode(utf16Text[start:end]))
		extracted = strings.TrimSpace(extracted)
		if extracted != "" {
			return extracted
		}
	}
	// Fallback: raw backtick regex (handles case where entities are absent)
	m := codeBlockRe.FindStringSubmatch(text)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
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

// detectModel infers a model ID from post text.
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

// handleChannelPost handles ChannelPost updates (requires bot to be admin of the channel).
func (b *Bot) handleChannelPost(msg *tgmodels.Message) {
	if b.cfg.ChannelSyncID == 0 {
		return
	}
	if msg.Chat.ID != b.cfg.ChannelSyncID {
		log.Printf("[channel_sync] ignored post from chat_id=%d (expected %d)", msg.Chat.ID, b.cfg.ChannelSyncID)
		return
	}

	log.Printf("[channel_sync] received channel_post message_id=%d chat=%d", msg.ID, msg.Chat.ID)
	b.processChannelMessage(msg, fmt.Sprintf("tg_channel_%d", msg.ID))
}

// TryHandleForwardedChannelPost processes a message forwarded from the channel by any user.
// Used for testing WITHOUT the bot being a channel admin: admin just forwards a post.
// Returns true if the message was handled (regardless of success).
func (b *Bot) TryHandleForwardedChannelPost(msg *tgmodels.Message) bool {
	if b.cfg.ChannelSyncID == 0 {
		return false
	}
	if msg.ForwardOrigin == nil {
		return false
	}
	if msg.ForwardOrigin.Type != tgmodels.MessageOriginTypeChannel {
		return false
	}
	ch := msg.ForwardOrigin.MessageOriginChannel
	if ch == nil || ch.Chat.ID != b.cfg.ChannelSyncID {
		return false
	}

	// Only admins can trigger manual import via forwarding
	if isAdmin, err := b.userService.IsUserAdmin(msg.From.ID); err != nil || !isAdmin {
		b.sendText(msg.Chat.ID, "⛔ Только администраторы могут импортировать посты через пересылку.")
		return true
	}

	log.Printf("[channel_sync] forwarded post from message_id=%d by admin=%d", ch.MessageID, msg.From.ID)
	sourceID := fmt.Sprintf("tg_channel_%d", ch.MessageID)
	ok := b.processChannelMessage(msg, sourceID)
	if ok {
		b.sendText(msg.Chat.ID, "✅ Пост добавлен в Идеи на сайте.")
	} else {
		b.sendText(msg.Chat.ID, "❌ Не удалось добавить пост. Проверь логи (нужен код-блок ``` и фото).")
	}
	return true
}

// processChannelMessage is the shared processing core.
// msg may be a ChannelPost OR a forwarded message from the channel.
// sourceID is the deduplication key.
// Returns true on success.
func (b *Bot) processChannelMessage(msg *tgmodels.Message, sourceID string) bool {
	// Pick text + entities: caption for media posts, text for text-only posts
	text := msg.Caption
	entities := msg.CaptionEntities
	if text == "" {
		text = msg.Text
		entities = msg.Entities
	}
	if text == "" {
		log.Printf("[channel_sync] %s: no text/caption", sourceID)
		return false
	}

	log.Printf("[channel_sync] %s: text_len=%d entities=%d photo=%d", sourceID, len(text), len(entities), len(msg.Photo))

	prompt := extractCodeBlockFromMessage(text, entities)
	if prompt == "" {
		log.Printf("[channel_sync] %s: no code block found", sourceID)
		return false
	}

	model := detectModel(text)

	if b.db == nil {
		log.Printf("[channel_sync] %s: db is nil", sourceID)
		return false
	}

	// Deduplication
	var exists bool
	_ = b.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM gallery_ideas WHERE source_id = $1)`, sourceID).Scan(&exists)
	if exists {
		log.Printf("[channel_sync] %s: duplicate, skipping", sourceID)
		return false
	}

	if len(msg.Photo) == 0 {
		log.Printf("[channel_sync] %s: no photo attached", sourceID)
		return false
	}

	largest := msg.Photo[len(msg.Photo)-1]
	photoURL, err := b.downloadAndSaveTelegramFile(largest.FileID, sourceID)
	if err != nil {
		log.Printf("[channel_sync] %s: photo download error: %v", sourceID, err)
		return false
	}

	// Duplicate check already done above via SELECT EXISTS; plain INSERT is safe here.
	_, err = b.db.Exec(`
		INSERT INTO gallery_ideas (model, output, prompt, source_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, model, photoURL, prompt, sourceID)
	if err != nil {
		log.Printf("[channel_sync] %s: db insert error: %v", sourceID, err)
		return false
	}

	log.Printf("[channel_sync] %s: saved model=%s url=%s", sourceID, model, photoURL)
	return true
}

// downloadAndSaveTelegramFile downloads a Telegram file by fileID,
// saves it to b.uploadDir, and returns the public URL.
func (b *Bot) downloadAndSaveTelegramFile(fileID, sourceID string) (string, error) {
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
	resp, err := http.Get(downloadURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("http.Get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("telegram returned %d", resp.StatusCode)
	}

	ext := filepath.Ext(tgFile.FilePath)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("channel_%s_%d%s", sourceID, time.Now().UnixNano(), ext)
	dst := filepath.Join(b.uploadDir, filename)

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("os.Create %s: %w", dst, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("io.Copy: %w", err)
	}

	base := strings.TrimRight(b.webBaseURL, "/")
	return base + "/api/uploads/" + filename, nil
}
