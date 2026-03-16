package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"telegram-ai-face-bot/internal/config"
	"telegram-ai-face-bot/internal/models"
	"telegram-ai-face-bot/internal/redis"
	"telegram-ai-face-bot/internal/services"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

type Bot struct {
	api               *tgbot.Bot
	ctx               context.Context
	cancel            context.CancelFunc
	botUser           *tgmodels.User
	userService       *services.UserService
	generationService *services.GenerationService
	paymentService    *services.PaymentService
	redisClient       *redis.Client
	cfg               *config.Config
	concurrencySem    chan struct{}
	shutdownOnce      sync.Once
	shuttingDown      atomic.Bool
	wg                sync.WaitGroup
	albumMu           sync.Mutex
	albumBuffers      map[string]*albumBuffer
	recentMu          sync.Mutex
	recentPhotos      map[int64][]photoRecord
	sunoMu            sync.Mutex
	sunoTasks         map[string]sunoTask
	sunoInstrMu       sync.Mutex
	sunoInstrumental  map[int64]bool
	sunoVoiceMu       sync.Mutex
	sunoVoice         map[int64]string
}

// --- Telegram helper constructors (replaces tgbotapi convenience functions) ---

// Button styles for Telegram Bot API 9.4+
const (
	ButtonStyleDefault = ""
	ButtonStylePrimary = "primary" // blue
	ButtonStyleDanger  = "danger"  // red
	ButtonStyleSuccess = "success" // green
)

// Custom emoji IDs
const (
	EmojiIDBack          = "5352759161945867747"
	EmojiIDBuy           = "5224257782013769471"
	EmojiIDGift          = "5203996991054432397"
	EmojiIDSelectModel   = "5271912827869737544"
	EmojiIDText          = "5429273196870254885"
	EmojiIDPhoto         = "5429200581858182504"
	EmojiIDAudio         = "5429372861586359061"
	EmojiIDPhotoCategory = "5235837920081887219"
	EmojiIDMenu          = "5416041192905265756"
)

func newInlineKeyboardButtonData(text, data string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data}
}

func newInlineKeyboardButtonDataStyled(text, data, style string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data, Style: style}
}

func newInlineKeyboardButtonDataWithEmoji(text, data, emojiID string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data, IconCustomEmojiID: emojiID}
}

func newInlineKeyboardButtonDataStyledWithEmoji(text, data, style, emojiID string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data, Style: style, IconCustomEmojiID: emojiID}
}

// newBackButton creates a styled back button with custom emoji
func newBackButton(text, data string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data, Style: ButtonStyleDanger, IconCustomEmojiID: EmojiIDBack}
}

func newInlineKeyboardButtonURL(text, url string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, URL: url}
}

func newInlineKeyboardRow(buttons ...tgmodels.InlineKeyboardButton) []tgmodels.InlineKeyboardButton {
	return buttons
}

func newInlineKeyboardMarkup(rows ...[]tgmodels.InlineKeyboardButton) tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}
func newKeyboardButton(text string) tgmodels.KeyboardButton {
	return tgmodels.KeyboardButton{Text: text}
}
func newKeyboardButtonEmoji(text, emojiId string) tgmodels.KeyboardButton {
	return tgmodels.KeyboardButton{Text: text, IconCustomEmojiID: emojiId}
}

func newKeyboardButtonRow(buttons ...tgmodels.KeyboardButton) []tgmodels.KeyboardButton {
	return buttons
}

func newReplyKeyboard(rows ...[]tgmodels.KeyboardButton) tgmodels.ReplyKeyboardMarkup {
	return tgmodels.ReplyKeyboardMarkup{
		Keyboard:       rows,
		ResizeKeyboard: true,
	}
}

// messageConfig mimics old tgbotapi.MessageConfig for easy migration
type messageConfig struct {
	ChatID                int64
	Text                  string
	ParseMode             string
	ReplyMarkup           interface{}
	DisableWebPagePreview bool
	DisableNotification   bool
}

func newMessageConfig(chatID int64, text string) messageConfig {
	return messageConfig{ChatID: chatID, Text: text}
}

func (b *Bot) sendMsg(msg messageConfig) (*tgmodels.Message, error) {
	params := &tgbot.SendMessageParams{
		ChatID:              msg.ChatID,
		Text:                msg.Text,
		ParseMode:           tgmodels.ParseMode(msg.ParseMode),
		DisableNotification: msg.DisableNotification,
	}
	if msg.DisableWebPagePreview {
		params.LinkPreviewOptions = &tgmodels.LinkPreviewOptions{IsDisabled: &msg.DisableWebPagePreview}
	}
	if msg.ReplyMarkup != nil {
		params.ReplyMarkup = msg.ReplyMarkup
	}
	return b.api.SendMessage(b.ctx, params)
}

// isCommand checks if the message is a bot command (starts with /)
func isCommand(msg *tgmodels.Message) bool {
	if msg == nil || msg.Text == "" {
		return false
	}
	return strings.HasPrefix(msg.Text, "/")
}

// getCommand extracts the command from a message (without the leading /)
func getCommand(msg *tgmodels.Message) string {
	if msg == nil || msg.Text == "" {
		return ""
	}
	text := msg.Text
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	text = text[1:] // remove leading /
	// Command ends at space or @ (for @botname)
	if idx := strings.IndexAny(text, " @"); idx != -1 {
		return text[:idx]
	}
	return text
}

// getCommandArgs extracts the arguments after the command
func getCommandArgs(msg *tgmodels.Message) string {
	if msg == nil || msg.Text == "" {
		return ""
	}
	text := msg.Text
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	// Find space after command
	if idx := strings.Index(text, " "); idx != -1 {
		return strings.TrimSpace(text[idx+1:])
	}
	return ""
}

// getCallbackMessage safely extracts Message from MaybeInaccessibleMessage
func getCallbackMessage(callback *tgmodels.CallbackQuery) *tgmodels.Message {
	if callback == nil {
		return nil
	}
	// MaybeInaccessibleMessage has a Message field that is a pointer
	return callback.Message.Message
}

// getCallbackChatID extracts chat ID from callback, handling MaybeInaccessibleMessage
func getCallbackChatID(callback *tgmodels.CallbackQuery) int64 {
	if callback == nil {
		return 0
	}
	if callback.Message.Message != nil {
		return callback.Message.Message.Chat.ID
	}
	if callback.Message.InaccessibleMessage != nil {
		return callback.Message.InaccessibleMessage.Chat.ID
	}
	return 0
}

// getCallbackMessageID extracts message ID from callback
func getCallbackMessageID(callback *tgmodels.CallbackQuery) int {
	if callback == nil {
		return 0
	}
	if callback.Message.Message != nil {
		return callback.Message.Message.ID
	}
	if callback.Message.InaccessibleMessage != nil {
		return callback.Message.InaccessibleMessage.MessageID
	}
	return 0
}

// getFileURL constructs the file download URL from a File object
func (b *Bot) getFileURL(file *tgmodels.File) string {
	if file == nil || file.FilePath == "" {
		return ""
	}
	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.cfg.TelegramToken, file.FilePath)
}

// sendPhoto sends a photo message
func (b *Bot) sendPhoto(chatID int64, photo tgmodels.InputFile, caption string, parseMode string, replyMarkup interface{}) (*tgmodels.Message, error) {
	params := &tgbot.SendPhotoParams{
		ChatID:  chatID,
		Photo:   photo,
		Caption: caption,
	}
	if parseMode != "" {
		params.ParseMode = tgmodels.ParseMode(parseMode)
	}
	if replyMarkup != nil {
		params.ReplyMarkup = replyMarkup
	}
	return b.api.SendPhoto(b.ctx, params)
}

// sendVideo sends a video message
func (b *Bot) sendVideo(chatID int64, video tgmodels.InputFile, caption string, parseMode string) (*tgmodels.Message, error) {
	params := &tgbot.SendVideoParams{
		ChatID:  chatID,
		Video:   video,
		Caption: caption,
	}
	if parseMode != "" {
		params.ParseMode = tgmodels.ParseMode(parseMode)
	}
	return b.api.SendVideo(b.ctx, params)
}

// sendAudio sends an audio message
func (b *Bot) sendAudio(chatID int64, audio tgmodels.InputFile, caption string) (*tgmodels.Message, error) {
	return b.api.SendAudio(b.ctx, &tgbot.SendAudioParams{
		ChatID:  chatID,
		Audio:   audio,
		Caption: caption,
	})
}

// sendDocument sends a document message
func (b *Bot) sendDocument(chatID int64, document tgmodels.InputFile, caption string) (*tgmodels.Message, error) {
	return b.api.SendDocument(b.ctx, &tgbot.SendDocumentParams{
		ChatID:   chatID,
		Document: document,
		Caption:  caption,
	})
}

// answerCallback answers a callback query
func (b *Bot) answerCallback(callbackID string, text string) error {
	_, err := b.api.AnswerCallbackQuery(b.ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	return err
}

// editMessageText edits the text of a message (with HTML parse mode)
func (b *Bot) editMessageText(chatID int64, messageID int, text string, markup *tgmodels.InlineKeyboardMarkup) error {
	params := &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, err := b.api.EditMessageText(b.ctx, params)
	return err
}

// editMessageCaption edits the caption of a message
func (b *Bot) editMessageCaption(chatID int64, messageID int, caption string, markup *tgmodels.InlineKeyboardMarkup) error {
	params := &tgbot.EditMessageCaptionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Caption:   caption,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, err := b.api.EditMessageCaption(b.ctx, params)
	return err
}

// deleteMessage deletes a message
func (b *Bot) deleteMessage(chatID int64, messageID int) error {
	_, err := b.api.DeleteMessage(b.ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	return err
}

// editMessageMedia edits the media of a message (photo/video)
func (b *Bot) editMessageMedia(chatID int64, messageID int, media tgmodels.InputMedia, markup *tgmodels.InlineKeyboardMarkup) error {
	params := &tgbot.EditMessageMediaParams{
		ChatID:    chatID,
		MessageID: messageID,
		Media:     media,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, err := b.api.EditMessageMedia(b.ctx, params)
	return err
}

// --- End Telegram helper constructors ---

// sendVideoTotalInfo показывает итоговую цену и инструкцию по отправке фото и промпта
func (b *Bot) sendVideoTotalInfo(chatID int64, userID int64, modelID string) {
	loc := b.getLocalization(userID)
	total := b.getVideoTotalCost(userID, modelID)
	if total < 1 {
		total = 1
	}
	modelLabel := b.modelLabelLoc(modelID, loc)
	text := fmt.Sprintf("Модель: %s\nСтоимость: %d %s\n\nОтправьте фото и в подписи укажите промпт (описание что вы хотите получить)",
		html.EscapeString(modelLabel),
		total,
		html.EscapeString(requestWord(total, loc)))

	msg := newMessageConfig(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send video total info: %v", err)
	}
}

func (b *Bot) isChatModelAllowed(userID int64, model ModelOption) bool {
	if model.ID != "google/gemini-3-flash" && model.ID != "openai/gpt-5-mini" && model.ID != "openai/gpt-5-nano" {
		return true
	}
	if isAdmin, err := b.userService.IsUserAdmin(userID); err == nil && isAdmin {
		return true
	}
	label, ok := b.userService.GetSubscriptionLabel(userID)
	if !ok {
		return false
	}
	label = strings.ToLower(label)
	switch model.ID {
	case "google/gemini-3-flash":
		return label == "start" || label == "pro"
	case "openai/gpt-5-mini", "openai/gpt-5-nano":
		return label == "mini" || label == "start" || label == "pro"
	default:
		return true
	}
}

func (b *Bot) sendMarkdownLong(chatID int64, text string) error {
	const maxChunkBytes = 3800
	text = strings.TrimSpace(convertBroadcastMarkupToHTML(text))
	for len(text) > 0 {
		if len(text) <= maxChunkBytes {
			msg := newMessageConfig(chatID, text)
			msg.ParseMode = "HTML"
			msg.DisableWebPagePreview = true
			_, err := b.sendMsg(msg)
			return err
		}

		cut := maxChunkBytes
		if cut > len(text) {
			cut = len(text)
		}

		if idx := strings.LastIndex(text[:cut], "\n"); idx > 0 && idx >= cut-600 {
			cut = idx
		} else if idx := strings.LastIndex(text[:cut], " "); idx > 0 && idx >= cut-300 {
			cut = idx
		}

		for cut > 0 && !utf8.ValidString(text[:cut]) {
			cut--
		}
		if cut == 0 {
			cut = maxChunkBytes
			for cut > 0 && !utf8.RuneStart(text[cut-1]) {
				cut--
			}
			if cut == 0 {
				cut = len(text)
			}
		}

		chunk := strings.TrimSpace(text[:cut])
		text = strings.TrimSpace(text[cut:])

		if chunk == "" {
			continue
		}
		msg := newMessageConfig(chatID, chunk)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		if _, err := b.sendMsg(msg); err != nil {
			return err
		}
	}
	return nil
}

func convertBroadcastMarkupToHTML(text string) string {
	text = html.EscapeString(text)

	codeBlockRe := regexp.MustCompile("(?s)```(.*?)```")
	text = codeBlockRe.ReplaceAllString(text, "<pre>$1</pre>")

	inlineCodeRe := regexp.MustCompile("`([^`\n]+)`")
	text = inlineCodeRe.ReplaceAllString(text, "<code>$1</code>")

	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	text = boldRe.ReplaceAllString(text, "<b>$1</b>")

	italicRe := regexp.MustCompile(`__(.+?)__`)
	text = italicRe.ReplaceAllString(text, "<i>$1</i>")

	strikeRe := regexp.MustCompile(`~~(.+?)~~`)
	text = strikeRe.ReplaceAllString(text, "<s>$1</s>")

	return text
}

func (b *Bot) sendStartTrialMenu(chatID int64) {
	btn := newInlineKeyboardButtonData("Проверить подписку", "trial:check")
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(btn),
	)
	text := fmt.Sprintf("Чтобы получить 1 пробную генерацию фото — подпишитесь на канал %s\nи нажмите «Проверить подписку»\n%s", requiredChannelUsername, requiredChannelLink)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("sendStartTrialMenu send message error: %v", err)
	}
}

func (b *Bot) chatModelAccessMessage(modelID string) string {
	switch modelID {
	case "google/gemini-3-flash":
		return "Модель доступна только с подпиской Start или Pro"
	case "openai/gpt-5-mini", "openai/gpt-5-nano":
		return "Модель доступна только с подпиской Mini, Start или Pro"
	default:
		return "Модель доступна только с активной подпиской"
	}
}

func (b *Bot) extrasPrice(category string, qty int) (int, bool) {
	if b.paymentService == nil {
		return 0, false
	}
	return b.paymentService.ExtrasPrice(category, qty)
}

func (b *Bot) getPhotoDiscountPercent() int {
	if b.redisClient == nil {
		return 0
	}
	discount, err := b.redisClient.GetPhotoDiscount()
	if err != nil || discount == nil {
		return 0
	}
	return discount.Percent
}

func (b *Bot) formatExtrasPriceMarkdownV2(category string, currentPrice int) string {
	if currentPrice < 0 {
		currentPrice = 0
	}
	if category == "image" {
		discountPercent := b.getPhotoDiscountPercent()
		if discountPercent > 0 && discountPercent < 100 {
			// Calculate discounted price
			discountedPrice := currentPrice * (100 - discountPercent) / 100
			return fmt.Sprintf("%d ₽ <s>%d ₽</s>", discountedPrice, currentPrice)
		}
	}
	return fmt.Sprintf("%d ₽", currentPrice)
}

func (b *Bot) extrasDiscountedPrice(category string, currentPrice int) int {
	if currentPrice < 0 {
		currentPrice = 0
	}
	if category == "image" {
		discountPercent := b.getPhotoDiscountPercent()
		if discountPercent > 0 && discountPercent < 100 {
			return currentPrice * (100 - discountPercent) / 100
		}
	}
	return currentPrice
}

func escapeMarkdownV2(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}

func (b *Bot) getUserAspectRatio(userID int64) string {
	ratio, err := b.redisClient.GetUserAspectRatio(userID)
	if err != nil {
		log.Printf("getUserAspectRatio err: %v", err)
	}
	if ratio == "" {
		return "1:1"
	}
	switch ratio {
	case "16:9", "9:16", "1:1", "auto":
		return ratio
	default:
		return "1:1"
	}
}

// getAspectRatioForModel возвращает формат, допустимый для категории модели.
// Фото: 16:9, 9:16, 1:1 (auto → 1:1). Видео: 16:9, 9:16, auto (1:1 → auto).
func (b *Bot) getAspectRatioForModel(userID int64, modelID string) string {
	ratio := b.getUserAspectRatio(userID)
	opt, ok := findModelOption(modelID)
	if !ok {
		return ratio
	}
	if opt.Category == ModelCategoryVideo {
		if ratio == "1:1" {
			return "auto"
		}
	} else {
		if ratio == "auto" {
			return "1:1"
		}
	}
	return ratio
}

func (b *Bot) setUserAspectRatio(userID int64, ratio string) {
	switch ratio {
	case "16:9", "9:16", "1:1", "auto":
		// valid
	default:
		ratio = "1:1"
	}
	if err := b.redisClient.SetUserAspectRatio(userID, ratio); err != nil {
		log.Printf("setUserAspectRatio err: %v", err)
	}
}

func (b *Bot) sendNanoBananaAPIStatus(chatID int64) {
	provider, err := b.userService.GetNanoBananaProvider()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось получить состояние Фото API")
		return
	}
	state := "KieAPI"
	toggleLabel := "Переключить на DefAPI"
	if strings.EqualFold(provider, "defapi") {
		state = "DefAPI"
		toggleLabel = "Переключить на KieAPI"
	}
	text := fmt.Sprintf("📸 Фото API: %s", state)
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData(toggleLabel, "admin:nano_api_toggle"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send photo api status: %v", err)
	}
}

func (b *Bot) toggleNanoBananaAPI(chatID int64) {
	provider, err := b.userService.GetNanoBananaProvider()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось проверить Фото API")
		return
	}
	next := "defapi"
	if strings.EqualFold(provider, "defapi") {
		next = "kieapi"
	}
	if err := b.userService.SetNanoBananaProvider(next); err != nil {
		b.sendErrorMessage(chatID, "Не удалось переключить Фото API")
		return
	}
	b.sendNanoBananaAPIStatus(chatID)
}

type albumBuffer struct {
	chatID  int64
	userID  int64
	photos  []albumPhoto
	caption string
	timer   *time.Timer
	created time.Time
}

type albumPhoto struct {
	msgID int
	photo tgmodels.PhotoSize
}

type photoRecord struct {
	URL      string
	UniqueID string
	Time     time.Time
}

type sunoTask struct {
	ChatID      int64
	UserID      int64
	RequestID   int64
	RequestCost int
	PrimaryUsed int
	ExtraUsed   int
	ModelLabel  string
	Prompt      string
	CreatedAt   time.Time
	AudioURLs   []string
	FirstAudio  time.Time
	Timer       *time.Timer
}

func (b *Bot) Config() *config.Config {
	return b.cfg
}

func (b *Bot) sendBuyExtrasCategory(chatID int64, userID int64, category string, callbackPrefix string) {
	loc := b.getLocalization(userID)
	packs := []int{10, 50, 100, 250, 500}

	if category == "" {
		parts := strings.Split(callbackPrefix, ":")
		if len(parts) >= 2 && parts[1] != "" {
			category = parts[1]
		} else {
			category = "text"
		}
	}

	if category == "music" {
		packs = []int{1, 5, 10, 50, 100}
	} else if category == "video" {
		packs = []int{1, 5, 10, 25, 50, 100}
	}

	var header, unit string
	switch category {
	case "image":
		header = loc.ExtrasImage
		unit = loc.BuyPackageLabelImage
	case "music":
		header = loc.ExtrasMusico
		unit = loc.BuyPackageLabelMusic
	case "video":
		header = loc.ExtrasVideos
		unit = loc.BuyPackageLabelVideo
	default:
		header = loc.ExtrasTexts
		unit = loc.BuyPackageLabelText
	}

	var sb strings.Builder
	sb.WriteString(html.EscapeString(header) + "\n\n" + html.EscapeString(loc.BuySelectAction) + "\n")
	escapedUnit := html.EscapeString(unit)
	for _, p := range packs {
		if price, ok := b.extrasPrice(category, p); ok {
			effectivePrice := b.extrasDiscountedPrice(category, price)
			perItem := float64(effectivePrice) / float64(p)
			sb.WriteString(fmt.Sprintf("• %d %s — %s (%0.1f ₽/шт)\n", p, escapedUnit, b.formatExtrasPriceMarkdownV2(category, price), perItem))
		} else {
			sb.WriteString(fmt.Sprintf("• %d %s\n", p, escapedUnit))
		}
	}
	text := sb.String() + "\n" + html.EscapeString(loc.BuyConsentNote)

	var rows [][]tgmodels.InlineKeyboardButton
	for i := 0; i < len(packs); i += 2 {
		btn1 := newInlineKeyboardButtonDataStyled(fmt.Sprintf("%d", packs[i]), fmt.Sprintf("%s:%d", callbackPrefix, packs[i]), "success")
		if i+1 < len(packs) {
			btn2 := newInlineKeyboardButtonDataStyled(fmt.Sprintf("%d", packs[i+1]), fmt.Sprintf("%s:%d", callbackPrefix, packs[i+1]), "success")
			rows = append(rows, newInlineKeyboardRow(btn1, btn2))
		} else {
			rows = append(rows, newInlineKeyboardRow(btn1))
		}
	}
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn, "buy:extras"),
	))

	keyboard := newInlineKeyboardMarkup(rows...)
	reply := newMessageConfig(chatID, text)
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = keyboard

	if _, err := b.sendMsg(reply); err != nil {
		log.Printf("Failed to send buy extras category: %v", err)
	}
}

func (b *Bot) sendBuyPackageInfo(chatID int64, userID int64, category string, pack string) {
	if b.paymentService == nil {
		b.sendErrorMessage(chatID, "Платёжный сервис не настроен")
		return
	}
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}

	qty, err := strconv.Atoi(pack)
	if err != nil || qty <= 0 {
		b.sendErrorMessage(chatID, "Неверный пакет")
		return
	}

	// Проверяем включенность категории
	switch category {
	case "text":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryChat) {
			return
		}
	case "image":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryPhoto) {
			return
		}
	case "music":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryMusic) {
			return
		}
	case "video":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
			return
		}
	}

	loc := b.getLocalization(userID)
	resp, err := b.paymentService.CreateExtrasPayment(userID, category, qty)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf(loc.ErrCreatePayment, err))
		return
	}

	label := loc.BuyPackageLabelText
	switch category {
	case "text":
		label = loc.BuyPackageLabelText
	case "image":
		label = loc.BuyPackageLabelImage
	case "music":
		label = loc.BuyPackageLabelMusic
	case "video":
		label = loc.BuyPackageLabelVideo
	}

	text := fmt.Sprintf(loc.BuyPackageTitle, label, qty, resp.CheckoutURL) + "\n\n" + loc.BuyConsentNote

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newBackButton(loc.BuyPackageBackBtn, "buy:extras"),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send buy package info: %v", err)
	}
}

// Admin: категории
func (b *Bot) sendAdminCategories(chatID int64) {
	settings, err := b.userService.GetCategorySettings()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось получить категории")
		return
	}

	lines := []string{"⚙️ Категории:"}
	rows := [][]tgmodels.InlineKeyboardButton{}
	for _, s := range settings {
		state := "❌"
		if s.Enabled {
			state = "✅"
		}
		label := fmt.Sprintf("%s %s", state, categoryLabelByKey(s.Category))
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(label, "admin:cat:"+s.Category),
		))
	}
	rows = append(rows, newInlineKeyboardRow(
		newInlineKeyboardButtonData("🏠 Меню", "menu"),
	))

	for _, s := range settings {
		state := "выключена"
		if s.Enabled {
			state = "включена"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", categoryLabelByKey(s.Category), state))
	}

	text := strings.Join(lines, "\n")
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = newInlineKeyboardMarkup(rows...)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send admin categories: %v", err)
	}
}

func (b *Bot) sendPaymentsStatus(chatID int64) {
	enabled, err := b.userService.IsPaymentsEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось получить состояние платежей")
		return
	}
	state := "❌ Выключены"
	toggleLabel := "Включить платежи"
	if enabled {
		state = "✅ Включены"
		toggleLabel = "Выключить платежи"
	}

	text := fmt.Sprintf("💳 Платежи: %s", state)
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData(toggleLabel, "admin:payments_toggle"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📊 Статистика платежей", "admin:pay_stats"),
			newInlineKeyboardButtonData("📋 Последние платежи", "admin:pay_list"),
		),
		newInlineKeyboardRow(
			newBackButton("Назад", "admin:menu"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send payments status: %v", err)
	}
}

func (b *Bot) sendPaymentStats(chatID int64) {
	now := time.Now().UTC()

	dayStats, errDay := b.userService.GetPaymentStats(now.Add(-24 * time.Hour))
	weekStats, errWeek := b.userService.GetPaymentStats(now.Add(-7 * 24 * time.Hour))
	allStats, errAll := b.userService.GetPaymentStatsAll()

	if errDay != nil || errWeek != nil || errAll != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении статистики платежей")
		return
	}

	text := fmt.Sprintf(`📊 <b>Статистика платежей</b>

📅 <b>За день</b> (24ч):
  Платежей: %d
  Сумма: %.2f ₽

📆 <b>За неделю</b> (7д):
  Платежей: %d
  Сумма: %.2f ₽

♾️ <b>За всё время</b>:
  Платежей: %d
  Сумма: %.2f ₽`,
		dayStats.Count, dayStats.TotalAmount,
		weekStats.Count, weekStats.TotalAmount,
		allStats.Count, allStats.TotalAmount,
	)

	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📋 Последние платежи", "admin:pay_list"),
		),
		newInlineKeyboardRow(
			newBackButton("Назад", "admin:payments"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send payment stats: %v", err)
	}
}

func (b *Bot) sendRecentPayments(chatID int64) {
	payments, err := b.userService.GetRecentPayments(20)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении списка платежей")
		return
	}

	if len(payments) == 0 {
		text := "📋 <b>Платежи</b>\n\nПлатежей пока нет."
		kb := newInlineKeyboardMarkup(
			newInlineKeyboardRow(
				newInlineKeyboardButtonData("◀️ Назад", "admin:payments"),
				newInlineKeyboardButtonData("🏠 Меню", "menu"),
			),
		)
		msg := newMessageConfig(chatID, text)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = kb
		b.sendMsg(msg)
		return
	}

	text := "📋 <b>Последние платежи</b>\n\n"
	for _, p := range payments {
		username := p.Username
		if username == "" {
			username = "—"
		}
		nameParts := strings.TrimSpace(strings.Join([]string{p.FirstName, p.LastName}, " "))
		if nameParts == "" {
			nameParts = "—"
		}
		text += fmt.Sprintf("• <b>%.2f ₽</b> | @%s | ID: %d\n  Имя: %s | %s × %d | %s\n",
			p.Amount,
			html.EscapeString(username),
			p.TelegramID,
			html.EscapeString(nameParts),
			html.EscapeString(p.Category),
			p.Qty,
			p.CreatedAt.In(time.FixedZone("MSK", 3*60*60)).Format("02.01.2006 15:04"),
		)
	}

	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📊 Статистика", "admin:pay_stats"),
		),
		newInlineKeyboardRow(
			newBackButton("Назад", "admin:payments"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send recent payments: %v", err)
	}
}

func (b *Bot) sendSubscriptionsStatus(chatID int64) {
	enabled, err := b.userService.IsSubscriptionsEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось получить состояние подписок")
		return
	}
	state := "❌ Выключены"
	toggleLabel := "Включить подписки"
	if enabled {
		state = "✅ Включены"
		toggleLabel = "Выключить подписки"
	}

	text := fmt.Sprintf("🔒 Подписки: %s", state)
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData(toggleLabel, "admin:subs_toggle"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send subscriptions status: %v", err)
	}
}

func (b *Bot) toggleSubscriptions(chatID int64) error {
	enabled, err := b.userService.IsSubscriptionsEnabled()
	if err != nil {
		return err
	}
	if err := b.userService.SetSubscriptionsEnabled(!enabled); err != nil {
		return err
	}
	b.sendSubscriptionsStatus(chatID)
	return nil
}

func (b *Bot) togglePayments(chatID int64) {
	enabled, err := b.userService.IsPaymentsEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось проверить платежи")
		return
	}
	if err := b.userService.SetPaymentsEnabled(!enabled); err != nil {
		b.sendErrorMessage(chatID, "Не удалось переключить платежи")
		return
	}
	b.sendPaymentsStatus(chatID)
}

// sendAdminMenu выводит инлайн-меню админки
func (b *Bot) sendAdminMenu(chatID int64) {
	text := "👑 Админ-панель\nВыберите действие:"
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📊 Статистика", "admin:stats"),
			newInlineKeyboardButtonData("👥 Пользователи", "admin:users"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📈 Топ за день", "admin:top_users"),
			newInlineKeyboardButtonData("👤 Кол-во пользователей", "admin:users_count"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("⚙️ Категории", "admin:categories"),
			newInlineKeyboardButtonData("💳 Платежи", "admin:payments"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("💰 Баланс API", "admin:suno_balance"),
			newInlineKeyboardButtonData("📸 Фото API", "admin:nano_api"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("🔒 Подписки", "admin:subs"),
			newInlineKeyboardButtonData("ℹ️ Справка", "admin:help"),
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send admin menu: %v", err)
	}
}

func (b *Bot) toggleCategory(chatID int64, category string) {
	enabled, err := b.userService.IsCategoryEnabled(category)
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось проверить категорию")
		return
	}
	if err := b.userService.SetCategoryEnabled(category, !enabled); err != nil {
		b.sendErrorMessage(chatID, "Не удалось изменить категорию")
		return
	}
	b.sendAdminCategories(chatID)
}

func categoryLabelByKey(key string) string {
	switch key {
	case "photo":
		return "Фото"
	case "video":
		return "Видео"
	case "music":
		return "Музыка"
	case "chat":
		return "Текст"
	default:
		return key
	}
}

const requiredChannelUsername = "@AIFaceApps"
const requiredChannelLink = "https://t.me/AIFaceApps"

const modelsDescription = `Категории моделей:
📸 Фото — редактирование и генерация изображений.
🎵 Песни — генерация треков.
💬 Текст — ответы на вопросы и сопровождение.`

type chatStyle struct {
	ID string
}

var chatStyles = []chatStyle{
	{ID: "normal"},
	{ID: "formal"},
	{ID: "humor"},
	{ID: "informal"},
	{ID: "friendly"},
	{ID: "expert"},
	{ID: "empathetic"},
}

type ModelCategory string

const (
	ModelCategoryPhoto ModelCategory = "photo"
	ModelCategoryMusic ModelCategory = "music"
	ModelCategoryVideo ModelCategory = "video"
	ModelCategoryChat  ModelCategory = "chat"
)

type ModelOption struct {
	ID          string
	ApiModel    string
	Label       string
	Desc        string
	Category    ModelCategory
	RequestCost int
	TaskType    string
	AdminOnly   bool
	EmojiID     string // Custom emoji ID for button icon
}

// isUserAllowed проверяет доступ по белому списку (ADMIN_TELEGRAM_IDS)
func (b *Bot) isUserAllowed(userID int64) bool {
	isAdmin, err := b.userService.IsUserAdmin(userID)
	if err != nil {
		return false
	}
	return isAdmin
}

// Доступные модели для выбора
var modelOptions = []ModelOption{
	{ID: "google/nano-banana", Label: "Nano Banana", Desc: locRU.ModelNanoBanana, Category: ModelCategoryPhoto, RequestCost: 1, EmojiID: "5188481279963715781"},
	{ID: "google/nano-banana-pro", Label: "Nano Banana Pro", Desc: locRU.ModelNanoBananaPro, Category: ModelCategoryPhoto, RequestCost: 4, EmojiID: "5463289097336405244"},
	{ID: "nano-banana-2", Label: "Nano Banana 2", Desc: locRU.ModelNanoBanana2, Category: ModelCategoryPhoto, RequestCost: 2, EmojiID: "5258203794772085854"},
	{ID: "seedream/4.5-edit", Label: "👙 Seedream 4.5", Desc: locRU.ModelSeedream, Category: ModelCategoryPhoto, RequestCost: 3},
	{ID: "veo3_fast", ApiModel: "veo3_fast", Label: "🎬 Veo 3.1 Fast", Desc: locRU.ModelVeo3Fast, Category: ModelCategoryVideo, RequestCost: 1},
	{ID: "wan/2-6-image-to-video", ApiModel: "wan/2-6-image-to-video", Label: "🎥 Wan 2.6", Desc: locRU.ModelWan26, Category: ModelCategoryVideo, RequestCost: 2},
	{ID: "kling-2.6/image-to-video", ApiModel: "kling-2.6/image-to-video", Label: "🎬 Kling 2.6", Desc: locRU.ModelKling26, Category: ModelCategoryVideo, RequestCost: 1},
	{ID: "music-suno", ApiModel: "suno", Label: "Suno Music", Desc: locRU.ModelSunoMusic, Category: ModelCategoryMusic, RequestCost: 1, TaskType: "music", EmojiID: "5217933090483098080"},
	{ID: "google/gemini-3-flash", Label: "Gemini 3 Flash", Desc: "", Category: ModelCategoryChat, RequestCost: 1, EmojiID: "5443038326535759644"},
	{ID: "openai/gpt-5-mini", Label: "GPT-5 mini", Desc: locRU.ModelGPT5Mini, Category: ModelCategoryChat, RequestCost: 1, EmojiID: "5443038326535759644"},
	{ID: "openai/gpt-5-nano", Label: "GPT-5 nano", Desc: locRU.ModelGPT5Nano, Category: ModelCategoryChat, RequestCost: 1, EmojiID: "5443038326535759644"},
	{ID: "chat-gpt-4.1mini", ApiModel: "gpt-4.1-mini", Label: "GPT-4.1 mini", Desc: locRU.ModelGPT41Mini, Category: ModelCategoryChat, RequestCost: 1, TaskType: "chat", EmojiID: "5443038326535759644"},
}

var modelCategories = []ModelCategory{ModelCategoryPhoto, ModelCategoryVideo, ModelCategoryMusic, ModelCategoryChat}
var adminModelCategories = []ModelCategory{ModelCategoryPhoto, ModelCategoryVideo, ModelCategoryMusic, ModelCategoryChat}

func NewBot(token string, userService *services.UserService, generationService *services.GenerationService, paymentService *services.PaymentService, redisClient *redis.Client, cfg *config.Config) (*Bot, error) {
	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		ctx:               ctx,
		cancel:            cancel,
		userService:       userService,
		generationService: generationService,
		paymentService:    paymentService,
		redisClient:       redisClient,
		cfg:               cfg,
		concurrencySem:    make(chan struct{}, 10),
		albumBuffers:      make(map[string]*albumBuffer),
		recentPhotos:      make(map[int64][]photoRecord),
		sunoTasks:         make(map[string]sunoTask),
		sunoInstrumental:  make(map[int64]bool),
		sunoVoice:         make(map[int64]string),
	}

	opts := []tgbot.Option{
		tgbot.WithDefaultHandler(func(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
			bot.goLimited(func() {
				bot.safeHandleUpdate(update)
			})
		}),
	}

	api, err := tgbot.New(token, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}
	bot.api = api

	// Get bot user info
	botUser, err := api.GetMe(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}
	bot.botUser = botUser

	// Уведомление после успешного платежа
	paymentService.SetNotifier(bot.notifyPaymentSuccess)

	return bot, nil
}

func (b *Bot) Start() error {
	log.Printf("Authorized on account %s", b.botUser.Username)

	// Delete webhook if any
	_, err := b.api.DeleteWebhook(b.ctx, &tgbot.DeleteWebhookParams{DropPendingUpdates: false})
	if err != nil {
		log.Printf("failed to delete webhook: %v", err)
	}

	// Устанавливаем команды меню
	b.setCommands()

	// Запускаем планировщик напоминаний о пробной генерации
	go b.startTrialReminderScheduler()

	// Start polling via the new library
	b.api.Start(b.ctx)

	return nil
}

func (b *Bot) Stop() {
	b.shutdownOnce.Do(func() {
		b.shuttingDown.Store(true)
		if b.cancel != nil {
			b.cancel()
		}
	})
	b.wg.Wait()
}

func (b *Bot) PendingSunoTasks() int {
	b.sunoMu.Lock()
	defer b.sunoMu.Unlock()
	return len(b.sunoTasks)
}

func (b *Bot) WaitSunoTasks(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if b.PendingSunoTasks() == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// safeHandleUpdate оборачивает обработку апдейта с восстановлением после паники
func (b *Bot) safeHandleUpdate(update *tgmodels.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in update handler: %v", r)
		}
	}()

	if update.Message != nil {
		if b.shuttingDown.Load() {
			b.sendText(update.Message.Chat.ID, "В данный момент бот перезагружается. Попробуйте ещё через пару минут.")
			return
		}
		b.handleMessage(update.Message)
		return
	}
	if update.CallbackQuery != nil {
		if b.shuttingDown.Load() {
			// убираем крутилку у пользователя
			b.api.AnswerCallbackQuery(b.ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
			})
			if update.CallbackQuery.Message.Message != nil {
				b.sendText(update.CallbackQuery.Message.Message.Chat.ID, "В данный момент бот перезагружается. Попробуйте ещё через пару минут.")
			}
			return
		}
		b.handleCallback(update.CallbackQuery)
		return
	}
}

// goLimited запускает функцию в горутине с ограничением параллелизма
func (b *Bot) goLimited(fn func()) {
	b.wg.Add(1)
	deferFunc := func() {
		b.wg.Done()
	}
	select {
	case b.concurrencySem <- struct{}{}:
		go func() {
			defer func() {
				<-b.concurrencySem
				deferFunc()
				if r := recover(); r != nil {
					log.Printf("panic in goroutine: %v", r)
				}
			}()
			fn()
		}()
	default:
		// Если лимит исчерпан, обработаем синхронно, но с защитой
		defer deferFunc()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in goroutine (sync fallback): %v", r)
			}
		}()
		fn()
	}
}

// setCommands устанавливает команды меню бота
func (b *Bot) setCommands() {
	commands := []tgmodels.BotCommand{
		{Command: "start", Description: "Начать"},
		{Command: "menu", Description: "Главное меню"},
		{Command: "account", Description: "Аккаунт и лимиты"},
		{Command: "buy", Description: "Купить (подписку,генерации)"},
		{Command: "invite", Description: "Пригласить друзей"},
		{Command: "rules", Description: "Правила использования"},
		{Command: "help", Description: "Помощь"},
		{Command: "privacy", Description: "Политика и Польз. соглашение"},
		{Command: "settings", Description: "Настройки стиля общения"},
	}

	_, err := b.api.SetMyCommands(b.ctx, &tgbot.SetMyCommandsParams{
		Commands: commands,
	})
	if err != nil {
		log.Printf("Failed to set commands: %v", err)
	}
}

// setChatCommands устанавливает команды для конкретного чата (например, добавить /admin только админам)
func (b *Bot) setChatCommands(chatID int64, isAdmin bool) {
	commands := []tgmodels.BotCommand{
		{Command: "start", Description: "Начать"},
		{Command: "menu", Description: "Главное меню"},
		{Command: "account", Description: "Аккаунт и лимиты"},
		{Command: "buy", Description: "Купить (подписку,генерации)"},
		{Command: "invite", Description: "Пригласить друзей"},
		{Command: "rules", Description: "Правила использования"},
		{Command: "help", Description: "Помощь"},
		{Command: "privacy", Description: "Политика и Польз. соглашение"},
		{Command: "settings", Description: "Настройки стиля общения"},
	}
	if isAdmin {
		commands = append(commands, tgmodels.BotCommand{Command: "admin", Description: "Админ-панель"})
	}
	_, err := b.api.SetMyCommands(b.ctx, &tgbot.SetMyCommandsParams{
		Commands: commands,
		Scope:    &tgmodels.BotCommandScopeChat{ChatID: chatID},
	})
	if err != nil {
		log.Printf("Failed to set chat commands: %v", err)
	}
}

func (b *Bot) handleMessage(msg *tgmodels.Message) {
	user := msg.From
	if user == nil {
		return
	}

	// Если /start с реферальным кодом — обрабатываем раньше, чтобы не создать пользователя без referrer
	if isCommand(msg) {
		cmd := getCommand(msg)
		args := getCommandArgs(msg)
		if cmd == "start" && args != "" {
			// Сначала создаем/обновляем пользователя с referrer_id, а потом проверяем подписку
			if _, err := b.userService.GetOrCreateUserWithReferrer(
				user.ID,
				user.Username,
				user.FirstName,
				user.LastName,
				user.LanguageCode,
				args,
			); err != nil {
				log.Printf("Failed to create user with referral: %v", err)
			}
			b.handleStartWithReferral(msg, args)
			return
		}
	}

	// Получаем или создаем пользователя
	_, err := b.userService.GetOrCreateUser(
		user.ID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
	)
	if err != nil {
		log.Printf("Failed to get/create user %d: %v", user.ID, err)
		return
	}

	// Не режем альбомы кулдауном, чтобы не терять кадры одной медиагруппы
	if msg.MediaGroupID == "" || msg.Photo == nil {
		if !b.checkCooldown(msg.Chat.ID, user.ID) {
			return
		}
	}

	// Middleware: проверка и выдача пробного запроса при подписке на канал
	b.checkAndGrantChannelTrial(user.ID)

	// Обрабатываем команды
	if isCommand(msg) {
		b.handleCommand(msg)
		return
	}

	// Обрабатываем фото
	if msg.Photo != nil {
		b.handlePhoto(msg)
		return
	}

	// Обрабатываем документы как изображения (если это картинка)
	if msg.Document != nil {
		b.handleDocument(msg)
		return
	}

	// Обрабатываем текстовые сообщения
	b.handleTextMessage(msg)
}

func (b *Bot) handleCommand(msg *tgmodels.Message) {
	cmd := getCommand(msg)
	args := getCommandArgs(msg)

	switch cmd {
	case "start":
		// Проверяем реферальный код в аргументах
		if args != "" {
			b.handleStartWithReferral(msg, args)
		} else {
			b.handleStart(msg)
		}
	case "menu":
		b.sendMainMenu(msg.Chat.ID, msg.From.ID)
	case "account":
		b.sendAccount(msg.Chat.ID, msg.From.ID)
	case "buy":
		if !b.ensurePaymentsEnabled(msg.Chat.ID) {
			return
		}
		b.sendBuyMenu(msg.Chat.ID, msg.From.ID)
	case "invite":
		b.sendInviteInfo(msg.Chat.ID, msg.From.ID)
	case "help":
		b.sendHelpMessage(msg.Chat.ID)
	case "emoji":
		b.handleEmojiCommand(msg)
	case "rules":
		b.sendRulesMessage(msg.Chat.ID, msg.From.ID)
	case "privacy":
		b.sendPrivacyMessage(msg.Chat.ID, msg.From.ID)
	case "settings":
		b.sendSettingsMenu(msg.Chat.ID, msg.From.ID)
	case "admin":
		b.handleAdminCommand(msg)
	default:
		b.sendUnknownCommand(msg.Chat.ID)
	}
}

func (b *Bot) handlePhoto(msg *tgmodels.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	loc := b.getLocalization(userID)

	caption := strings.TrimSpace(msg.Caption)
	if strings.HasPrefix(caption, "/admin") {
		b.handleAdminCommand(msg)
		return
	}

	// Альбом до 4 фото для Nano Banana / Nano Banana Pro
	if msg.MediaGroupID != "" {
		b.handleAlbumPhoto(msg)
		return
	}

	// Текущая модель
	modelID := b.getUserModel(userID)
	modelOpt, ok := findModelOption(modelID)
	if !ok {
		modelOpt = ModelOption{ID: modelID, Category: ModelCategoryPhoto, RequestCost: 1}
	}

	// Проверяем, есть ли подпись к фото (caption) — только для фото-моделей и видео-моделей
	if caption == "" && (modelOpt.Category == ModelCategoryPhoto || modelOpt.Category == ModelCategoryVideo) {
		text := loc.PhotoReceived + "\n\n" + loc.PhotoAddCaption
		reply := newMessageConfig(chatID, text)
		_, _ = b.sendMsg(reply)
		return
	}

	// Получаем самое большое фото
	photo := msg.Photo[len(msg.Photo)-1]

	// Получаем информацию о файле
	file, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: photo.FileID})
	if err != nil {
		log.Printf("Failed to get file info: %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить изображение")
		return
	}

	fileURL := b.getFileURL(file)

	// Если выбрана видео-модель — запускаем видео-генерацию с промптом
	if modelOpt.Category == ModelCategoryVideo {
		b.processVideoGeneration(chatID, userID, fileURL, caption, modelOpt)
		return
	}

	// Определяем тип генерации по описанию
	genType := b.detectGenerationType(caption)

	// Запускаем генерацию
	b.processGeneration(chatID, userID, []string{fileURL}, genType, caption)
}

// handleDocument обрабатывает документы как изображения (если это картинка)
func (b *Bot) handleDocument(msg *tgmodels.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	caption := strings.TrimSpace(msg.Caption)
	if strings.HasPrefix(caption, "/admin") {
		b.handleAdminCommand(msg)
		return
	}

	// Поддерживаем только изображения
	if msg.Document == nil || msg.Document.MimeType == "" || !strings.HasPrefix(msg.Document.MimeType, "image/") {
		b.sendErrorMessage(chatID, "Отправьте изображение или фото")
		return
	}

	// Альбом документов (медиагруппа)
	if msg.MediaGroupID != "" {
		b.handleAlbumPhoto(msg)
		return
	}

	// Текущая модель
	modelID := b.getUserModel(userID)
	modelOpt, ok := findModelOption(modelID)
	if !ok {
		modelOpt = ModelOption{ID: modelID, Category: ModelCategoryPhoto, RequestCost: 1}
	}

	// Подпись
	if caption == "" && (modelOpt.Category == ModelCategoryPhoto || modelOpt.Category == ModelCategoryVideo) {
		text := `📷 Фото получено!

Пожалуйста, отправьте фото ещё раз, но в подписи укажите промпт.`
		reply := newMessageConfig(chatID, text)
		_, _ = b.sendMsg(reply)
		return
	}

	// Получаем информацию о файле
	file, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: msg.Document.FileID})
	if err != nil {
		log.Printf("Failed to get file info (doc): %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить изображение")
		return
	}
	fileURL := b.getFileURL(file)

	// Если выбрана видео-модель — запускаем видео-генерацию с промптом
	if modelOpt.Category == ModelCategoryVideo {
		b.processVideoGeneration(chatID, userID, fileURL, caption, modelOpt)
		return
	}

	// Определяем тип генерации по описанию
	genType := b.detectGenerationType(caption)

	// Запускаем генерацию
	b.processGeneration(chatID, userID, []string{fileURL}, genType, caption)
}

// handleAlbumPhoto агрегирует альбом до 4 фото и запускает генерацию одним запросом
func (b *Bot) handleAlbumPhoto(msg *tgmodels.Message) {
	mediaID := msg.MediaGroupID
	if mediaID == "" {
		return
	}

	// Сохраняем самый большой размер фото
	var fileID, uniqueID string
	var fileSize int
	if msg.Photo != nil {
		photo := msg.Photo[len(msg.Photo)-1]
		fileID = photo.FileID
		uniqueID = photo.FileUniqueID
		fileSize = photo.FileSize
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		uniqueID = msg.Document.FileUniqueID
		fileSize = int(msg.Document.FileSize)
	} else {
		return
	}
	log.Printf("handleAlbumPhoto: mediaGroup=%s msgID=%d fileID=%s uniqueID=%s", mediaID, msg.ID, fileID, uniqueID)

	b.albumMu.Lock()
	if b.albumBuffers == nil {
		b.albumBuffers = make(map[string]*albumBuffer)
	}
	buf, ok := b.albumBuffers[mediaID]
	if !ok {
		buf = &albumBuffer{
			chatID:  msg.Chat.ID,
			userID:  msg.From.ID,
			created: time.Now(),
		}
		b.albumBuffers[mediaID] = buf
	}
	buf.photos = append(buf.photos, albumPhoto{
		msgID: msg.ID,
		photo: tgmodels.PhotoSize{
			FileID:       fileID,
			FileUniqueID: uniqueID,
			FileSize:     fileSize,
		},
	})
	if msg.Caption != "" {
		buf.caption = strings.TrimSpace(msg.Caption)
	}
	if len(buf.photos) > 4 {
		buf.photos = buf.photos[:4] // жёсткий лимит по ТЗ
	}

	// Перезапускаем таймер, чтобы дождаться остальных частей альбома
	if buf.timer != nil {
		buf.timer.Stop()
	}
	buf.timer = time.AfterFunc(2*time.Second, func() {
		b.flushAlbum(mediaID)
	})
	b.albumMu.Unlock()
}

func (b *Bot) flushAlbum(mediaGroupID string) {
	b.albumMu.Lock()
	buf, ok := b.albumBuffers[mediaGroupID]
	if ok {
		delete(b.albumBuffers, mediaGroupID)
	}
	b.albumMu.Unlock()

	if !ok || buf == nil || len(buf.photos) == 0 {
		return
	}

	// Определяем модель
	modelID := b.getUserModel(buf.userID)
	modelOpt, ok := findModelOption(modelID)
	if !ok {
		modelOpt = ModelOption{ID: modelID, Category: ModelCategoryPhoto, RequestCost: 1}
	}

	// Альбом поддерживаем для фото-моделей и wan/2-6-image-to-video
	if modelOpt.Category != ModelCategoryPhoto && modelOpt.ID != "wan/2-6-image-to-video" {
		b.sendErrorMessage(buf.chatID, "Для альбомов выберите фото-модель или Wan 2.6 в /menu")
		return
	}

	// Сортируем по msgID, чтобы сохранить порядок кадров из альбома
	sort.Slice(buf.photos, func(i, j int) bool {
		return buf.photos[i].msgID < buf.photos[j].msgID
	})

	// Получаем URL для каждого фото
	var imageURLs []string
	var fileIDs []string
	var fileUniqueIDs []string
	for _, p := range buf.photos {
		file, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: p.photo.FileID})
		if err != nil {
			log.Printf("Failed to get file info from album: %v", err)
			continue
		}
		fileIDs = append(fileIDs, p.photo.FileID)
		fileUniqueIDs = append(fileUniqueIDs, p.photo.FileUniqueID)
		url := b.getFileURL(file)
		imageURLs = append(imageURLs, url)
		b.rememberPhoto(buf.userID, url, p.photo.FileUniqueID)
	}
	log.Printf("album collected: mediaGroup=%s photos=%d urls=%d fileIDs=%v uniqueIDs=%v urlsList=%v", mediaGroupID, len(buf.photos), len(imageURLs), fileIDs, fileUniqueIDs, imageURLs)

	if len(imageURLs) == 0 {
		b.sendErrorMessage(buf.chatID, "Не удалось получить изображения из альбома")
		return
	}

	// Ограничения по количеству фото в зависимости от модели
	maxPhotos := 1
	if modelOpt.ID == "google/nano-banana" {
		maxPhotos = 2
	} else if modelOpt.ID == "google/nano-banana-pro" || modelOpt.ID == "nano-banana-2" || modelOpt.ID == "seedream/4.5-edit" {
		maxPhotos = 4
	} else if modelOpt.ID == "wan/2-6-image-to-video" {
		maxPhotos = 4 // wan 2.6 supports up to 4 photos
	}
	if len(imageURLs) > maxPhotos {
		imageURLs = imageURLs[:maxPhotos]
	}

	caption := strings.TrimSpace(buf.caption)
	if caption == "" {
		loc := b.getLocalization(buf.userID)
		text := loc.PhotoReceived + "\n\n" + loc.PhotoAddCaption
		reply := newMessageConfig(buf.chatID, text)
		_, _ = b.sendMsg(reply)
		return
	}

	// Если это wan/2-6-image-to-video, запускаем видео-генерацию с несколькими фото
	if modelOpt.ID == "wan/2-6-image-to-video" {
		b.processVideoGenerationMultiPhoto(buf.chatID, buf.userID, imageURLs, caption, modelOpt)
		return
	}

	// Тип генерации по описанию
	genType := b.detectGenerationType(caption)

	// Запускаем генерацию с несколькими фото
	b.processGeneration(buf.chatID, buf.userID, imageURLs, genType, caption)
}

func (b *Bot) rememberPhoto(userID int64, url string, uniqueID string) {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recentPhotos[userID]
	list = append(list, photoRecord{URL: url, UniqueID: uniqueID, Time: time.Now()})
	if len(list) > 10 {
		list = list[len(list)-10:]
	}

	b.recentPhotos[userID] = list
}

// pickRecentDistinctPhotos возвращает до n последних разных фото (uniqueID) кроме excludeUniqueID
func (b *Bot) pickRecentDistinctPhotos(userID int64, excludeUniqueID string, n int) []string {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recentPhotos[userID]
	var res []string
	seen := map[string]struct{}{excludeUniqueID: {}}
	for i := len(list) - 1; i >= 0 && len(res) < n; i-- {
		rec := list[i]
		if _, ok := seen[rec.UniqueID]; ok {
			continue
		}
		seen[rec.UniqueID] = struct{}{}
		res = append(res, rec.URL)
	}
	return res
}

// detectGenerationType определяет тип генерации по тексту описания
func (b *Bot) detectGenerationType(description string) string {
	descLower := strings.ToLower(description)

	// Ключевые слова для причёски
	hairKeywords := []string{"прическ", "волос", "стрижк", "укладк", "чёлк", "челк", "блонд", "брюнет", "рыж", "каре", "локон", "кудр"}
	for _, kw := range hairKeywords {
		if strings.Contains(descLower, kw) {
			return "hair_change"
		}
	}

	// Ключевые слова для одежды
	clothingKeywords := []string{"одежд", "платье", "костюм", "примерить", "надеть", "футболк", "рубашк", "пиджак", "куртк", "пальто", "джинс", "брюк", "юбк"}
	for _, kw := range clothingKeywords {
		if strings.Contains(descLower, kw) {
			return "clothing_tryon"
		}
	}

	// По умолчанию - изменение прически
	return "hair_change"
}

func (b *Bot) getUserModel(userID int64) string {
	model, err := b.redisClient.GetUserModel(userID)
	if err != nil {
		log.Printf("getUserModel redis error: %v", err)
	}
	if opt, ok := findModelOption(model); ok {
		return opt.ID
	}

	// Пытаемся использовать модель из конфига
	if opt, ok := findModelOption(b.cfg.OpenRouter.Model); ok {
		return opt.ID
	}

	// Фолбек — первая фото-модель
	for _, m := range modelOptions {
		if m.Category == ModelCategoryPhoto {
			return m.ID
		}
	}

	// Если почему-то нет ни одной фото-модели — возвращаем первую доступную
	if len(modelOptions) > 0 {
		return modelOptions[0].ID
	}

	return ""
}

func (b *Bot) setUserModel(userID int64, model string) {
	if model == "" {
		return
	}
	if _, ok := findModelOption(model); !ok {
		log.Printf("setUserModel: unknown model %s", model)
		return
	}
	if err := b.redisClient.SetUserModel(userID, model); err != nil {
		log.Printf("setUserModel error: %v", err)
	}
}

func modelLabel(id string) string {
	if m, ok := findModelOption(id); ok {
		return m.Label
	}
	return id
}

func modelDescription(id string) string {
	if m, ok := findModelOption(id); ok {
		return m.Desc
	}
	return ""
}

func modelRequestCost(id string) int {
	if m, ok := findModelOption(id); ok && m.RequestCost > 0 {
		return m.RequestCost
	}
	return 1
}

func (b *Bot) ensureCategoryEnabled(chatID int64, cat ModelCategory) bool {
	key := categoryKey(cat)
	enabled, err := b.userService.IsCategoryEnabled(key)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка проверки категории")
		return false
	}
	if !enabled {
		b.sendErrorMessage(chatID, fmt.Sprintf("Категория %s временно недоступна", categoryLabel(cat)))
		return false
	}
	return true
}

func (b *Bot) ensurePaymentsEnabled(chatID int64) bool {
	enabled, err := b.userService.IsPaymentsEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка проверки платежей")
		return false
	}
	if !enabled {
		b.sendErrorMessage(chatID, "Платежи временно отключены администратором.")
		return false
	}
	return true
}

func (b *Bot) ensureAdmin(chatID int64, userID int64) bool {
	isAdmin, err := b.userService.IsUserAdmin(userID)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при проверке прав администратора")
		return false
	}
	if !isAdmin {
		b.sendErrorMessage(chatID, "У вас нет прав администратора")
		return false
	}
	return true
}

func categoryKey(cat ModelCategory) string {
	switch cat {
	case ModelCategoryPhoto:
		return "photo"
	case ModelCategoryVideo:
		return "video"
	case ModelCategoryMusic:
		return "music"
	case ModelCategoryChat:
		return "chat"
	default:
		return "photo"
	}
}

func containsCategory(list []ModelCategory, c ModelCategory) bool {
	for _, v := range list {
		if v == c {
			return true
		}
	}
	return false
}

func (b *Bot) getEnabledCategories(includeDisabledAsAdmin bool) []ModelCategory {
	var enabled []ModelCategory
	for _, cat := range modelCategories {
		if includeDisabledAsAdmin {
			enabled = append(enabled, cat)
			continue
		}
		enabledFlag, err := b.userService.IsCategoryEnabled(categoryKey(cat))
		if err != nil {
			continue
		}
		if enabledFlag {
			enabled = append(enabled, cat)
		}
	}
	return enabled
}

func quotaCategoryForModel(cat ModelCategory) models.QuotaCategory {
	switch cat {
	case ModelCategoryPhoto:
		return models.QuotaCategoryImage
	case ModelCategoryVideo:
		return models.QuotaCategoryVideo
	case ModelCategoryMusic:
		return models.QuotaCategoryMusic
	case ModelCategoryChat:
		return models.QuotaCategoryText
	default:
		return models.QuotaCategoryImage
	}
}

func findModelOption(id string) (ModelOption, bool) {
	if id == "" {
		return ModelOption{}, false
	}
	// Алиасы
	if strings.EqualFold(id, "gemini") || strings.EqualFold(id, "gemini-2.5-flash-image") || strings.EqualFold(id, "nano-banana") {
		id = "google/nano-banana"
	}
	if strings.EqualFold(id, "kie/nano-banana-edit") {
		id = "google/nano-banana"
	}
	if strings.EqualFold(id, "kie/nano-banana-pro") {
		id = "google/nano-banana-pro"
	}
	for _, m := range modelOptions {
		if strings.EqualFold(m.ID, id) {
			return m, true
		}
	}
	return ModelOption{}, false
}

func modelOptionsByCategory(category ModelCategory) []ModelOption {
	var result []ModelOption
	for _, m := range modelOptions {
		if m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

func (b *Bot) isModelVisibleToUser(userID int64, m ModelOption) bool {
	if m.AdminOnly {
		return b.isUserAllowed(userID)
	}
	return true
}

func categoryLabel(cat ModelCategory) string {
	switch cat {
	case ModelCategoryPhoto:
		return "📸 Фото"
	case ModelCategoryVideo:
		return "🎬 Видео"
	case ModelCategoryMusic:
		return "🎵 Музыка"
	case ModelCategoryChat:
		return "💬 Текст"
	default:
		return string(cat)
	}
}

func categoryUnitLoc(cat ModelCategory, loc *Localization) string {
	switch cat {
	case ModelCategoryMusic:
		return loc.TrackUnit
	case ModelCategoryVideo:
		return loc.VideoUnit
	case ModelCategoryChat:
		return loc.QueryUnit
	default:
		return loc.PhotoUnit
	}
}

func categoryUnit(cat ModelCategory) string {
	switch cat {
	case ModelCategoryMusic:
		return "трек"
	case ModelCategoryVideo:
		return "видео"
	case ModelCategoryChat:
		return "запрос"
	default:
		return "фото"
	}
}

// requestWord возвращает правильную форму слова "запрос" / "request" по количеству
func requestWord(count int, loc *Localization) string {
	if count < 0 {
		count = -count
	}

	// Английский
	if loc == &locEN {
		if count == 1 {
			return "request"
		}
		return "requests"
	}

	// Русский (по умолчанию)
	mod10 := count % 10
	mod100 := count % 100
	if mod10 == 1 && mod100 != 11 {
		return "запрос"
	}
	if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
		return "запроса"
	}
	return "запросов"
}

// instructionForModel возвращает инструкцию, завязанную на конкретную модель
func instructionForModel(m ModelOption) string {
	cost := m.RequestCost
	if cost < 1 {
		cost = 1
	}
	base := fmt.Sprintf("%s Модель: %s\nОписание: %s\nТребуется: %d запрос(ов) за 1 %s.\n\n",
		categoryLabel(m.Category), modelLabel(m.ID), modelDescription(m.ID), cost, categoryUnit(m.Category))

	switch m.ID {
	case "google/nano-banana":
		return base + `Принимает фото с подписью и возвращает изображение среднего качества.

Можно отправить до 2 фото за запрос.

Как отправить:
1) Пришлите фото.
2) В подписи опишите, что нужно сделать (ретушь, правки, изменение).

Используйте /menu для выбора другой модели.`
	case "google/nano-banana-pro":
		return base + `Принимает фото с подписью и возвращает улучшенное изображение (лучшее качество, но дольше).

Можно отправить до 4 фото за запрос.

Как отправить:
1) Пришлите фото.
2) В подписи опишите, что нужно изменить или улучшить.

Используйте /menu для выбора другой модели.`
	case "nano-banana-2":
		return base + `Новейшая модель. Поддерживает текст без фото, альбомы до 4 фото, разрешение 1K/2K/4K и Google Search.

Как отправить:
1) Отправьте текстовый промпт (фото необязательно).
2) Или пришлите до 4 фото с подписью.

Используйте /menu для выбора другой модели.`
	case "veo3_fast":
		return base + `Принимает 1 фото с подписью и генерирует видео.

Как отправить:
1) Пришлите 1 фото.
2) В подписи опишите, какое видео хотите получить.
3) Дождитесь готового видео (2-5 минут).

Используйте /menu для выбора другой модели.`
	case "music-suno":
		return base + `Принимает текст и генерирует аудио-трек.

Как отправить:
1) Напишите текст запроса или идеи трека.
2) Отправьте в чат — получите ссылку/файл с аудио.

Используйте /menu для выбора другой модели.`
	case "chat-gpt4mini":
		return base + `Быстрые текстовые ответы (GPT-4.1 mini).

Как отправить:
1) Напишите свой вопрос или задачу.
2) Отправьте в чат и получите ответ.

Системный промпт:
` + locRU.ChatSystemPrompt + `

Используйте /menu для выбора другой модели.`
	default:
		// Fallback по категории
		switch m.Category {
		case ModelCategoryPhoto:
			return base + `Эта модель принимает фото с подписью и возвращает обработанное фото.

Как отправить:
1) Пришлите фото.
2) В подписи опишите, что нужно изменить.

Используйте /menu для выбора другой модели.`
		case ModelCategoryVideo:
			return base + `Эта модель принимает 1 фото с подписью и вернёт короткое видео.

Как отправить:
1) Пришлите 1 фото.
2) В подписи опишите, какое видео хотите получить.
3) Дождитесь готового видео.

Используйте /menu для выбора другой модели.`
		case ModelCategoryMusic:
			return base + `Эта модель принимает текст и генерирует аудио-трек.

Как отправить:
1) Напишите текст запроса.
2) Отправьте в чат — получите аудио.

Используйте /menu для выбора другой модели.`
		default:
			return base + "Используйте /menu для выбора другой модели."
		}
	}
}

// checkCooldown применяет общий кулдаун на любые запросы к боту
func (b *Bot) checkCooldown(chatID, userID int64) bool {
	// Без кулдауна для активной подписки или премиум-флага
	if label, ok := b.userService.GetSubscriptionLabel(userID); ok && label != "Free" {
		return true
	}
	if user, err := b.userService.GetUserByTelegramID(userID); err == nil && user != nil && user.IsPremium {
		return true
	}
	// Админы — без кулдауна
	if isAdmin, err := b.userService.IsUserAdmin(userID); err == nil && isAdmin {
		return true
	}

	ok, ttlLeft, err := b.redisClient.TryAcquireCooldown(userID, 2*time.Second)
	if err != nil {
		log.Printf("cooldown check error: %v", err)
		return true // не блокируем при ошибке
	}
	if ok {
		return true
	}

	seconds := int(ttlLeft.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	text := fmt.Sprintf("⏳ Подождите ещё %d секунд перед следующим запросом. Или купите любую подписку.", seconds)
	b.sendText(chatID, text)
	return false
}

// decodeDataURLGeneric декодирует data: URI и возвращает байты и имя файла
func decodeDataURLGeneric(dataURL string, defaultExt string) ([]byte, string, string, error) {
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return nil, "", "", fmt.Errorf("invalid data url")
	}
	meta := dataURL[len("data:"):comma]
	dataPart := dataURL[comma+1:]

	mimeType := "application/octet-stream"
	for _, part := range strings.Split(meta, ";") {
		if strings.Contains(part, "/") {
			mimeType = part
			break
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(dataPart)
	if err != nil {
		return nil, "", mimeType, fmt.Errorf("failed to decode base64: %w", err)
	}

	ext := defaultExt
	if strings.Contains(mimeType, "png") {
		ext = "png"
	} else if strings.Contains(mimeType, "webp") {
		ext = "webp"
	} else if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		ext = "jpg"
	} else if strings.Contains(mimeType, "gif") {
		ext = "gif"
	} else if strings.Contains(mimeType, "mp3") || strings.Contains(mimeType, "mpeg") {
		ext = "mp3"
	} else if strings.Contains(mimeType, "wav") {
		ext = "wav"
	} else if strings.Contains(mimeType, "ogg") {
		ext = "ogg"
	}

	return decoded, "result." + ext, mimeType, nil
}

// processAudioMessage обрабатывает текст как запрос на озвучку
func (b *Bot) processAudioMessage(msg *tgmodels.Message, modelOpt ModelOption) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		b.sendErrorMessage(chatID, "Пустой запрос для озвучки")
		return
	}

	userRec, _ := b.userService.GetUserByTelegramID(userID)
	username := ""
	if userRec != nil {
		username = userRec.Username
	}

	if !b.ensureCategoryEnabled(chatID, ModelCategoryMusic) {
		return
	}

	if modelOpt.Category != ModelCategoryMusic {
		b.sendErrorMessage(chatID, "Выберите музыкальную модель в меню моделей, чтобы сгенерировать песню.")
		return
	}

	requestCost := modelOpt.RequestCost
	if requestCost < 1 {
		requestCost = 1
	}

	instrumental := b.getSunoInstrumental(userID)
	voice := b.getSunoVoice(userID)

	primaryUsed, extraUsed, err := b.userService.ConsumeQuotaDetailed(userID, models.QuotaCategoryMusic, requestCost)
	if err != nil {
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryMusic, requestCost, err)
		return
	}

	modeLine := "Режим: с голосом"
	if instrumental {
		modeLine = "Режим: инструментал (без голоса)"
	}
	voiceLine := "Голос: мужской"
	if voice == "f" {
		voiceLine = "Голос: женский"
	}
	startText := fmt.Sprintf("🔄 Озвучка запущена\nМодель: %s\nСписано: %d музыкальных запрос(ов)\n%s\n%s\nОжидание до 15 минут\n\nТекст:\n%s",
		modelOpt.Label, requestCost, modeLine, voiceLine, truncate(text, 700))
	b.sendText(chatID, truncate(startText, 1000))

	// Асинхронно выполняем озвучку, чтобы не блокировать обработчик
	b.goLimited(func() {
		// Внутренний серверный кулдаун на музыкальные запросы к PiAPI (20 секунд), без уведомлений пользователя
		if ok, ttlLeft, err := b.redisClient.TryAcquireCustomCooldown(userID, "music", 20*time.Second); err != nil {
			log.Printf("music cooldown error: %v", err)
		} else if !ok {
			wait := ttlLeft
			if wait < time.Second {
				wait = time.Second
			}
			time.Sleep(wait)
			if ok2, _, err2 := b.redisClient.TryAcquireCustomCooldown(userID, "music", 20*time.Second); err2 != nil {
				log.Printf("music cooldown recheck error: %v", err2)
			} else if !ok2 {
				log.Printf("music cooldown still active, skipping generation")
				_ = b.userService.RefundQuota(userID, models.QuotaCategoryMusic, primaryUsed, extraUsed)
				return
			}
		}

		apiModel := modelOpt.ApiModel
		if apiModel == "" {
			apiModel = modelOpt.ID
		}
		var resultURL string
		var taskID string
		var err error
		if apiModel == "music-suno" || strings.Contains(strings.ToLower(apiModel), "suno") {
			sunoPrompt := text
			if voice == "f" {
				sunoPrompt += "\nженский голос"
			} else {
				sunoPrompt += "\nмужской голос"
			}
			resultURL, taskID, err = b.generationService.GenerateMusicSuno(sunoPrompt, voice, instrumental)
		} else if modelOpt.Category == ModelCategoryMusic {
			_ = b.userService.RefundQuota(userID, models.QuotaCategoryMusic, primaryUsed, extraUsed)
			b.sendErrorMessage(chatID, "Музыкальная генерация доступна только через Suno.")
			return
		} else {
			resultURL, err = b.generationService.GenerateAudio(text, apiModel, modelOpt.TaskType, "")
		}
		if err != nil {
			req, logErr := b.generationService.LogRequest(userID, services.LogRequestOptions{
				Username:          username,
				ModelType:         string(modelOpt.Category),
				Model:             apiModel,
				Prompt:            text,
				Status:            "failed",
				ErrorMsg:          err.Error(),
				TokensUsed:        requestCost,
				TokensPrimaryUsed: primaryUsed,
				TokensExtraUsed:   extraUsed,
			})
			if logErr != nil {
				log.Printf("LogRequest(music failed) error: %v", logErr)
			}
			_ = b.userService.RefundQuota(userID, models.QuotaCategoryMusic, primaryUsed, extraUsed)
			if req != nil {
				if resetErr := b.generationService.ResetRequestTokensUsed(req.ID); resetErr != nil {
					log.Printf("ResetRequestTokensUsed(music failed) error: %v", resetErr)
				}
			}
			b.sendErrorMessage(chatID, fmt.Sprintf("Не удалось озвучить: %s", friendlyGenerationError(err)))
			return
		}

		if resultURL != "" {
			if _, logErr := b.generationService.LogRequest(userID, services.LogRequestOptions{
				Username:          username,
				ModelType:         string(modelOpt.Category),
				Model:             apiModel,
				Prompt:            text,
				Status:            "completed",
				Output:            resultURL,
				TokensUsed:        requestCost,
				TokensPrimaryUsed: primaryUsed,
				TokensExtraUsed:   extraUsed,
			}); logErr != nil {
				log.Printf("LogRequest(music completed) error: %v", logErr)
			}
			caption := fmt.Sprintf("🔊 Аудио готово\nМодель: %s\nСписано: %d музыкальных запрос(ов)", modelOpt.Label, requestCost)
			b.sendAudioResult(chatID, caption, resultURL)
			return
		}

		if taskID != "" {
			req, logErr := b.generationService.LogRequest(userID, services.LogRequestOptions{
				Username:          username,
				ModelType:         string(modelOpt.Category),
				Model:             apiModel,
				Prompt:            text,
				ExternalTaskID:    taskID,
				Status:            "processing",
				TokensUsed:        requestCost,
				TokensPrimaryUsed: primaryUsed,
				TokensExtraUsed:   extraUsed,
			})
			if logErr != nil {
				log.Printf("LogRequest(music processing) error: %v", logErr)
			}
			requestID := int64(0)
			if req != nil {
				requestID = req.ID
			}
			b.registerSunoTask(taskID, sunoTask{
				ChatID:      chatID,
				UserID:      userID,
				RequestID:   requestID,
				RequestCost: requestCost,
				PrimaryUsed: primaryUsed,
				ExtraUsed:   extraUsed,
				ModelLabel:  modelOpt.Label,
				Prompt:      text,
				CreatedAt:   time.Now(),
			})
			return
		}

		// На всякий случай, если нет ни URL, ни taskId
		b.sendErrorMessage(chatID, "Сервис не вернул ссылку и taskId. Попробуйте позже.")
		req, logErr := b.generationService.LogRequest(userID, services.LogRequestOptions{
			Username:          username,
			ModelType:         string(modelOpt.Category),
			Model:             apiModel,
			Prompt:            text,
			Status:            "failed",
			ErrorMsg:          "service returned empty result and taskId",
			TokensUsed:        requestCost,
			TokensPrimaryUsed: primaryUsed,
			TokensExtraUsed:   extraUsed,
		})
		if logErr != nil {
			log.Printf("LogRequest(music empty result) error: %v", logErr)
		}
		_ = b.userService.RefundQuota(userID, models.QuotaCategoryMusic, primaryUsed, extraUsed)
		if req != nil {
			if resetErr := b.generationService.ResetRequestTokensUsed(req.ID); resetErr != nil {
				log.Printf("ResetRequestTokensUsed(music empty result) error: %v", resetErr)
			}
		}
	})
}

func (b *Bot) handleAdminBroadcastPhoto(chatID int64, fileID tgbotapi.FileID, text string) {
	const (
		batchSize    = 500
		captionLimit = 900
		numWorkers   = 20
	)

	caption := strings.TrimSpace(text)
	remaining := ""
	if len(caption) > captionLimit {
		caption = strings.TrimSpace(caption[:captionLimit])
		remaining = strings.TrimSpace(text[len(caption):])
	}

	b.sendText(chatID, "📣 Запускаю рассылку с фото...")
	b.goLimited(func() {
		// Collect all users first
		var allUsers []int64
		for offset := 0; ; offset += batchSize {
			users, err := b.userService.GetAllUsers(batchSize, offset)
			if err != nil {
				b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка при получении пользователей: %v", err))
				return
			}
			if len(users) == 0 {
				break
			}
			for _, user := range users {
				if user != nil && user.TelegramID != 0 {
					allUsers = append(allUsers, user.TelegramID)
				}
			}
		}

		total := len(allUsers)
		if total == 0 {
			b.sendText(chatID, "❌ Нет пользователей для рассылки")
			return
		}

		b.sendText(chatID, fmt.Sprintf("📊 Найдено %d пользователей, начинаю рассылку...", total))

		// Create channels for worker pool
		jobs := make(chan int64, total)
		results := make(chan bool, total)

		// Start workers
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for userID := range jobs {
					captionHTML := ""
					parseMode := ""
					if caption != "" {
						captionHTML = convertBroadcastMarkupToHTML(caption)
						parseMode = "HTML"
					}
					if _, err := b.sendPhoto(userID, &tgmodels.InputFileString{Data: string(fileID)}, captionHTML, parseMode, nil); err != nil {
						log.Printf("broadcast photo to %d failed: %v", userID, err)
						results <- false
					} else {
						if remaining != "" {
							if err := b.sendMarkdownLong(userID, remaining); err != nil {
								log.Printf("broadcast photo extra text to %d failed: %v", userID, err)
							}
						}
						results <- true
					}
					time.Sleep(35 * time.Millisecond)
				}
			}()
		}

		// Send jobs to workers
		go func() {
			for _, userID := range allUsers {
				jobs <- userID
			}
			close(jobs)
		}()

		// Wait for all workers to finish
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		sent := 0
		failed := 0
		for success := range results {
			if success {
				sent++
			} else {
				failed++
			}
		}

		b.sendText(chatID, fmt.Sprintf("✅ Рассылка завершена\nПолучателей: %d\nУспешно: %d\nОшибки: %d", total, sent, failed))
	})
}

// registerSunoTask сохраняет информацию о запросе Suno для дальнейшего callback
func (b *Bot) registerSunoTask(taskID string, task sunoTask) {
	if taskID == "" {
		return
	}
	b.sunoMu.Lock()
	b.sunoTasks[taskID] = task
	b.sunoMu.Unlock()
	log.Printf("Suno task registered: %s for chat %d", taskID, task.ChatID)
}

// HandleSunoCallback обрабатывает callback от Suno с готовыми аудио (может быть несколько вариантов)
func (b *Bot) HandleSunoCallback(taskID string, audioURLs []string) {
	if taskID == "" || len(audioURLs) == 0 {
		log.Printf("Suno callback missing taskID or audioURLs: taskID=%s", taskID)
		return
	}

	valid := make([]string, 0, len(audioURLs))
	for _, url := range audioURLs {
		if url != "" {
			valid = append(valid, url)
		}
	}
	if len(valid) == 0 {
		return
	}

	now := time.Now()

	b.sunoMu.Lock()
	task, ok := b.sunoTasks[taskID]
	if !ok {
		b.sunoMu.Unlock()
		log.Printf("Suno callback unknown taskID=%s", taskID)
		return
	}

	// merge unique URLs
	existing := make(map[string]struct{}, len(task.AudioURLs))
	for _, u := range task.AudioURLs {
		existing[u] = struct{}{}
	}
	for _, u := range valid {
		if _, seen := existing[u]; !seen {
			task.AudioURLs = append(task.AudioURLs, u)
			existing[u] = struct{}{}
		}
	}

	if task.FirstAudio.IsZero() {
		task.FirstAudio = now
	}

	// Decide if we can send now
	readyToSend := len(task.AudioURLs) >= 2

	// Calculate next deadline if waiting for second variant
	if !readyToSend {
		deadlineSecond := task.FirstAudio.Add(6 * time.Minute)
		deadlineHard := task.CreatedAt.Add(8 * time.Minute)
		if deadlineHard.Before(deadlineSecond) {
			deadlineSecond = deadlineHard
		}
		delay := time.Until(deadlineSecond)
		if delay < 0 {
			delay = 0
		}
		if task.Timer != nil {
			task.Timer.Stop()
		}
		if delay == 0 {
			readyToSend = true
		} else {
			// schedule finalize
			task.Timer = time.AfterFunc(delay, func() {
				b.finalizeSunoTask(taskID, "timeout")
			})
		}
	}

	b.sunoTasks[taskID] = task
	b.sunoMu.Unlock()

	if readyToSend {
		b.finalizeSunoTask(taskID, "complete")
	}
}

// finalizeSunoTask отправляет накопленные аудио и очищает задачу
func (b *Bot) finalizeSunoTask(taskID string, reason string) {
	b.sunoMu.Lock()
	task, ok := b.sunoTasks[taskID]
	if ok {
		delete(b.sunoTasks, taskID)
		if task.Timer != nil {
			task.Timer.Stop()
		}
	}
	b.sunoMu.Unlock()

	if !ok {
		log.Printf("Suno finalize: unknown taskID=%s", taskID)
		return
	}

	urls := task.AudioURLs
	if len(urls) == 0 {
		log.Printf("Suno finalize: no audio urls for task=%s reason=%s", taskID, reason)
		return
	}
	if len(urls) > 2 {
		urls = urls[:2]
	}

	if len(urls) > 1 {
		b.sendText(task.ChatID, "Вот 2 разных варианта песни:")
	}

	for i, url := range urls {
		caption := fmt.Sprintf("🔊 Аудио готово (%d/%d)\nМодель: %s\nСписано: %d музыкальных запрос(ов)", i+1, len(urls), task.ModelLabel, task.RequestCost)
		b.sendAudioResult(task.ChatID, caption, url)
	}
	if len(urls) > 0 {
		if err := b.generationService.CompleteRequestByExternalTaskID(taskID, urls[0]); err != nil {
			log.Printf("CompleteRequestByExternalTaskID error: %v", err)
		}
	}
}

func (b *Bot) HandleSunoError(taskID, errMsg string) {
	if taskID == "" {
		return
	}

	b.sunoMu.Lock()
	task, ok := b.sunoTasks[taskID]
	if ok {
		delete(b.sunoTasks, taskID)
		if task.Timer != nil {
			task.Timer.Stop()
		}
	}
	b.sunoMu.Unlock()

	if !ok {
		log.Printf("Suno error unknown taskID=%s", taskID)
		return
	}

	if err := b.generationService.FailRequestByExternalTaskID(taskID, strings.TrimSpace(errMsg)); err != nil {
		log.Printf("FailRequestByExternalTaskID error: %v", err)
	}
	_ = b.userService.RefundQuota(task.UserID, models.QuotaCategoryMusic, task.PrimaryUsed, task.ExtraUsed)
	if task.RequestID != 0 {
		if resetErr := b.generationService.ResetRequestTokensUsed(task.RequestID); resetErr != nil {
			log.Printf("ResetRequestTokensUsed(suno error) error: %v", resetErr)
		}
	}
	message := fmt.Sprintf("Не удалось сгенерировать песню: %s", strings.TrimSpace(errMsg))
	if strings.TrimSpace(errMsg) == "" {
		message = "Не удалось сгенерировать песню. Попробуйте изменить запрос и повторить."
	}
	if task.Prompt != "" {
		message += "\n\nЗапрос:\n" + truncate(task.Prompt, 700)
	}
	b.sendErrorMessage(task.ChatID, message)
}

func (b *Bot) getSunoInstrumental(userID int64) bool {
	b.sunoInstrMu.Lock()
	defer b.sunoInstrMu.Unlock()
	v, ok := b.sunoInstrumental[userID]
	if !ok {
		b.sunoInstrumental[userID] = false
		return false
	}
	return v
}

func (b *Bot) setSunoInstrumental(userID int64, v bool) {
	b.sunoInstrMu.Lock()
	b.sunoInstrumental[userID] = v
	b.sunoInstrMu.Unlock()
}

func (b *Bot) getSunoVoice(userID int64) string {
	b.sunoVoiceMu.Lock()
	defer b.sunoVoiceMu.Unlock()
	voice, ok := b.sunoVoice[userID]
	if !ok {
		b.sunoVoice[userID] = "m"
		return "m"
	}
	if voice != "f" {
		return "m"
	}
	return voice
}

func (b *Bot) setSunoVoice(userID int64, v string) {
	b.sunoVoiceMu.Lock()
	if v != "f" {
		v = "m"
	}
	b.sunoVoice[userID] = v
	b.sunoVoiceMu.Unlock()
}

// labelWithCheck оставлено для совместимости, сейчас не используется
func labelWithCheck(label string, active bool) string {
	if active {
		return "✅ " + label
	}
	return label
}

func (b *Bot) sendSunoBalance(chatID int64) {
	apiKey := os.Getenv("SUNO_API_KEY")
	if apiKey == "" {
		b.sendErrorMessage(chatID, "SUNO_API_KEY не задан в окружении")
		return
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.sunoapi.org/api/v1/generate/credit", nil)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка создания запроса: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Запрос баланса не удался: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка чтения ответа: %v", err))
		return
	}

	var (
		balanceStr string
		rawShown   = string(body)
	)

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		if num, ok := findNumberByKeys(payload, "credit", "credits", "balance", "available_credits"); ok {
			balanceStr = fmt.Sprintf("%.0f", num)
		} else if s, ok := findStringByKeys(payload, "credit", "credits", "balance", "available_credits"); ok {
			balanceStr = s
		}
	}

	if balanceStr != "" {
		msg := fmt.Sprintf("Баланс Suno API: %s\n(HTTP %d)", balanceStr, resp.StatusCode)
		b.sendText(chatID, msg)
	} else {
		b.sendText(chatID, fmt.Sprintf("Баланс Suno API (HTTP %d):\n%s", resp.StatusCode, rawShown))
	}

	b.sendDefAPIBalance(chatID)
	b.sendKieAPIBalance(chatID)
}

func (b *Bot) sendDefAPIBalance(chatID int64) {
	if b.cfg == nil {
		b.sendErrorMessage(chatID, "Конфиг не загружен")
		return
	}
	apiKey := strings.TrimSpace(b.cfg.DefAPI.APIKey)
	baseURL := strings.TrimSpace(b.cfg.DefAPI.BaseURL)
	if apiKey == "" || baseURL == "" {
		b.sendErrorMessage(chatID, "DEF_API_KEY или DEF_BASE_URL не заданы")
		return
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := baseURL + "/api/user"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка создания DefAPI запроса: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Запрос DefAPI баланса не удался: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка чтения ответа DefAPI: %v", err))
		return
	}

	var (
		balanceStr string
		rawShown   = string(body)
	)

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		if s, ok := findStringByKeys(payload, "credit"); ok {
			balanceStr = s
		} else if num, ok := findNumberByKeys(payload, "credit"); ok {
			balanceStr = fmt.Sprintf("%.8f", num)
		}
	}

	if balanceStr != "" {
		msg := fmt.Sprintf("Баланс DefAPI: %s\n(HTTP %d)", balanceStr, resp.StatusCode)
		b.sendText(chatID, msg)
		return
	}

	b.sendText(chatID, fmt.Sprintf("Баланс DefAPI (HTTP %d):\n%s", resp.StatusCode, rawShown))
}

func (b *Bot) sendKieAPIBalance(chatID int64) {
	if b.cfg == nil {
		b.sendErrorMessage(chatID, "Конфиг не загружен")
		return
	}
	apiKey := strings.TrimSpace(b.cfg.KieAPI.APIKey)
	baseURL := strings.TrimSpace(b.cfg.KieAPI.BaseURL)
	if apiKey == "" || baseURL == "" {
		b.sendErrorMessage(chatID, "KIEAPI_API_KEY или KIEAPI_BASE_URL не заданы")
		return
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := baseURL + "/api/v1/chat/credit"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка создания KieAPI запроса: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Запрос KieAPI баланса не удался: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка чтения ответа KieAPI: %v", err))
		return
	}

	var (
		balanceStr string
		rawShown   = string(body)
	)

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		if s, ok := findStringByKeys(payload, "credit", "credits", "balance", "available_credits", "available"); ok {
			balanceStr = s
		} else if num, ok := findNumberByKeys(payload, "credit", "credits", "balance", "available_credits", "available"); ok {
			balanceStr = fmt.Sprintf("%.8f", num)
		}
	}

	if balanceStr != "" {
		msg := fmt.Sprintf("Баланс KieAPI: %s\n(HTTP %d)", balanceStr, resp.StatusCode)
		b.sendText(chatID, msg)
		return
	}

	b.sendText(chatID, fmt.Sprintf("Баланс KieAPI (HTTP %d):\n%s", resp.StatusCode, rawShown))
}

// findNumberByKeys ищет первое числовое значение по ключам (float64/числовая строка)
func findNumberByKeys(v interface{}, keys ...string) (float64, bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if n, okNum := toFloat(raw); okNum {
					return n, true
				}
			}
		}
		for _, vv := range val {
			if n, ok := findNumberByKeys(vv, keys...); ok {
				return n, true
			}
		}
	case []interface{}:
		for _, vv := range val {
			if n, ok := findNumberByKeys(vv, keys...); ok {
				return n, true
			}
		}
	}
	return 0, false
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// findStringByKeys ищет первую непустую строку по ключам
func findStringByKeys(v interface{}, keys ...string) (string, bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if raw, ok := val[k]; ok {
				if s, ok2 := raw.(string); ok2 && strings.TrimSpace(s) != "" {
					return s, true
				}
			}
		}
		for _, vv := range val {
			if s, ok := findStringByKeys(vv, keys...); ok {
				return s, true
			}
		}
	case []interface{}:
		for _, vv := range val {
			if s, ok := findStringByKeys(vv, keys...); ok {
				return s, true
			}
		}
	}
	return "", false
}

// ===== Chat style settings =====

func (b *Bot) getUserChatStyle(userID int64) string {
	style, err := b.redisClient.GetUserChatStyle(userID)
	if err != nil {
		log.Printf("getUserChatStyle err: %v", err)
	}
	if style == "" {
		return "normal"
	}
	for _, st := range chatStyles {
		if st.ID == style {
			return style
		}
	}
	return "normal"
}

func (b *Bot) setUserChatStyle(userID int64, style string) {
	if style == "" {
		style = "normal"
	}
	found := false
	for _, st := range chatStyles {
		if st.ID == style {
			found = true
			break
		}
	}
	if !found {
		style = "normal"
	}
	if err := b.redisClient.SetUserChatStyle(userID, style); err != nil {
		log.Printf("setUserChatStyle err: %v", err)
	}
}

func (b *Bot) isChatStyleAllowed(userID int64) bool {
	label, active := b.userService.GetSubscriptionLabel(userID)
	if !active {
		return false
	}
	return strings.EqualFold(label, "pro")
}

func (b *Bot) ensureChatStyleAllowed(chatID int64, userID int64) bool {
	if b.isChatStyleAllowed(userID) {
		return true
	}
	loc := b.getLocalization(userID)
	b.sendErrorMessage(chatID, loc.ChatStyleProOnly)
	return false
}

func (b *Bot) buildChatSystemPrompt(userID int64) string {
	loc := b.getLocalization(userID)
	basePrompt := loc.ChatSystemPrompt
	if basePrompt == "" {
		basePrompt = loc.ChatSystemPrompt
	}
	if hint := strings.TrimSpace(loc.ChatSystemLangHint); hint != "" {
		basePrompt += " " + hint
	}

	styleID := b.getUserChatStyle(userID)
	stylePrompt := b.chatStylePrompt(styleID, loc)
	if stylePrompt == "" {
		return basePrompt
	}

	return basePrompt + " " + stylePrompt
}

func (b *Bot) getUserLanguage(userID int64) string {
	lang, err := b.redisClient.GetUserLanguage(userID)
	if err != nil {
		log.Printf("getUserLanguage error: %v", err)
		return "ru"
	}
	if lang == "" {
		return "ru"
	}
	return lang
}

func (b *Bot) setUserLanguage(userID int64, lang string) {
	if lang != "ru" && lang != "en" {
		lang = "ru"
	}
	if err := b.redisClient.SetUserLanguage(userID, lang); err != nil {
		log.Printf("setUserLanguage error: %v", err)
	}
}

func (b *Bot) getLocalization(userID int64) *Localization {
	lang := b.getUserLanguage(userID)
	return GetLocalization(lang)
}

func (b *Bot) chatStyleLabel(styleID string, loc *Localization) string {
	switch styleID {
	case "normal":
		return loc.StyleNormal
	case "formal":
		return loc.StyleFormal
	case "humor":
		return loc.StyleHumor
	case "informal":
		return loc.StyleInformal
	case "friendly":
		return loc.StyleFriendly
	case "expert":
		return loc.StyleExpert
	case "empathetic":
		return loc.StyleEmpathetic
	default:
		return styleID
	}
}

func (b *Bot) chatStylePrompt(styleID string, loc *Localization) string {
	switch styleID {
	case "normal":
		return loc.StylePromptNormal
	case "formal":
		return loc.StylePromptFormal
	case "humor":
		return loc.StylePromptHumor
	case "informal":
		return loc.StylePromptInformal
	case "friendly":
		return loc.StylePromptFriendly
	case "expert":
		return loc.StylePromptExpert
	case "empathetic":
		return loc.StylePromptEmpathetic
	default:
		return ""
	}
}

func (b *Bot) modelLabelLoc(id string, loc *Localization) string {
	if opt, ok := findModelOption(id); ok {
		return opt.Label
	}
	return id
}

func (b *Bot) modelDescriptionLoc(id string, loc *Localization) string {
	switch id {
	case "google/nano-banana":
		return loc.ModelNanoBanana
	case "google/nano-banana-pro":
		return loc.ModelNanoBananaPro
	case "nano-banana-2":
		return loc.ModelNanoBanana2
	case "seedream/4.5-edit":
		return loc.ModelSeedream
	case "veo3_fast":
		return loc.ModelVeo3Fast
	case "wan/2-6-image-to-video":
		return loc.ModelWan26
	case "kling-2.6/image-to-video":
		return loc.ModelKling26
	case "music-suno":
		return loc.ModelSunoMusic
	case "google/gemini-3-flash":
		return loc.ModelGeminiFlash
	case "openai/gpt-5-mini":
		return loc.ModelGPT5Mini
	case "openai/gpt-5-nano":
		return loc.ModelGPT5Nano
	case "chat-gpt-4.1mini":
		return loc.ModelGPT41Mini
	default:
		return id
	}
}

func (b *Bot) modelInstructionLoc(id string, loc *Localization) string {
	if opt, ok := findModelOption(id); ok {
		switch opt.ID {
		case "google/nano-banana", "google/nano-banana-pro", "nano-banana-2", "seedream/4.5-edit":
			return loc.InstrNanoBanana
		case "veo3_fast":
			return loc.InstrVeo3Video
		case "music-suno":
			return loc.InstrSunoMusic
		}
		if opt.Category == ModelCategoryChat {
			return loc.InstrTextModels
		}
	}
	return ""
}

func (b *Bot) categoryLabelLoc(cat ModelCategory, loc *Localization) string {
	switch cat {
	case ModelCategoryPhoto:
		return loc.ModelsCatPhoto
	case ModelCategoryVideo:
		return loc.ModelsCatVideo
	case ModelCategoryMusic:
		return loc.ModelsCatMusic
	case ModelCategoryChat:
		return loc.ModelsCatChat
	default:
		return string(cat)
	}
}

func (b *Bot) sendSettingsMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	rows := [][]tgmodels.InlineKeyboardButton{
		{
			newInlineKeyboardButtonData(loc.SettingsChatStyle, "settings:style"),
		},
		{
			newInlineKeyboardButtonData(loc.SettingsLanguage, "settings:language"),
		},
	}
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.SettingsBackBtn, "menu"),
	))

	text := loc.SettingsTitle + "\n\n" + loc.SettingsSelect
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = newInlineKeyboardMarkup(rows...)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send settings menu: %v", err)
	}
}

func (b *Bot) sendChatStyleMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	if !b.ensureChatStyleAllowed(chatID, userID) {
		return
	}
	current := b.getUserChatStyle(userID)
	var rows [][]tgmodels.InlineKeyboardButton
	for i := 0; i < len(chatStyles); i += 2 {
		btns := []tgmodels.InlineKeyboardButton{}
		for j := i; j < len(chatStyles) && j < i+2; j++ {
			st := chatStyles[j]
			label := b.chatStyleLabel(st.ID, loc)
			if st.ID == current {
				label = "✅ " + label
			}
			btns = append(btns, newInlineKeyboardButtonData(label, "set_style:"+st.ID))
		}
		rows = append(rows, newInlineKeyboardRow(btns...))
	}
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn, "settings"),
	))

	text := loc.StyleSelectTitle
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = newInlineKeyboardMarkup(rows...)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send chat style menu: %v", err)
	}
}

func (b *Bot) sendLanguageMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	currentLang := b.getUserLanguage(userID)

	ruLabel := loc.LangRussian
	enLabel := loc.LangEnglish
	if currentLang == "ru" {
		ruLabel = "✅ " + ruLabel
	} else {
		enLabel = "✅ " + enLabel
	}

	rows := [][]tgmodels.InlineKeyboardButton{
		{
			newInlineKeyboardButtonData(ruLabel, "set_lang:ru"),
			newInlineKeyboardButtonData(enLabel, "set_lang:en"),
		},
	}
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn, "settings"),
		newInlineKeyboardButtonData(loc.MenuBtn, "menu"),
	))

	msg := newMessageConfig(chatID, loc.LangTitle)
	msg.ReplyMarkup = newInlineKeyboardMarkup(rows...)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send language menu: %v", err)
	}
}

// handleAnimatePhoto обрабатывает нажатие кнопки "Оживить фото" — берёт фото из сообщения и запускает veo3_fast
func (b *Bot) handleAnimatePhoto(chatID int64, userID int64, callback *tgmodels.CallbackQuery) {
	cbMsg := getCallbackMessage(callback)
	if cbMsg == nil || cbMsg.Photo == nil || len(cbMsg.Photo) == 0 {
		b.sendErrorMessage(chatID, "Не удалось найти фото в сообщении.")
		return
	}

	// Берём самое большое фото из сообщения
	photo := cbMsg.Photo[len(cbMsg.Photo)-1]
	file, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: photo.FileID})
	if err != nil {
		log.Printf("animate_photo: failed to get file: %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить фото для оживления.")
		return
	}
	photoURL := b.getFileURL(file)

	veoOpt, ok := findModelOption("veo3_fast")
	if !ok {
		b.sendErrorMessage(chatID, "Модель Veo 3.1 Fast не найдена.")
		return
	}

	b.processVideoGeneration(chatID, userID, photoURL, "оживи фото", veoOpt)
}

// processVideoGenerationMultiPhoto обрабатывает видео-генерацию из нескольких фото
func (b *Bot) processVideoGenerationMultiPhoto(chatID int64, userID int64, photoURLs []string, prompt string, modelOpt ModelOption) {
	if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
		return
	}

	if modelOpt.Category != ModelCategoryVideo {
		b.sendErrorMessage(chatID, "Выберите видео-модель в меню моделей.")
		return
	}

	if len(photoURLs) == 0 {
		b.sendErrorMessage(chatID, "Не удалось получить фото для видео-генерации")
		return
	}

	requestCost := modelOpt.RequestCost
	if requestCost < 1 {
		requestCost = 1
	}

	// For wan/2-6-image-to-video, calculate cost based on duration
	duration := "5"
	if modelOpt.ID == "wan/2-6-image-to-video" {
		requestCost = b.getVideoDurationCost(userID)
		duration = b.getUserVideoDuration(userID)
	}

	if err := b.userService.ConsumeQuota(userID, models.QuotaCategoryVideo, requestCost); err != nil {
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryVideo, requestCost, err)
		return
	}

	userRec, _ := b.userService.GetUserByTelegramID(userID)
	username := ""
	if userRec != nil {
		username = userRec.Username
	}
	resolution := b.getUserVideoResolution(userID)
	opts := services.GenerationOptions{
		InputImages:        photoURLs,
		InputImage:         photoURLs[0],
		Prompt:             prompt,
		TokensCost:         requestCost,
		ChatID:             chatID,
		Model:              modelOpt.ID,
		ModelType:          string(modelOpt.Category),
		Username:           username,
		AspectRatio:        duration,   // Pass duration via AspectRatio field
		NanoBananaProvider: resolution, // Pass resolution via NanoBananaProvider field
	}
	req, err := b.generationService.StartGeneration(userID, opts)
	if err != nil {
		_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryVideo, requestCost)
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка запуска видео генерации: %v", err))
		return
	}
	b.sendText(chatID, fmt.Sprintf("🔄 Запустили видео генерацию! ID: %d\nМодель: %s\nФото: %d\nДлительность: %s сек\nРазрешение: %s\nСписано: %d видео-запрос(ов)\n\nОжидайте результат...", req.ID, modelOpt.Label, len(photoURLs), duration, resolution, requestCost))
}

// processVideoGeneration обрабатывает видео-генерацию из фото (image_to_video)
func (b *Bot) processVideoGeneration(chatID int64, userID int64, photoURL string, prompt string, modelOpt ModelOption) {
	if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
		return
	}

	if modelOpt.Category != ModelCategoryVideo {
		b.sendErrorMessage(chatID, "Выберите видео-модель в меню моделей, чтобы оживить фото.")
		return
	}

	requestCost := modelOpt.RequestCost
	if requestCost < 1 {
		requestCost = 1
	}

	// For wan/2-6-image-to-video, calculate cost based on duration
	duration := "5"
	if modelOpt.ID == "wan/2-6-image-to-video" {
		requestCost = b.getVideoDurationCost(userID)
		duration = b.getUserVideoDuration(userID)
	}
	// For kling-2.6/image-to-video, calculate cost based on duration and sound
	if modelOpt.ID == "kling-2.6/image-to-video" {
		requestCost = b.getKlingVideoCost(userID)
		duration = b.getUserVideoDuration(userID)
		// Kling only supports 5 and 10 seconds
		if duration != "5" && duration != "10" {
			duration = "5"
		}
	}

	if err := b.userService.ConsumeQuota(userID, models.QuotaCategoryVideo, requestCost); err != nil {
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryVideo, requestCost, err)
		return
	}

	// kling-2.6/image-to-video — через KieAPI async с duration и sound
	if modelOpt.ID == "kling-2.6/image-to-video" {
		userRec, _ := b.userService.GetUserByTelegramID(userID)
		username := ""
		if userRec != nil {
			username = userRec.Username
		}
		sound := b.getUserVideoSound(userID)
		opts := services.GenerationOptions{
			InputImages:        []string{photoURL},
			InputImage:         photoURL,
			Prompt:             prompt,
			TokensCost:         requestCost,
			ChatID:             chatID,
			Model:              modelOpt.ID,
			ModelType:          string(modelOpt.Category),
			Username:           username,
			AspectRatio:        duration, // Pass duration via AspectRatio field
			NanoBananaProvider: sound,    // Pass sound via NanoBananaProvider field
		}
		req, err := b.generationService.StartGeneration(userID, opts)
		if err != nil {
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryVideo, requestCost)
			b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка запуска видео генерации: %v", err))
			return
		}
		soundLabel := "Без звука"
		if sound == "true" {
			soundLabel = "Со звуком"
		}
		b.sendText(chatID, fmt.Sprintf("🔄 Запустили видео генерацию! ID: %d\nМодель: %s\nДлительность: %s сек\nЗвук: %s\nСписано: %d видео-запрос(ов)\n\nОжидайте результат...", req.ID, modelOpt.Label, duration, soundLabel, requestCost))
		return
	}

	// wan/2-6-image-to-video — через KieAPI async с duration
	if modelOpt.ID == "wan/2-6-image-to-video" {
		userRec, _ := b.userService.GetUserByTelegramID(userID)
		username := ""
		if userRec != nil {
			username = userRec.Username
		}
		resolution := b.getUserVideoResolution(userID)
		opts := services.GenerationOptions{
			InputImages:        []string{photoURL},
			InputImage:         photoURL,
			Prompt:             prompt,
			TokensCost:         requestCost,
			ChatID:             chatID,
			Model:              modelOpt.ID,
			ModelType:          string(modelOpt.Category),
			Username:           username,
			AspectRatio:        duration,   // Pass duration via AspectRatio field
			NanoBananaProvider: resolution, // Pass resolution via NanoBananaProvider field
		}
		req, err := b.generationService.StartGeneration(userID, opts)
		if err != nil {
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryVideo, requestCost)
			b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка запуска видео генерации: %v", err))
			return
		}
		b.sendText(chatID, fmt.Sprintf("🔄 Запустили видео генерацию! ID: %d\nМодель: %s\nДлительность: %s сек\nРазрешение: %s\nСписано: %d видео-запрос(ов)\n\nОжидайте результат...", req.ID, modelOpt.Label, duration, resolution, requestCost))
		return
	}

	// veo3_fast — через KieAPI async (StartGeneration + callback)
	if modelOpt.ID == "veo3_fast" {
		userRec, _ := b.userService.GetUserByTelegramID(userID)
		username := ""
		if userRec != nil {
			username = userRec.Username
		}
		opts := services.GenerationOptions{
			InputImages: []string{photoURL},
			InputImage:  photoURL,
			Prompt:      prompt,
			TokensCost:  requestCost,
			ChatID:      chatID,
			Model:       modelOpt.ID,
			ModelType:   string(modelOpt.Category),
			Username:    username,
			AspectRatio: b.getAspectRatioForModel(userID, modelOpt.ID),
		}
		req, err := b.generationService.StartGeneration(userID, opts)
		if err != nil {
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryVideo, requestCost)
			b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка запуска видео генерации: %v", err))
			return
		}
		b.sendText(chatID, fmt.Sprintf("🔄 Запустили видео генерацию! ID: %d\nМодель: %s\nСписано: %d видео-запрос(ов)\n\nОжидайте результат...", req.ID, modelOpt.Label, requestCost))
		return
	}

	// Фолбек для других видео-моделей (синхронный путь)
	b.sendText(chatID, fmt.Sprintf("🔄 Запустили видео генерацию\nМодель: %s\nСписано: %d видео-запрос(ов)", modelOpt.Label, requestCost))

	apiModel := modelOpt.ApiModel
	if apiModel == "" {
		apiModel = modelOpt.ID
	}

	b.goLimited(func() {
		resultURL, err := b.generationService.GenerateVideoFromImage(photoURL, apiModel, modelOpt.TaskType)
		if err != nil {
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryVideo, requestCost)
			b.sendErrorMessage(chatID, fmt.Sprintf("Не удалось сделать видео: %s", friendlyGenerationError(err)))
			return
		}

		caption := fmt.Sprintf("🎬 Видео готово\nМодель: %s\nСписано: %d видео-запрос(ов)", modelOpt.Label, requestCost)
		b.sendVideoResult(chatID, caption, resultURL)
	})
}

// sendAudioResult отправляет аудио из URL или data: ссылки
func (b *Bot) sendAudioResult(chatID int64, caption, output string) {
	caption = truncate(caption, 900)

	if strings.HasPrefix(output, "data:") {
		audioBytes, fileName, err := decodeDataURLAudio(output)
		if err != nil {
			log.Printf("Failed to decode audio data URL: %v", err)
			b.sendText(chatID, truncate(caption+"\n\nАудио недоступно", 3800))
			return
		}
		if _, err := b.sendAudio(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(audioBytes)}, caption); err != nil {
			log.Printf("Failed to send audio (bytes): %v", err)
			b.sendText(chatID, truncate(caption+"\n\nАудио недоступно", 3800))
		}
		return
	}

	if strings.HasPrefix(output, "http") {
		audioBytes, fileName, err := downloadFileToBytes(output, "mp3")
		if err != nil {
			log.Printf("Failed to download audio url: %v", err)
			b.sendText(chatID, truncate(caption+"\n\nАудио недоступно", 3800))
			return
		}

		// Пытаемся отправить как аудио
		if _, err := b.sendAudio(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(audioBytes)}, caption); err != nil {
			log.Printf("Failed to send audio (bytes): %v, trying as document", err)
			// Пытаемся как документ (например, если формат flac не поддерживается как Audio)
			if _, err := b.sendDocument(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(audioBytes)}, caption); err != nil {
				log.Printf("Failed to send audio as document: %v", err)
				b.sendText(chatID, truncate(caption+"\n\nАудио недоступно", 3800))
			}
		}
		return
	}

	// Фолбек текстом
	b.sendText(chatID, truncate(caption+"\n\nАудио: "+output, 3800))
}

func (b *Bot) sendVideoResult(chatID int64, caption, output string) {
	caption = truncate(caption, 900)

	if strings.HasPrefix(output, "http") {
		videoBytes, fileName, err := downloadFileToBytes(output, "mp4")
		if err != nil {
			log.Printf("Failed to download video url: %v", err)
			b.sendText(chatID, truncate(caption+"\n\nВидео недоступно", 3800))
			return
		}

		if _, err := b.sendVideo(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(videoBytes)}, caption, ""); err != nil {
			log.Printf("Failed to send video: %v, trying as document", err)
			if _, err := b.sendDocument(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(videoBytes)}, caption); err != nil {
				log.Printf("Failed to send video as document: %v", err)
				b.sendText(chatID, truncate(caption+"\n\nВидео недоступно", 3800))
			}
		}
		return
	}

	b.sendText(chatID, truncate(caption+"\n\nВидео: "+output, 3800))
}

func (b *Bot) handleCallback(callback *tgmodels.CallbackQuery) {
	data := callback.Data
	chatID := getCallbackChatID(callback)
	userID := callback.From.ID

	// Мгновенно отвечаем, чтобы убрать индикатор ожидания
	_ = b.answerCallback(callback.ID, "")

	// Кулдаун на любые запросы (кроме настроек Suno)
	if !strings.HasPrefix(data, "suno_instr") && !strings.HasPrefix(data, "suno_voice") {
		if !b.checkCooldown(chatID, userID) {
			return
		}
	}

	// Middleware: проверка и выдача пробного запроса при подписке на канал
	// Исключаем trial:check, чтобы не было дублирования логики
	if data != "trial:check" {
		b.checkAndGrantChannelTrial(userID)
	}

	switch data {
	case "menu":
		b.sendMainMenu(chatID, userID)
	case "buy":
		if !b.ensurePaymentsEnabled(chatID) {
			return
		}
		b.sendBuyMenu(chatID, userID)
	case "suno_instr_toggle":
		cur := b.getSunoInstrumental(userID)
		b.setSunoInstrumental(userID, !cur)
		b.sendModelMenu(chatID, userID, ModelCategoryMusic, getCallbackMessageID(callback))
	case "suno_voice_toggle":
		cur := b.getSunoVoice(userID)
		if cur == "f" {
			b.setSunoVoice(userID, "m")
		} else {
			b.setSunoVoice(userID, "f")
		}
		b.sendModelMenu(chatID, userID, ModelCategoryMusic, getCallbackMessageID(callback))
	case "buy:extras":
		if !b.ensurePaymentsEnabled(chatID) {
			return
		}
		b.sendBuyExtrasMenu(chatID, userID)
	case "buy:sub":
		if !b.ensurePaymentsEnabled(chatID) {
			return
		}
		if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
			loc := b.getLocalization(userID)
			b.sendErrorMessage(chatID, loc.SubsUnavailable)
			return
		}
		b.sendBuySubscription(chatID, userID)
	case "buy_sub:mini":
		b.sendBuySubscriptionPayment(chatID, userID, "mini")
	case "buy_sub:start":
		b.sendBuySubscriptionPayment(chatID, userID, "start")
	case "buy_sub:pro":
		b.sendBuySubscriptionPayment(chatID, userID, "pro")
	case "buy:text":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryChat) {
			return
		}
		b.sendBuyExtrasCategory(chatID, userID, "", "buy_package:text")
	case "buy:image":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryPhoto) {
			return
		}
		b.sendBuyExtrasCategory(chatID, userID, "", "buy_package:image")
	case "buy:music":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryMusic) {
			return
		}
		b.sendBuyExtrasCategory(chatID, userID, "", "buy_package:music")
	case "buy:video":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
			return
		}
		b.sendBuyExtrasCategory(chatID, userID, "", "buy_package:video")
	case "admin:categories":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendAdminCategories(chatID)
	case "admin:cat:photo":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.toggleCategory(chatID, "photo")
	case "admin:cat:video":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.toggleCategory(chatID, "video")
	case "admin:cat:music":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.toggleCategory(chatID, "music")
	case "admin:cat:chat":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.toggleCategory(chatID, "chat")
	case "admin:menu":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendAdminMenu(chatID)
	case "admin:stats":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminStats(chatID)
	case "admin:stats:day":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminStatsPeriod(chatID, "day")
	case "admin:stats:week":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminStatsPeriod(chatID, "week")
	case "admin:stats:month":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminStatsPeriod(chatID, "month")
	case "admin:stats:all":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminStats(chatID)
	case "admin:users":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminUsers(chatID)
	case "admin:users_count":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminUsersCount(chatID)
	case "admin:payments":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendPaymentsStatus(chatID)
	case "admin:nano_api":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendNanoBananaAPIStatus(chatID)
	case "admin:subs":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendSubscriptionsStatus(chatID)
	case "admin:subs_toggle":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		if err := b.toggleSubscriptions(chatID); err != nil {
			b.sendErrorMessage(chatID, "Не удалось переключить подписки")
		}
	case "admin:help":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.handleAdminHelp(chatID)
	case "admin:payments_toggle":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.togglePayments(chatID)
	case "admin:pay_stats":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendPaymentStats(chatID)
	case "admin:pay_list":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendRecentPayments(chatID)
	case "admin:nano_api_toggle":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.toggleNanoBananaAPI(chatID)
	case "admin:suno_balance":
		if !b.ensureAdmin(chatID, userID) {
			return
		}
		b.sendSunoBalance(chatID)
	case "trial:check":
		claimed, err := b.userService.HasClaimedChannelTrial(userID)
		if err != nil {
			log.Printf("trial claimed check error: %v", err)
			b.sendErrorMessage(chatID, "Не удалось проверить пробный запрос")
			return
		}
		if claimed {
			b.sendText(chatID, "Вы уже получали пробный запрос")
			b.sendMainMenu(chatID, userID)
			return
		}
		member, mErr := b.isChannelMember(userID)
		if mErr != nil {
			log.Printf("trial member check error: %v", mErr)
			b.sendErrorMessage(chatID, "Не удалось проверить подписку. Попробуйте ещё раз.")
			return
		}
		if !member {
			b.sendText(chatID, "Не удалось проверить подписку, попробуйте еще раз")
			return
		}
		if err := b.userService.AddExtraQuota(userID, models.QuotaCategoryImage, 1); err != nil {
			log.Printf("trial grant quota error: %v", err)
			b.sendErrorMessage(chatID, "Не удалось выдать пробный запрос")
			return
		}
		if err := b.userService.MarkChannelTrialClaimed(userID); err != nil {
			log.Printf("trial mark claimed error: %v", err)
		}
		b.sendText(chatID, "Пробный запрос успешно выдан")
		b.sendMainMenu(chatID, userID)
	case "settings":
		b.sendSettingsMenu(chatID, userID)
	case "settings:style":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.sendChatStyleMenu(chatID, userID)
	case "settings:language":
		b.sendLanguageMenu(chatID, userID)
	case "set_lang:ru":
		b.setUserLanguage(userID, "ru")
		loc := b.getLocalization(userID)
		b.sendText(chatID, loc.LangChanged)
		b.sendLanguageMenu(chatID, userID)
		b.sendMainMenu(chatID, userID)
	case "set_lang:en":
		b.setUserLanguage(userID, "en")
		loc := b.getLocalization(userID)
		b.sendText(chatID, loc.LangChanged)
		b.sendLanguageMenu(chatID, userID)
		b.sendMainMenu(chatID, userID)
	case "aspect_menu":
		b.sendAspectRatioMenu(chatID, userID, 0)
	case "duration_menu":
		b.sendVideoDurationMenu(chatID, userID, 0)
	case "duration_menu_kling":
		b.sendVideoDurationMenuKling(chatID, userID, 0)
	case "resolution_menu":
		b.sendVideoResolutionMenu(chatID, userID, 0)
	case "sound_menu":
		b.sendVideoSoundMenu(chatID, userID, 0)
	case "set_style":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		// default style
		b.setUserChatStyle(userID, "normal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:normal":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "normal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:formal":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "formal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:humor":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "humor")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:informal":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "informal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:friendly":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "friendly")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:expert":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "expert")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:empathetic":
		if !b.ensureChatStyleAllowed(chatID, userID) {
			return
		}
		b.setUserChatStyle(userID, "empathetic")
		b.sendChatStyleMenu(chatID, userID)
	case "animate_photo":
		// Open Kling 2.6 filter menu instead of direct generation
		b.sendKlingAnimateMenu(chatID, userID, getCallbackMessageID(callback))
	case "kling_animate_start":
		b.handleKlingAnimateStart(chatID, userID, callback)
	case "invite":
		b.sendInviteInfo(chatID, userID)
	case "models_menu":
		msgID := getCallbackMessageID(callback)
		cbMsg := getCallbackMessage(callback)
		if cbMsg != nil && cbMsg.Photo != nil && len(cbMsg.Photo) > 0 {
			msgID = 0 // не редактируем фото-превью, отправляем новое меню без картинки
		}
		b.sendModelMenu(chatID, userID, ModelCategoryPhoto, msgID)
	default:
		// Обрабатываем confirm_generation
		if strings.HasPrefix(data, "confirm_generation:") {
			parts := strings.Split(data, ":")
			if len(parts) >= 2 {
				b.confirmGeneration(chatID, userID, parts[1])
			}
		} else if strings.HasPrefix(data, "aspect_set:") {
			ratio := strings.TrimPrefix(data, "aspect_set:")
			// If user tapped the current ratio, do nothing to avoid "message is not modified" errors.
			if b.getUserAspectRatio(userID) == ratio {
				return
			}
			b.setUserAspectRatio(userID, ratio)
			b.sendAspectRatioMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "duration_set:") {
			duration := strings.TrimPrefix(data, "duration_set:")
			if b.getUserVideoDuration(userID) == duration {
				return
			}
			b.setUserVideoDuration(userID, duration)
			b.sendVideoDurationMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "resolution_set:") {
			resolution := strings.TrimPrefix(data, "resolution_set:")
			if b.getUserVideoResolution(userID) == resolution {
				return
			}
			b.setUserVideoResolution(userID, resolution)
			b.sendVideoResolutionMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "sound_set:") {
			sound := strings.TrimPrefix(data, "sound_set:")
			if b.getUserVideoSound(userID) == sound {
				return
			}
			b.setUserVideoSound(userID, sound)
			b.sendVideoSoundMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "sound_toggle:") {
			sound := strings.TrimPrefix(data, "sound_toggle:")
			b.setUserVideoSound(userID, sound)
			b.sendModelMenu(chatID, userID, ModelCategoryVideo, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "kling_animate_duration:") {
			duration := strings.TrimPrefix(data, "kling_animate_duration:")
			if b.getUserVideoDuration(userID) == duration {
				return
			}
			b.setUserVideoDuration(userID, duration)
			b.sendKlingAnimateMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "kling_animate_sound:") {
			sound := strings.TrimPrefix(data, "kling_animate_sound:")
			if b.getUserVideoSound(userID) == sound {
				return
			}
			b.setUserVideoSound(userID, sound)
			b.sendKlingAnimateMenu(chatID, userID, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "duration_toggle:") {
			duration := strings.TrimPrefix(data, "duration_toggle:")
			b.setUserVideoDuration(userID, duration)
			b.sendModelMenu(chatID, userID, ModelCategoryVideo, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "duration_toggle_kling:") {
			duration := strings.TrimPrefix(data, "duration_toggle_kling:")
			b.setUserVideoDuration(userID, duration)
			b.sendModelMenu(chatID, userID, ModelCategoryVideo, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "resolution_toggle:") {
			resolution := strings.TrimPrefix(data, "resolution_toggle:")
			b.setUserVideoResolution(userID, resolution)
			b.sendModelMenu(chatID, userID, ModelCategoryVideo, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "photo_resolution_toggle:") {
			resolution := strings.TrimPrefix(data, "photo_resolution_toggle:")
			b.setUserPhotoResolution(userID, resolution)
			b.sendModelMenu(chatID, userID, ModelCategoryPhoto, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "google_search_toggle:") {
			val := strings.TrimPrefix(data, "google_search_toggle:")
			b.setUserGoogleSearch(userID, val)
			b.sendModelMenu(chatID, userID, ModelCategoryPhoto, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "models_menu:") {
			cat := ModelCategory(strings.TrimPrefix(data, "models_menu:"))
			b.sendModelMenu(chatID, userID, cat, getCallbackMessageID(callback))
		} else if strings.HasPrefix(data, "video_total:") {
			modelID := strings.TrimPrefix(data, "video_total:")
			b.sendVideoTotalInfo(chatID, userID, modelID)
		} else if strings.HasPrefix(data, "buy_package:") {
			parts := strings.Split(data, ":")
			if len(parts) == 3 {
				category := parts[1]
				pack := parts[2]
				b.sendBuyPackageInfo(chatID, userID, category, pack)
			}
		} else if strings.HasPrefix(data, "admin:stats:") {
			period := strings.TrimPrefix(data, "admin:stats:")
			if period == "all" {
				b.handleAdminStats(chatID)
			} else {
				b.handleAdminStatsPeriod(chatID, period)
			}
		} else if data == "admin:top_users" {
			b.handleAdminTopUsers(chatID)
		} else if strings.HasPrefix(data, "model_set:") {
			model := strings.TrimPrefix(data, "model_set:")
			opt, ok := findModelOption(model)
			if !ok {
				b.sendErrorMessage(chatID, "Неизвестная модель")
				return
			}
			if !b.isModelVisibleToUser(userID, opt) {
				b.sendErrorMessage(chatID, "Доступ к этой модели только для админов")
				return
			}
			if opt.Category == ModelCategoryChat && !b.isChatModelAllowed(userID, opt) {
				b.sendErrorMessage(chatID, b.chatModelAccessMessage(opt.ID))
				return
			}
			if opt.ID == "seedream/4.5-edit" {
				if provider, err := b.userService.GetNanoBananaProvider(); err == nil && strings.EqualFold(provider, "defapi") {
					b.sendErrorMessage(chatID, "⚠️ Модель Seedream временно недоступна. Попробуйте позже.")
					return
				}
			}
			b.setUserModel(userID, model)

			// Обновляем меню моделей в том же сообщении
			b.sendModelMenu(chatID, userID, opt.Category, getCallbackMessageID(callback))
		}
	}

	// Отвечаем на callback
	// (пусто — ответили мгновенно в начале)
}

func (b *Bot) isChannelMember(userID int64) (bool, error) {
	member, err := b.api.GetChatMember(b.ctx, &tgbot.GetChatMemberParams{
		ChatID: requiredChannelUsername,
		UserID: userID,
	})
	if err != nil {
		return false, err
	}
	// Check member type
	switch member.Type {
	case tgmodels.ChatMemberTypeOwner, tgmodels.ChatMemberTypeAdministrator, tgmodels.ChatMemberTypeMember:
		return true, nil
	case tgmodels.ChatMemberTypeRestricted:
		if member.Restricted != nil && member.Restricted.IsMember {
			return true, nil
		}
	}
	return false, nil
}

func (b *Bot) sendChannelTrialMenu(chatID int64) {
	checkBtn := newInlineKeyboardButtonData("Проверить подписку", "trial:check")
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(checkBtn),
	)
	text := "Подпишитесь на канал t.me/AIFaceApps, чтобы получить 1 пробную генерацию фото.\nПосле подписки нажмите «Проверить подписку» и попробуйте снова."
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("sendChannelTrialMenu send message error: %v", err)
	}
}

// checkAndGrantChannelTrial проверяет подписку на канал и выдаёт пробный запрос
// если пользователь ещё не получал его (channel_trial_claimed = false)
func (b *Bot) checkAndGrantChannelTrial(userID int64) {
	// Проверяем, получал ли пользователь уже пробный запрос
	claimed, err := b.userService.HasClaimedChannelTrial(userID)
	if err != nil {
		log.Printf("checkAndGrantChannelTrial HasClaimedChannelTrial error: %v", err)
		return
	}
	if claimed {
		return
	}

	// Проверяем подписку на канал
	member, err := b.isChannelMember(userID)
	if err != nil {
		log.Printf("checkAndGrantChannelTrial isChannelMember error: %v", err)
		return
	}
	if !member {
		return
	}

	// Выдаём пробный запрос
	if err := b.userService.AddExtraQuota(userID, models.QuotaCategoryImage, 1); err != nil {
		log.Printf("checkAndGrantChannelTrial AddExtraQuota error: %v", err)
		return
	}

	// Помечаем, что пробный запрос выдан
	if err := b.userService.MarkChannelTrialClaimed(userID); err != nil {
		log.Printf("checkAndGrantChannelTrial MarkChannelTrialClaimed error: %v", err)
	}

	log.Printf("Granted channel trial to user %d", userID)
}

// startTrialReminderScheduler запускает планировщик для отправки напоминаний
// пользователям, которые не получили бесплатную генерацию через час после регистрации
func (b *Bot) startTrialReminderScheduler() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if b.shuttingDown.Load() {
			return
		}
		b.sendTrialReminders()
	}
}

// sendTrialReminders отправляет напоминания пользователям о бесплатной генерации
func (b *Bot) sendTrialReminders() {
	users, err := b.userService.GetUsersForTrialReminder()
	if err != nil {
		log.Printf("GetUsersForTrialReminder error: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	log.Printf("Sending trial reminders to %d users", len(users))

	for _, user := range users {
		if user == nil || user.TelegramID == 0 {
			continue
		}

		text := `<b>Ты так и не попробовал первую бесплатную генерацию</b> 🎁

Самое крутое — тебе нужно всего 1 фото, а результат будет как после настоящей фотосессии 📸✨

Попробуй прямо сейчас и посмотри, каким классным может быть твой снимок!

Подписывайся на канал, выбирай образ, выбирай модель Nano Banana и загрузи фото — это бесплатно 👇
https://t.me/aifaceapps`

		kb := newInlineKeyboardMarkup(
			newInlineKeyboardRow(
				newInlineKeyboardButtonURL("Выбрать образ", "https://t.me/aifaceapps"),
			),
		)

		msg := newMessageConfig(user.TelegramID, text)
		msg.ReplyMarkup = kb
		msg.DisableWebPagePreview = true
		msg.ParseMode = "HTML"

		if _, err := b.sendMsg(msg); err != nil {
			log.Printf("Failed to send trial reminder to %d: %v", user.TelegramID, err)
		} else {
			log.Printf("Sent trial reminder to user %d", user.TelegramID)
		}

		// Помечаем, что напоминание отправлено
		if err := b.userService.MarkTrialReminderSent(user.TelegramID); err != nil {
			log.Printf("MarkTrialReminderSent error for %d: %v", user.TelegramID, err)
		}

		// Небольшая задержка между отправками
		time.Sleep(50 * time.Millisecond)
	}
}

// processGeneration обрабатывает генерацию изображения
func (b *Bot) processGeneration(chatID int64, userID int64, photoURLs []string, genType string, prompt string) {
	if !b.ensureCategoryEnabled(chatID, ModelCategoryPhoto) {
		return
	}

	if len(photoURLs) == 0 {
		// nano-banana-2 supports text-only generation
		modelIDCheck := b.getUserModel(userID)
		if modelIDCheck != "nano-banana-2" {
			b.sendErrorMessage(chatID, "Не получено ни одного изображения")
			return
		}
	}
	if len(photoURLs) > 4 {
		photoURLs = photoURLs[:4]
	}

	modelID := b.getUserModel(userID)
	modelOpt, ok := findModelOption(modelID)
	if !ok {
		// Фолбек к первой фото-модели
		for _, m := range modelOptions {
			if m.Category == ModelCategoryPhoto {
				modelOpt = m
				ok = true
				break
			}
		}
	}

	if modelOpt.ID == "" {
		modelOpt = ModelOption{
			ID:          modelID,
			Category:    ModelCategoryPhoto,
			RequestCost: 1,
		}
	}

	if modelOpt.AdminOnly && !b.isUserAllowed(userID) {
		b.sendErrorMessage(chatID, "Доступ к этой модели только для админов")
		return
	}

	// Запрещаем запуск фото-генерации с не-фото моделью
	if ok && modelOpt.Category != ModelCategoryPhoto {
		b.sendErrorMessage(chatID, fmt.Sprintf("Текущая модель относится к категории %s.\nДля редактирования фото выберите модель из категории «Фото» в меню моделей.", categoryLabel(modelOpt.Category)))
		return
	}

	requestCost := modelOpt.RequestCost
	if requestCost < 1 {
		requestCost = 1
	}
	// Dynamic cost for pro and nano-banana-2 based on resolution
	if modelOpt.ID == "google/nano-banana-pro" || modelOpt.ID == "nano-banana-2" {
		requestCost = b.getPhotoResolutionCost(userID, modelOpt.ID)
	}

	// Списываем запросы согласно выбранной модели
	primaryUsed, extraUsed, err := b.userService.ConsumeQuotaDetailed(userID, models.QuotaCategoryImage, requestCost)
	if err != nil {
		// 1 пробная генерация фото за подписку на канал (только 1 раз)
		// Выдача происходит только по кнопке "Проверить подписку" (trial:check)
		if requestCost == 1 {
			claimed, cErr := b.userService.HasClaimedChannelTrial(userID)
			if cErr == nil && !claimed {
				member, mErr := b.isChannelMember(userID)
				if mErr != nil {
					log.Printf("channel member check error: %v", mErr)
					b.sendErrorMessage(chatID, "Не удалось проверить подписку. Попробуйте ещё раз.")
					return
				}
				if !member {
					b.sendChannelTrialMenu(chatID)
					return
				}
				b.sendText(chatID, "Чтобы получить 1 пробную генерацию, нажмите «Проверить подписку» в /start")
			}
		}
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryImage, requestCost, err)
		return
	}

	// Используем photoURL как base64 (в реальной реализации нужно скачать и конвертировать)
	imageList := photoURLs

	// Запускаем генерацию через сервис
	userRec, _ := b.userService.GetUserByTelegramID(userID)
	username := ""
	if userRec != nil {
		username = userRec.Username
	}
	inputImage := ""
	if len(imageList) > 0 {
		inputImage = imageList[0]
	}
	opts := services.GenerationOptions{
		InputImages:       imageList,
		InputImage:        inputImage,
		Prompt:            prompt,
		TokensCost:        requestCost,
		TokensPrimaryUsed: primaryUsed,
		TokensExtraUsed:   extraUsed,
		ChatID:            chatID,
		Model:             modelOpt.ID,
		ModelType:         string(modelOpt.Category),
		Username:          username,
	}
	if modelOpt.ID == "google/nano-banana" || modelOpt.ID == "google/nano-banana-pro" || modelOpt.ID == "nano-banana-2" || modelOpt.ID == "seedream/4.5-edit" {
		opts.AspectRatio = b.getAspectRatioForModel(userID, modelOpt.ID)
	}
	if modelOpt.ID == "google/nano-banana" || modelOpt.ID == "google/nano-banana-pro" || modelOpt.ID == "nano-banana-2" || modelOpt.ID == "seedream/4.5-edit" {
		if provider, err := b.userService.GetNanoBananaProvider(); err == nil {
			opts.NanoBananaProvider = provider
		}
		opts.UseDefAPI = false
	} else {
		if useDef, err := b.userService.IsNanoBananaDefAPIEnabled(); err == nil {
			opts.UseDefAPI = useDef
		}
	}
	// Pass photo resolution for pro and nano-banana-2
	if modelOpt.ID == "google/nano-banana-pro" || modelOpt.ID == "nano-banana-2" {
		opts.PhotoResolution = b.getUserPhotoResolution(userID)
	}
	// Pass google search for nano-banana-2
	if modelOpt.ID == "nano-banana-2" {
		opts.GoogleSearch = b.getUserGoogleSearch(userID)
	}

	req, err := b.generationService.StartGeneration(userID, opts)
	if err != nil {
		// Возвращаем запросы в исходные бакеты при ошибке запуска
		_ = b.userService.RefundQuota(userID, models.QuotaCategoryImage, primaryUsed, extraUsed)
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка запуска генерации: %v", err))
		return
	}

	// Отправляем сообщение о начале генерации
	text := fmt.Sprintf("🔄 Генерация запущена! ID запроса: %d\n\nОжидайте результат...", req.ID)
	msg := newMessageConfig(chatID, text)
	_, err = b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send generation start message: %v", err)
	}
}

func (b *Bot) confirmGeneration(chatID int64, userID int64, requestIDStr string) {
	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		b.sendErrorMessage(chatID, "Неверный ID запроса")
		return
	}

	// Получаем запрос на генерацию
	req, err := b.generationService.GetGenerationRequest(requestID)
	if err != nil {
		b.sendErrorMessage(chatID, "Запрос не найден")
		return
	}

	if req.UserID != userID {
		b.sendErrorMessage(chatID, "Доступ запрещен")
		return
	}

	// Отправляем статус генерации
	b.sendGenerationStatus(chatID, req)
}

func (b *Bot) handleTextMessage(msg *tgmodels.Message) {
	userText := strings.TrimSpace(msg.Text)
	loc := b.getLocalization(msg.From.ID)
	ruLoc := GetLocalization("ru")
	enLoc := GetLocalization("en")
	modelID := b.getUserModel(msg.From.ID)
	modelOpt, ok := findModelOption(modelID)

	// Обрабатываем нажатия кнопок реплай-меню
	switch userText {
	case loc.MenuGenPhotoBtn, ruLoc.MenuGenPhotoBtn, enLoc.MenuGenPhotoBtn:
		if !b.ensureCategoryEnabled(msg.Chat.ID, ModelCategoryPhoto) {
			return
		}
		b.setUserModel(msg.From.ID, "google/nano-banana")
		b.sendModelMenu(msg.Chat.ID, msg.From.ID, ModelCategoryPhoto, 0)
		return
	case loc.MenuGenMusicBtn, ruLoc.MenuGenMusicBtn, enLoc.MenuGenMusicBtn:
		if !b.ensureCategoryEnabled(msg.Chat.ID, ModelCategoryMusic) {
			return
		}
		b.setUserModel(msg.From.ID, "music-suno")
		b.sendModelMenu(msg.Chat.ID, msg.From.ID, ModelCategoryMusic, 0)
		return
	case loc.MenuGenVideoBtn, ruLoc.MenuGenVideoBtn, enLoc.MenuGenVideoBtn:
		if !b.ensureCategoryEnabled(msg.Chat.ID, ModelCategoryVideo) {
			return
		}
		b.setUserModel(msg.From.ID, "kling-2.6/image-to-video")
		b.sendModelMenu(msg.Chat.ID, msg.From.ID, ModelCategoryVideo, 0)
		return
	case loc.MenuInviteFriendBtn, ruLoc.MenuInviteFriendBtn, enLoc.MenuInviteFriendBtn, "👥 Пригласить друга":
		b.sendInviteInfo(msg.Chat.ID, msg.From.ID)
		return
	case loc.MenuBuyBtn, ruLoc.MenuBuyBtn, enLoc.MenuBuyBtn:
		if !b.ensurePaymentsEnabled(msg.Chat.ID) {
			return
		}
		b.sendBuyMenu(msg.Chat.ID, msg.From.ID)
		return
	case loc.MenuAccountBtn, ruLoc.MenuAccountBtn, enLoc.MenuAccountBtn:
		b.sendAccount(msg.Chat.ID, msg.From.ID)
		return
	case loc.MenuSettingsBtn, ruLoc.MenuSettingsBtn, enLoc.MenuSettingsBtn:
		b.sendSettingsMenu(msg.Chat.ID, msg.From.ID)
		return
	case loc.MenuHelpBtn, ruLoc.MenuHelpBtn, enLoc.MenuHelpBtn:
		b.sendHelpMessage(msg.Chat.ID)
		return
	}

	// nano-banana-2: поддержка генерации только по тексту (без фото)
	if ok && modelOpt.ID == "nano-banana-2" {
		b.processGeneration(msg.Chat.ID, msg.From.ID, []string{}, "text_only", userText)
		return
	}

	// Аудио-модель: озвучиваем текст сразу
	if ok && modelOpt.Category == ModelCategoryMusic {
		b.processAudioMessage(msg, modelOpt)
		return
	}

	// Чат-модель: выдаём ответ с учётом системного промпта (пока локально)
	if ok && modelOpt.Category == ModelCategoryChat {
		// Сохраняем сообщение в контекст (до 5 последних) только для текстовых моделей
		b.saveMessageToContext(msg.From.ID, loc.UserPrefix+" "+userText)
		if !b.isChatModelAllowed(msg.From.ID, modelOpt) {
			b.sendErrorMessage(msg.Chat.ID, b.chatModelAccessMessage(modelOpt.ID))
			return
		}
		requestCost := modelOpt.RequestCost
		if requestCost < 1 {
			requestCost = 1
		}

		if !b.ensureCategoryEnabled(msg.Chat.ID, ModelCategoryChat) {
			return
		}

		primaryUsed, extraUsed, err := b.userService.ConsumeQuotaDetailed(msg.From.ID, models.QuotaCategoryText, requestCost)
		if err != nil {
			b.sendInsufficientQuotaMessage(msg.Chat.ID, models.QuotaCategoryText, requestCost, err)
			return
		}

		userRec, _ := b.userService.GetUserByTelegramID(msg.From.ID)
		username := ""
		if userRec != nil {
			username = userRec.Username
		}

		apiModel := modelOpt.ApiModel
		if apiModel == "" {
			apiModel = modelOpt.ID
		}
		reply := newMessageConfig(msg.Chat.ID, loc.Thinking)
		if _, err := b.sendMsg(reply); err != nil {
			log.Printf("Failed to send typing message: %v", err)
		}

		b.goLimited(func() {
			req, logErr := b.generationService.LogRequest(msg.From.ID, services.LogRequestOptions{
				Username:          username,
				ModelType:         string(modelOpt.Category),
				Model:             apiModel,
				Prompt:            userText,
				Status:            "processing",
				TokensUsed:        requestCost,
				TokensPrimaryUsed: primaryUsed,
				TokensExtraUsed:   extraUsed,
			})
			if logErr != nil {
				log.Printf("LogRequest(chat processing) error: %v", logErr)
			}
			requestID := int64(0)
			if req != nil {
				requestID = req.ID
			}
			systemPrompt := b.buildChatSystemPrompt(msg.From.ID)
			messages := []map[string]string{
				{"role": "system", "content": systemPrompt},
			}
			// добавляем контекст пользователя из redis с явным префиксом как системный блок
			if ctx, err := b.redisClient.GetContext(msg.From.ID); err == nil && ctx != nil && len(ctx.Messages) > 0 {
				contextBlock := loc.ContextPrefix + strings.Join(ctx.Messages, "\n")
				messages = append(messages, map[string]string{"role": "system", "content": contextBlock})
			}
			// текущий запрос
			messages = append(messages, map[string]string{"role": "user", "content": userText})

			resp, err := b.generationService.GenerateChat(apiModel, messages)
			if err != nil {
				if requestID != 0 {
					if updErr := b.generationService.FailRequest(requestID, err.Error()); updErr != nil {
						log.Printf("FailRequest(chat) error: %v", updErr)
					}
				}
				_ = b.userService.RefundQuota(msg.From.ID, models.QuotaCategoryText, primaryUsed, extraUsed)
				if requestID != 0 {
					if resetErr := b.generationService.ResetRequestTokensUsed(requestID); resetErr != nil {
						log.Printf("ResetRequestTokensUsed(chat) error: %v", resetErr)
					}
				}
				b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Не удалось ответить: %s", friendlyGenerationError(err)))
				return
			}
			if requestID != 0 {
				if updErr := b.generationService.CompleteRequest(requestID, resp); updErr != nil {
					log.Printf("CompleteRequest(chat) error: %v", updErr)
				}
			}
			b.saveMessageToContext(msg.From.ID, loc.BotPrefix+" "+resp)
			b.sendLongText(msg.Chat.ID, resp)
		})
		return
	}

	// Для всех остальных категорий показываем инструкцию конкретной модели
	instruction := instructionForModel(modelOpt)
	reply := newMessageConfig(msg.Chat.ID, instruction)
	reply.ParseMode = "HTML"
	_, _ = b.sendMsg(reply)
}

// saveMessageToContext добавляет сообщение в контекст пользователя (максимум 5)
func (b *Bot) saveMessageToContext(userID int64, message string) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return
	}
	// Пытаемся добавить; если контекста нет — создаём пустой и пробуем снова
	if err := b.redisClient.AddMessage(userID, msg); err != nil {
		if err := b.redisClient.SavePhotoURL(userID, ""); err != nil {
			log.Printf("saveMessageToContext: failed to init context: %v", err)
			return
		}
		if err := b.redisClient.AddMessage(userID, msg); err != nil {
			log.Printf("saveMessageToContext: failed to add message: %v", err)
		}
	}
}

func (b *Bot) startPromoText(loc *Localization) string {
	if b.redisClient == nil {
		return ""
	}
	discount, err := b.redisClient.GetPhotoDiscount()
	if err != nil || discount == nil || discount.Percent <= 0 {
		return ""
	}
	endTime := time.Unix(discount.EndTime, 0)
	left := time.Until(endTime)
	if left <= 0 {
		return ""
	}
	days := int(left.Hours()) / 24
	hours := int(left.Hours()) % 24
	minutes := int(left.Minutes()) % 60
	seconds := int(left.Seconds()) % 60

	return fmt.Sprintf(loc.StartPromoTitle, discount.Percent) + "\n" + fmt.Sprintf(loc.StartPromoCountdown, days, hours, minutes, seconds)
}

// handleStart обрабатывает команду /start
func (b *Bot) handleStart(msg *tgmodels.Message) {
	if isAdmin, err := b.userService.IsUserAdmin(msg.From.ID); err == nil {
		b.setChatCommands(msg.Chat.ID, isAdmin)
	}
	loc := b.getLocalization(msg.From.ID)
	text := loc.WelcomeText
	if promo := b.startPromoText(loc); promo != "" {
		text += "\n\n" + promo
	}

	// Отправляем приветствие с кнопкой меню
	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyledWithEmoji(loc.MenuBtn, "menu", "success", EmojiIDMenu),
		),
	)
	reply := newMessageConfig(msg.Chat.ID, text)
	reply.ReplyMarkup = keyboard
	if _, err := b.sendMsg(reply); err != nil {
		log.Printf("Failed to send start message: %v", err)
	}

	claimed, err := b.userService.HasClaimedChannelTrial(msg.From.ID)
	if err != nil {
		log.Printf("trial claimed check error: %v", err)
		b.sendStartTrialMenu(msg.Chat.ID)
		return
	}
	if !claimed {
		b.sendStartTrialMenu(msg.Chat.ID)
	}
}

// handleStartWithReferral обрабатывает /start с реферальным кодом
func (b *Bot) handleStartWithReferral(msg *tgmodels.Message, referralCode string) {
	user := msg.From
	_, err := b.userService.GetOrCreateUserWithReferrer(
		user.ID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
		referralCode,
	)
	if err != nil {
		log.Printf("Failed to create user with referral: %v", err)
	}

	if isAdmin, err := b.userService.IsUserAdmin(user.ID); err == nil {
		b.setChatCommands(msg.Chat.ID, isAdmin)
	}
	loc := b.getLocalization(user.ID)
	text := loc.WelcomeText
	if promo := b.startPromoText(loc); promo != "" {
		text += "\n\n" + promo
	}
	b.sendText(msg.Chat.ID, text)
	claimed, err := b.userService.HasClaimedChannelTrial(user.ID)
	if err != nil {
		log.Printf("trial claimed check error: %v", err)
		b.sendStartTrialMenu(msg.Chat.ID)
		return
	}
	if !claimed {
		b.sendStartTrialMenu(msg.Chat.ID)
	}
}

// sendMainMenu отправляет главное меню
func (b *Bot) sendMainMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	// Получаем статистику пользователя
	if _, err := b.userService.GetUserStats(userID); err != nil {
		log.Printf("Failed to get user stats: %v", err)
	}

	user, _ := b.userService.GetUserByTelegramID(userID)
	quota, _ := b.userService.GetQuota(userID)

	subLabel, subActive := b.userService.GetSubscriptionLabel(userID)
	if !subActive && user != nil && user.IsPremium {
		subLabel = "Premium"
	}

	currentModel := b.getUserModel(userID)
	displayCategory := "" // не показываем категорию в строке модели, чтобы не путать с названием модели
	var limitLine = loc.MenuLimit
	if opt, ok := findModelOption(currentModel); ok {
		var base, extra int
		if quota != nil {
			switch opt.Category {
			case ModelCategoryPhoto:
				base, extra = quota.ImageWeekly, quota.ImageExtra
			case ModelCategoryVideo:
				base, extra = quota.VideoWeekly, quota.VideoExtra
			case ModelCategoryMusic:
				base, extra = quota.MusicWeekly, quota.MusicExtra
			case ModelCategoryChat:
				base, extra = quota.TextDaily, quota.TextExtra
			}
		}
		limitLine = fmt.Sprintf(loc.MenuLimitFormat, base, extra)
	}

	modelLabel := strings.TrimSpace(displayCategory + " " + b.modelLabelLoc(currentModel, loc))
	htmlText := fmt.Sprintf(
		loc.MenuMainHTML,
		html.EscapeString(fmt.Sprintf("%d", userID)),
		html.EscapeString(subLabel),
		html.EscapeString(modelLabel),
		html.EscapeString(limitLine),
	)

	// reply-клавиатура
	replyKB := newReplyKeyboard(
		newKeyboardButtonRow(
			newKeyboardButtonEmoji(loc.MenuGenPhotoBtn, "5235837920081887219"),
			newKeyboardButtonEmoji(loc.MenuGenMusicBtn, "5463107823946717464"),
		),
		newKeyboardButtonRow(
			newKeyboardButton(loc.MenuGenVideoBtn),
			newKeyboardButtonEmoji(loc.MenuBuyBtn, "5224257782013769471"),
		),
		newKeyboardButtonRow(
			newKeyboardButton(loc.MenuInviteFriendBtn),
		),
		newKeyboardButtonRow(
			newKeyboardButtonEmoji(loc.MenuAccountBtn, "5231200819986047254"),
			newKeyboardButtonEmoji(loc.MenuSettingsBtn, "5341715473882955310"),
		),
		newKeyboardButtonRow(
			newKeyboardButton(loc.MenuHelpBtn),
		),
	)
	replyKB.ResizeKeyboard = true

	// инлайн-кнопки (без обычных эмодзи - используем только custom emoji)
	inlineKB := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyledWithEmoji("Покупка", "buy", "success", EmojiIDBuy),
			newInlineKeyboardButtonDataStyledWithEmoji("Пригласить друга", "invite", "success", EmojiIDGift),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyledWithEmoji(loc.MenuSelectModelBtn, "models_menu", "success", EmojiIDSelectModel),
		),
	)

	// Админ-кнопка только для админов
	if isAdmin, err := b.userService.IsUserAdmin(userID); err == nil && isAdmin {
		b.setChatCommands(chatID, true)
	}

	// Сообщение для установки reply-клавиатуры (краткий текст)
	reply := newMessageConfig(chatID, loc.MenuBtn)
	reply.ReplyMarkup = replyKB
	reply.DisableNotification = true
	if _, err := b.sendMsg(reply); err != nil {
		log.Printf("Failed to send main menu reply keyboard: %v", err)
	}

	// Сообщение с основным текстом и инлайн-кнопками
	inlineMsg := newMessageConfig(chatID, htmlText)
	inlineMsg.ParseMode = "HTML"
	inlineMsg.ReplyMarkup = inlineKB
	if _, err := b.sendMsg(inlineMsg); err != nil {
		log.Printf("Failed to send inline main menu: %v", err)
	}
}

// sendAccount отправляет карточку аккаунта с лимитами (как на скриншоте)
func (b *Bot) sendAccount(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	user, _ := b.userService.GetUserByTelegramID(userID)
	quota, _ := b.userService.GetQuota(userID)

	subType := "Free"
	var subEndStr = "-"
	if lbl, ok, subEnd := b.userService.GetSubscriptionInfo(userID); ok {
		subType = lbl
		if subEnd != nil {
			msk := time.FixedZone("MSK", 3*3600)
			subEndStr = subEnd.In(msk).Format("02.01.2006 15:04")
		}
	} else if user != nil && user.IsPremium {
		subType = "Premium"
	}

	var (
		textDaily, imageWeekly, musicWeekly, videoWeekly int
		textExtra, imageExtra, musicExtra, videoExtra    int
	)
	if quota != nil {
		textDaily = quota.TextDaily
		imageWeekly = quota.ImageWeekly
		musicWeekly = quota.MusicWeekly
		videoWeekly = quota.VideoWeekly
		textExtra = quota.TextExtra
		imageExtra = quota.ImageExtra
		musicExtra = quota.MusicExtra
		videoExtra = quota.VideoExtra
	}

	accountText := fmt.Sprintf(
		loc.AccountCardTemplate,
		html.EscapeString(fmt.Sprintf("%d", userID)),
		html.EscapeString(subType),
		html.EscapeString(subEndStr),
		html.EscapeString(fmt.Sprintf("%d", textDaily)),
		html.EscapeString(fmt.Sprintf("%d", imageWeekly)),
		html.EscapeString(fmt.Sprintf("%d", musicWeekly)),
		html.EscapeString(fmt.Sprintf("%d", videoWeekly)),
		html.EscapeString(fmt.Sprintf("%d", textExtra)),
		html.EscapeString(fmt.Sprintf("%d", imageExtra)),
		html.EscapeString(fmt.Sprintf("%d", musicExtra)),
		html.EscapeString(fmt.Sprintf("%d", videoExtra)),
	)

	// Добавляем кнопку покупки
	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyledWithEmoji("Покупка", "buy", "success", EmojiIDBuy),
		),
	)

	msg := newMessageConfig(chatID, accountText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send account card: %v", err)
	}
}

// sendBuyMenu отправляет меню покупки доп. запросов
func (b *Bot) sendBuyMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}

	subsEnabled, _ := b.userService.IsSubscriptionsEnabled()

	text := loc.BuyTitle + "\n\n" + loc.BuySelectAction + "\n" + loc.BuyConsentNote

	rows := [][]tgmodels.InlineKeyboardButton{}
	row := []tgmodels.InlineKeyboardButton{}
	if subsEnabled {
		row = append(row, newInlineKeyboardButtonDataStyledWithEmoji(loc.BuySubscriptionBtn, "buy:sub", "success", "5215420556089776398"))
	}
	row = append(row, newInlineKeyboardButtonDataStyledWithEmoji(loc.BuyExtrasBtn, "buy:extras", "success", EmojiIDBuy))
	rows = append(rows, newInlineKeyboardRow(row...))
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn+" "+loc.MenuBtn, "menu"),
	))

	keyboard := newInlineKeyboardMarkup(rows...)

	reply := newMessageConfig(chatID, text)
	reply.ReplyMarkup = keyboard

	_, err := b.sendMsg(reply)
	if err != nil {
		log.Printf("Failed to send buy menu: %v", err)
	}
}

// sendBuyExtrasMenu отправляет меню покупки доп. запросов по категориям
func (b *Bot) sendBuyExtrasMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}

	enabled := b.getEnabledCategories(false)
	if len(enabled) == 0 {
		b.sendErrorMessage(chatID, loc.ErrAllCategoriesDisabled)
		return
	}

	rows := [][]tgmodels.InlineKeyboardButton{}
	for _, cat := range enabled {
		switch cat {
		case ModelCategoryChat:
			rows = append(rows, newInlineKeyboardRow(newInlineKeyboardButtonDataStyledWithEmoji(loc.ExtrasTexts, "buy:text", "success", "5429273196870254885")))
		case ModelCategoryPhoto:
			rows = append(rows, newInlineKeyboardRow(newInlineKeyboardButtonDataStyledWithEmoji(loc.ExtrasImage, "buy:image", "success", "5429200581858182504")))
		case ModelCategoryMusic:
			rows = append(rows, newInlineKeyboardRow(newInlineKeyboardButtonDataStyledWithEmoji(loc.ExtrasMusico, "buy:music", "success", "5429372861586359061")))
		case ModelCategoryVideo:
			rows = append(rows, newInlineKeyboardRow(newInlineKeyboardButtonDataStyled(loc.ExtrasVideos, "buy:video", "success")))
		}
	}
	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn, "buy"),
	))

	text := loc.ExtrasTitle + "\n\n" + loc.ExtrasSelectCat + "\n" + loc.BuyConsentNote

	keyboard := newInlineKeyboardMarkup(rows...)

	reply := newMessageConfig(chatID, text)
	reply.ReplyMarkup = keyboard

	if _, err := b.sendMsg(reply); err != nil {
		log.Printf("Failed to send buy extras menu: %v", err)
	}
}

func (b *Bot) sendBuySubscription(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}
	if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
		b.sendErrorMessage(chatID, loc.SubsUnavailable)
		return
	}

	miniPrice := b.subscriptionPrice("mini")
	startPrice := b.subscriptionPrice("start")
	proPrice := b.subscriptionPrice("pro")

	miniImg, miniMus, _ := b.userService.SubscriptionQuotas("mini")
	startImg, startMus, startVid := b.userService.SubscriptionQuotas("start")
	proImg, proMus, proVid := b.userService.SubscriptionQuotas("pro")

	text := fmt.Sprintf("%s\n%s \n\n✨ Mini — %d ₽ %s\n• 50 %s\n• %d %s\n• %d %s\n• %s\n• %s\n\n🚀 Start — %d ₽ %s\n• 100 %s\n• %d %s\n• %d %s\n• %d %s\n• %s\n• %s\n• %s\n\n👑 Pro — %d ₽ %s\n• 200 %s\n• %d %s\n• %d %s\n• %d %s\n• %s\n• %s, %s, %s\n• %s",
		loc.SubsTitle,
		loc.BuyConsentNote,
		miniPrice, loc.SubsPerWeek, loc.SubsTextDaily, miniImg, loc.SubsImages, miniMus, loc.SubsSongs, loc.SubsTextModelsMini, fmt.Sprintf(loc.SubsDiscount, 10),
		startPrice, loc.SubsPerWeek, loc.SubsTextDaily, startImg, loc.SubsImages, startVid, loc.SubsVideos, startMus, loc.SubsSongs, fmt.Sprintf(loc.SubsContext, 2), loc.SubsTextModelsHi, fmt.Sprintf(loc.SubsDiscount, 15),
		proPrice, loc.SubsPerWeek, loc.SubsTextDaily, proImg, loc.SubsImages, proVid, loc.SubsVideos, proMus, loc.SubsSongs, loc.SubsTextModelsHi, fmt.Sprintf(loc.SubsChatStyles, 6), fmt.Sprintf(loc.SubsContext, 3), loc.SubsNoAds, fmt.Sprintf(loc.SubsDiscount, 20),
	)

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyled("⭐ Mini", "buy_sub:mini", "success"),
			newInlineKeyboardButtonDataStyled("🚀 Start", "buy_sub:start", "success"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyled("👑 Pro", "buy_sub:pro", "success"),
		),
		newInlineKeyboardRow(
			newBackButton(loc.BackBtn, "buy"),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send buy subscription: %v", err)
	}
}

func (b *Bot) subscriptionPrice(plan string) int {
	if b.paymentService == nil {
		return 0
	}
	if price, ok := b.paymentService.SubscriptionPrice(plan); ok {
		return price
	}
	return 0
}

func (b *Bot) sendBuySubscriptionPayment(chatID int64, userID int64, plan string) {
	loc := b.getLocalization(userID)
	if b.paymentService == nil {
		b.sendErrorMessage(chatID, loc.ErrPaymentNotConfigured)
		return
	}
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}
	if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
		b.sendErrorMessage(chatID, loc.SubsUnavailable)
		return
	}

	resp, err := b.paymentService.CreateSubscriptionPayment(userID, plan, 7)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf(loc.ErrCreatePayment+": %v", err))
		return
	}

	label := strings.Title(plan)
	text := fmt.Sprintf(loc.SubsPaymentCreated, label, resp.CheckoutURL) + "\n\n" + loc.BuyConsentNote
	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData(loc.SubsBackToSubs, "buy:sub"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData(loc.MenuBtn, "menu"),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send buy subscription payment: %v", err)
	}
}

// sendInviteInfo отправляет информацию о реферальной программе
func (b *Bot) sendInviteInfo(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	user, err := b.userService.GetUserByTelegramID(userID)
	if err != nil {
		b.sendErrorMessage(chatID, loc.ErrGetStats)
		return
	}

	botInfo, err := b.api.GetMe(b.ctx)
	if err != nil {
		log.Printf("Failed to get bot info: %v", err)
		b.sendErrorMessage(chatID, loc.ErrGetStats)
		return
	}
	botUsername := botInfo.Username
	referralLink := fmt.Sprintf("`https://t.me/%s?start=%s`", botUsername, user.ReferralCode)

	text := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s\n\n%s %d",
		loc.InviteTitle,
		loc.InviteExample,
		loc.InviteLink,
		referralLink,
		loc.InviteCopyHint,
		loc.InviteCount,
		user.ReferralsCount,
	)

	// Кнопка вставляет ссылку в строку ввода (можно быстро скопировать или отправить)
	copyQuery := referralLink
	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			tgmodels.InlineKeyboardButton{Text: loc.InviteCopyHint, SwitchInlineQueryCurrentChat: copyQuery, IconCustomEmojiID: "5203996991054432397"},
		),
		newInlineKeyboardRow(
			newBackButton(loc.BackBtn+" "+loc.MenuBtn, "menu"),
		),
	)

	reply := newMessageConfig(chatID, text)
	reply.ReplyMarkup = keyboard
	reply.ParseMode = "Markdown"

	_, err = b.sendMsg(reply)
	if err != nil {
		log.Printf("Failed to send invite info: %v", err)
	}
}

// sendModelMenu показывает выбор модели с разбивкой по категориям
func (b *Bot) sendModelMenu(chatID int64, userID int64, category ModelCategory, messageID int) {
	loc := b.getLocalization(userID)
	enabledCats := b.getEnabledCategories(false)
	if len(enabledCats) == 0 {
		b.sendErrorMessage(chatID, loc.ErrAllCategoriesDisabled)
		return
	}
	if category == "" || !containsCategory(enabledCats, category) {
		category = enabledCats[0]
	}

	current := b.getUserModel(userID)
	// Если текущая модель относится к отключенной категории — сбрасываем на первую доступную
	if opt, ok := findModelOption(current); ok {
		if !containsCategory(enabledCats, opt.Category) {
			current = ""
		}
	} else {
		current = ""
	}
	if current == "" {
		for _, m := range modelOptions {
			if containsCategory(enabledCats, m.Category) {
				if !b.isModelVisibleToUser(userID, m) {
					continue
				}
				if m.Category == ModelCategoryChat && !b.isChatModelAllowed(userID, m) {
					continue
				}
				current = m.ID
				break
			}
		}
	}
	if current != "" {
		if opt, ok := findModelOption(current); ok {
			if opt.Category == ModelCategoryChat && !b.isChatModelAllowed(userID, opt) {
				current = ""
			}
		}
	}

	rows := [][]tgmodels.InlineKeyboardButton{}

	// Ряд с категориями (только включённые) - синий стиль
	catButtons := []tgmodels.InlineKeyboardButton{}
	for _, cat := range enabledCats {
		label := b.categoryLabelLoc(cat, loc)
		isSelected := cat == category
		if isSelected {
			label = "✔️ " + label
		}
		// Custom emoji for photo category
		if cat == ModelCategoryPhoto {
			catButtons = append(catButtons, newInlineKeyboardButtonDataStyledWithEmoji(label, "models_menu:"+string(cat), ButtonStylePrimary, EmojiIDPhotoCategory))
		} else {
			catButtons = append(catButtons, newInlineKeyboardButtonDataStyled(label, "models_menu:"+string(cat), ButtonStylePrimary))
		}
	}
	rows = append(rows, catButtons)

	// Ряды с моделями выбранной категории - зелёный стиль
	options := modelOptionsByCategory(category)
	row := []tgmodels.InlineKeyboardButton{}
	for i, m := range options {
		if !b.isModelVisibleToUser(userID, m) {
			continue
		}
		label := m.Label
		if m.Category == ModelCategoryChat && !b.isChatModelAllowed(userID, m) {
			label = "🔒 " + label
		}
		isSelected := m.ID == current
		if isSelected {
			label = "✔️ " + label
		}
		// Use custom emoji if available
		if m.EmojiID != "" {
			row = append(row, newInlineKeyboardButtonDataStyledWithEmoji(label, "model_set:"+m.ID, ButtonStyleSuccess, m.EmojiID))
		} else {
			row = append(row, newInlineKeyboardButtonDataStyled(label, "model_set:"+m.ID, ButtonStyleSuccess))
		}
		if len(row) == 2 || i == len(options)-1 {
			rows = append(rows, row)
			row = []tgmodels.InlineKeyboardButton{}
		}
	}

	rows = append(rows, newInlineKeyboardRow(
		newBackButton(loc.BackBtn, "menu"),
	))

	text := "<b>" + fmt.Sprintf(loc.ModelsCat, b.categoryLabelLoc(category, loc)) + "</b>\n\n"
	currentDesc := strings.TrimSpace(b.modelDescriptionLoc(current, loc))
	text += loc.ModelsCurrentLabel + " " + html.EscapeString(b.modelLabelLoc(current, loc))
	if currentDesc != "" {
		text += " — " + currentDesc
	}
	if current == "google/nano-banana-pro" {
		minCost := 4
		text += "\n<b>" + fmt.Sprintf(loc.ModelsCostFrom, minCost) + "</b>"
	} else if current == "nano-banana-2" {
		minCost := 2
		text += "\n<b>" + fmt.Sprintf(loc.ModelsCostFrom, minCost) + "</b>"
	} else if current == "wan/2-6-image-to-video" {
		minCost := 2
		text += "\n<b>" + fmt.Sprintf(loc.ModelsCostFrom, minCost) + "</b>"
	} else if current == "kling-2.6/image-to-video" {
		minCost := 1
		text += "\n<b>" + fmt.Sprintf(loc.ModelsCostFromSingular, minCost) + "</b>"
	} else if cost := modelRequestCost(current); cost > 0 {
		text += "\n<b>Расход:</b> " + fmt.Sprintf("%d %s", cost, requestWord(cost, loc))
	}
	if current == "google/nano-banana" {
		text += "\nМаксимум фото: 2\n"
	} else if current == "google/nano-banana-pro" || current == "seedream/4.5-edit" || current == "nano-banana-2" {
		text += "\nМаксимум фото: 4\n"
	}
	if instr := b.modelInstructionLoc(current, loc); instr != "" {
		text += "\n" + instr
	}
	if category == ModelCategoryMusic {
		if opt, ok := findModelOption(current); ok && (opt.ID == "music-suno" || strings.Contains(strings.ToLower(opt.ApiModel), "suno")) {
			instrOn := b.getSunoInstrumental(userID)
			instrBtn := "🎹 " + loc.MusicMode + " " + loc.MusicModeVocal
			if instrOn {
				instrBtn = "🎹 " + loc.MusicMode + " " + loc.MusicModeInstr
			}
			rows = append(rows, newInlineKeyboardRow(
				newInlineKeyboardButtonData(instrBtn, "suno_instr_toggle"),
			))
			if !instrOn {
				voice := b.getSunoVoice(userID)
				voiceBtn := "🗣️ " + loc.MusicVoice + " " + loc.MusicVoiceMale
				if voice == "f" {
					voiceBtn = "🗣️ " + loc.MusicVoice + " " + loc.MusicVoiceFemale
				}
				rows = append(rows, newInlineKeyboardRow(
					newInlineKeyboardButtonData(voiceBtn, "suno_voice_toggle"),
				))
			}
		}
	}
	if (category == ModelCategoryPhoto && (current == "google/nano-banana" || current == "google/nano-banana-pro" || current == "nano-banana-2" || current == "kie/nano-banana-edit" || current == "kie/nano-banana-pro" || current == "seedream/4.5-edit")) || (category == ModelCategoryVideo && current == "veo3_fast") {
		ratio := b.getAspectRatioForModel(userID, current)
		label := loc.AspectTitle + ": " + ratio
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(label, "aspect_menu"),
		))
	}
	if category == ModelCategoryPhoto && current == "google/nano-banana-pro" {
		resolution := b.getUserPhotoResolutionPro(userID)
		// Cycle: 2K -> 4K -> 2K (no 1K for Pro)
		nextRes := "4K"
		if resolution == "4K" {
			nextRes = "2K"
		}
		dynCost := b.getPhotoResolutionCost(userID, current)
		resLabel := fmt.Sprintf("📐 Разрешение: %s (%d ген.)", resolution, dynCost)
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(resLabel, "photo_resolution_toggle:"+nextRes),
		))
	}
	if category == ModelCategoryPhoto && current == "nano-banana-2" {
		resolution := b.getUserPhotoResolution(userID)
		// Cycle: 1K -> 2K -> 4K -> 1K
		nextRes := "2K"
		if resolution == "2K" {
			nextRes = "4K"
		} else if resolution == "4K" {
			nextRes = "1K"
		}
		dynCost := b.getPhotoResolutionCost(userID, current)
		resLabel := fmt.Sprintf("📐 Разрешение: %s (%d ген.)", resolution, dynCost)
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(resLabel, "photo_resolution_toggle:"+nextRes),
		))
	}
	if category == ModelCategoryPhoto && current == "nano-banana-2" {
		gsearch := b.getUserGoogleSearch(userID)
		gsLabel := "🔍 Google Поиск: выкл"
		nextGS := "true"
		if gsearch == "true" {
			gsLabel = "🔍 Google Поиск: вкл"
			nextGS = "false"
		}
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(gsLabel, "google_search_toggle:"+nextGS),
		))
	}
	if category == ModelCategoryVideo && current == "wan/2-6-image-to-video" {
		duration := b.getUserVideoDuration(userID)
		costMap := map[string]int{"5": 2, "10": 4, "15": 6}
		cost := costMap[duration]
		// Cycle: 5 -> 10 -> 15 -> 5
		nextDuration := "10"
		if duration == "10" {
			nextDuration = "15"
		} else if duration == "15" {
			nextDuration = "5"
		}
		label := fmt.Sprintf("⏱️ Длительность: %s сек (%d ген.)", duration, cost)
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(label, "duration_toggle:"+nextDuration),
		))
		resolution := b.getUserVideoResolution(userID)
		// Toggle: 720p <-> 1080p
		nextResolution := "1080p"
		if resolution == "1080p" {
			nextResolution = "720p"
		}
		resLabel := fmt.Sprintf("📺 Разрешение: %s", resolution)
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(resLabel, "resolution_toggle:"+nextResolution),
		))
	}
	if category == ModelCategoryVideo && current == "kling-2.6/image-to-video" {
		duration := b.getUserVideoDuration(userID)
		sound := b.getUserVideoSound(userID)
		// Kling only supports 5 and 10 seconds
		if duration != "5" && duration != "10" {
			duration = "5"
		}
		// Cycle: 5 -> 10 -> 5
		nextDuration := "10"
		if duration == "10" {
			nextDuration = "5"
		}
		label := fmt.Sprintf("⏱️ Длительность: %s сек", duration)
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(label, "duration_toggle_kling:"+nextDuration),
		))
		soundLabel := "🔇 Без звука"
		nextSound := "true"
		if sound == "true" {
			soundLabel = "🔊 Со звуком (x2)"
			nextSound = "false"
		}
		rows = append(rows, newInlineKeyboardRow(
			newInlineKeyboardButtonData(soundLabel, "sound_toggle:"+nextSound),
		))
	}
	// Показываем кнопку "Итого" только если текущая модель - видео модель
	if currentOpt, ok := findModelOption(current); ok && currentOpt.Category == ModelCategoryVideo {
		totalCost := b.getVideoTotalCost(userID, current)
		if totalCost > 0 {
			costLabel := fmt.Sprintf("💰 Итого: %d ген.", totalCost)
			rows = append(rows, newInlineKeyboardRow(
				newInlineKeyboardButtonData(costLabel, "video_total:"+current),
			))
		}
	}
	if desc := strings.TrimSpace(loc.ModelsDescription); desc != "" {
		text += "\n\n" + desc
	}

	keyboard := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageTextOrCaption(chatID, messageID, text, keyboard); err != nil {
			log.Printf("Failed to edit model menu: %v", err)
		}
		return
	}

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = "HTML"
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send model menu: %v", err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	text := `Тех поддержка: @wwqeew52`

	msg := newMessageConfig(chatID, text)
	_, err := b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

// handleEmojiCommand extracts custom_emoji_id from message entities (for admins)
func (b *Bot) handleEmojiCommand(msg *tgmodels.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Check if admin
	isAdmin, _ := b.userService.IsUserAdmin(userID)
	if !isAdmin {
		b.sendText(chatID, "Эта команда доступна только администраторам.")
		return
	}

	// Check entities for custom emoji
	var result strings.Builder
	result.WriteString("🔍 Custom Emoji IDs:\n\n")

	found := false
	if msg.Entities != nil {
		for _, entity := range msg.Entities {
			if entity.Type == "custom_emoji" && entity.CustomEmojiID != "" {
				found = true
				// Extract the emoji text
				emojiText := ""
				if entity.Offset >= 0 && entity.Offset+entity.Length <= len(msg.Text) {
					runes := []rune(msg.Text)
					if entity.Offset+entity.Length <= len(runes) {
						emojiText = string(runes[entity.Offset : entity.Offset+entity.Length])
					}
				}
				result.WriteString(fmt.Sprintf("Emoji: %s\nID: %s\n\n", emojiText, entity.CustomEmojiID))
			}
		}
	}

	if !found {
		result.WriteString("Не найдено custom emoji в сообщении.\n\n")
		result.WriteString("Отправьте сообщение с кастомными эмодзи (из Premium стикерпаков) после команды /emoji, например:\n")
		result.WriteString("/emoji 🍌\n\n")
		result.WriteString("Или ответьте на сообщение с кастомными эмодзи командой /emoji")
	}

	b.sendText(chatID, result.String())
}

func (b *Bot) sendAspectRatioMenu(chatID int64, userID int64, messageID int) {
	loc := b.getLocalization(userID)
	modelID := b.getUserModel(userID)
	modelOpt, _ := findModelOption(modelID)
	isVideo := modelOpt.Category == ModelCategoryVideo
	current := b.getAspectRatioForModel(userID, modelID)

	var rows [][]tgmodels.InlineKeyboardButton
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		aspectOptionButton(loc.AspectLandscape, "16:9", current),
		aspectOptionButton(loc.AspectPortrait, "9:16", current),
	})
	if isVideo {
		// Видео: 16:9, 9:16, Auto
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			aspectOptionButton(loc.AspectAuto, "auto", current),
		})
	} else {
		// Фото: 16:9, 9:16, 1:1
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			aspectOptionButton(loc.AspectSquare, "1:1", current),
		})
	}
	// Кнопка назад ведёт в меню моделей нужной категории
	backCallback := "models_menu:" + string(modelOpt.Category)
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newBackButton(loc.BackBtn, backCallback),
	})
	text := loc.AspectTitle + ": " + current
	markup := newInlineKeyboardMarkup(rows...)

	if previewURL := aspectPreviewURL(current); previewURL != "" {
		if messageID > 0 {
			// Edit media (photo) with new preview
			media := &tgmodels.InputMediaPhoto{
				Media:   previewURL,
				Caption: text,
			}
			if err := b.editMessageMedia(chatID, messageID, media, &markup); err != nil {
				if !strings.Contains(err.Error(), "message is not modified") {
					log.Printf("Failed to edit aspect ratio media: %v", err)
				}
			}
			return
		}
		if _, err := b.sendPhoto(chatID, &tgmodels.InputFileString{Data: previewURL}, text, "", markup); err != nil {
			log.Printf("Failed to send aspect ratio photo: %v", err)
		}
		return
	}

	if messageID > 0 {
		if err := b.editMessageText(chatID, messageID, text, &markup); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			// Если старое сообщение — фото, удаляем и отправляем новое текстовое
			if strings.Contains(errStr, "there is no text in the message") {
				_ = b.deleteMessage(chatID, messageID)
				newMsg := newMessageConfig(chatID, text)
				newMsg.ReplyMarkup = markup
				if _, err2 := b.sendMsg(newMsg); err2 != nil {
					log.Printf("Failed to send aspect ratio menu after delete: %v", err2)
				}
				return
			}
			log.Printf("Failed to edit aspect ratio menu: %v", err)
		}
		return
	}
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send aspect ratio menu: %v", err)
	}
}

func aspectOptionButton(label, ratio, current string) tgmodels.InlineKeyboardButton {
	if ratio == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "aspect_set:"+ratio)
}

func aspectPreviewURL(ratio string) string {
	switch ratio {
	case "16:9":
		return "https://imgfy.ru/oaLcdZy0wiGv5iT"
	case "1:1":
		return "https://habrastorage.org/webt/ou/xf/gu/ouxfgucpo8udtfashggbrle6_sm.png"
	case "9:16":
		return "https://habrastorage.org/webt/7x/pf/tf/7xpftfevnmgbjipjpzzxvuxsvq8.png"
	default:
		return ""
	}
}

func (b *Bot) editMessageTextOrCaption(chatID int64, messageID int, text string, markup tgmodels.InlineKeyboardMarkup) error {
	// Try editing text first
	if err := b.editMessageText(chatID, messageID, text, &markup); err == nil {
		return nil
	} else {
		errText := err.Error()
		if strings.Contains(errText, "message is not modified") {
			return nil
		}
		if strings.Contains(errText, "there is no text in the message to edit") || strings.Contains(errText, "message to edit not found") {
			// Try editing caption instead
			if errCap := b.editMessageCaption(chatID, messageID, text, &markup); errCap == nil {
				return nil
			} else {
				if strings.Contains(errCap.Error(), "message is not modified") {
					return nil
				}
				return errCap
			}
		}
		return err
	}
}

func (b *Bot) sendPrivacyMessage(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	text := loc.PrivacyPolicy + " https://telegra.ph/Politika-Konfidencialnosti-01-14-87\n\n" + loc.PrivacyTerms + " https://telegra.ph/Polzovatelskoe-soglashenie-Usloviya-EHkspluatacii-i-Obsluzhivaniya-01-14"
	msg := newMessageConfig(chatID, text)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send privacy message: %v", err)
	}
}

func (b *Bot) sendRulesMessage(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	text := loc.RulesTitle + "\n\n" + loc.RulesContent
	msg := newMessageConfig(chatID, text)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send rules message: %v", err)
	}
}

func (b *Bot) sendUserStats(chatID int64, userID int64) {
	stats, err := b.userService.GetUserStats(userID)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении статистики")
		return
	}

	user, _ := b.userService.GetUserByTelegramID(userID)

	text := fmt.Sprintf(`📊 Ваша статистика:

💰 Токенов: %v
🎨 Создано изображений: %v
👥 Приглашено друзей: %v`,
		stats["generations"],
		stats["completed_generations"],
		stats["referrals_count"],
	)

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("◀️ Меню", "menu"),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	_, err = b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send user stats: %v", err)
	}
	_ = user
}

func (b *Bot) sendInsufficientQuotaMessage(chatID int64, category models.QuotaCategory, need int, cause error) {
	var label string
	switch category {
	case models.QuotaCategoryText:
		label = "текстовых запросов"
	case models.QuotaCategoryImage:
		label = "запросов на изображения"
	case models.QuotaCategoryMusic:
		label = "музыкальные запросы"
	case models.QuotaCategoryVideo:
		label = "видео-запросов"
	default:
		label = "запросов"
	}

	text := fmt.Sprintf("❌ Недостаточно %s. Нужно %d.\n%s\n\nИспользуйте /buy чтобы докупить генерации.", label, need, friendlyGenerationError(cause))

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonDataStyledWithEmoji("Купить генерации", "buy", "success", EmojiIDBuy),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send insufficient quota message: %v", err)
	}
}

func (b *Bot) sendErrorMessage(chatID int64, errorText string) {
	text := fmt.Sprintf("❌ Ошибка: %s", errorText)
	msg := newMessageConfig(chatID, text)
	_, err := b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

func friendlyGenerationError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "public figure") {
		return "На фото обнаружена известная личность. К сожалению, генерация видео с публичными персонами недоступна. Попробуйте другое фото."
	}
	if strings.Contains(msg, "internal server error") ||
		strings.Contains(msg, "status code: 500") ||
		strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "task failed") ||
		strings.Contains(msg, "status code: 52") ||
		strings.Contains(msg, "status 52") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "gateway") ||
		strings.Contains(msg, "safety filter") ||
		strings.Contains(msg, "request blocked") ||
		strings.Contains(msg, "potentially dangerous") ||
		strings.Contains(msg, "content was flagged") {
		return "Нейросеть вернула ошибку. Прочитайте правила нашего бота /rules и попробуйте переформулировать ваш запрос и отправьте его снова."
	}
	return err.Error()
}

func (b *Bot) sendUnknownCommand(chatID int64) {
	text := "❓ Неизвестная команда. Используйте /help для получения списка доступных команд."
	msg := newMessageConfig(chatID, text)
	_, err := b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send unknown command message: %v", err)
	}
}

func (b *Bot) handleAdminCommand(msg *tgmodels.Message) {
	userID := msg.From.ID

	isAdmin, err := b.userService.IsUserAdmin(userID)
	if err != nil {
		b.sendErrorMessage(msg.Chat.ID, "Ошибка при проверке прав администратора")
		return
	}

	if !isAdmin {
		b.sendErrorMessage(msg.Chat.ID, "У вас нет прав администратора")
		return
	}

	cmdText := strings.TrimSpace(msg.Text)
	if cmdText == "" {
		cmdText = strings.TrimSpace(msg.Caption)
	}
	cmdName := strings.TrimSpace(getCommand(msg))
	if cmdName != "" {
		cmdPrefix := "/" + cmdName
		if strings.HasPrefix(cmdText, cmdPrefix) {
			cmdText = strings.TrimSpace(strings.TrimPrefix(cmdText, cmdPrefix))
		}
	}
	if strings.HasPrefix(cmdText, "/admin") {
		cmdText = strings.TrimSpace(strings.TrimPrefix(cmdText, "/admin"))
	}
	if cmdText == "" {
		b.sendAdminMenu(msg.Chat.ID)
		return
	}

	parts := strings.Fields(cmdText)
	if len(parts) == 0 {
		b.sendAdminMenu(msg.Chat.ID)
		return
	}

	command := parts[0]
	switch command {
	case "stats":
		b.handleAdminStats(msg.Chat.ID)
	case "users":
		b.handleAdminUsers(msg.Chat.ID)
	case "categories":
		b.sendAdminCategories(msg.Chat.ID)
	case "payments":
		b.sendPaymentsStatus(msg.Chat.ID)
	case "nano":
		b.sendNanoBananaAPIStatus(msg.Chat.ID)
	case "broadcast":
		messageText := strings.TrimSpace(strings.TrimPrefix(cmdText, command))
		if messageText == "" && msg.Photo == nil && msg.MediaGroupID == "" {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin broadcast <текст> или отправьте с фото/альбомом")
			return
		}
		if msg.MediaGroupID != "" {
			b.handleAdminBroadcastAlbum(msg)
			return
		}
		if msg.Photo != nil {
			photo := msg.Photo[len(msg.Photo)-1]
			b.handleAdminBroadcastPhoto(msg.Chat.ID, tgbotapi.FileID(photo.FileID), messageText)
			return
		}
		if msg.Document != nil && strings.HasPrefix(strings.ToLower(msg.Document.MimeType), "image/") {
			b.handleAdminBroadcastPhoto(msg.Chat.ID, tgbotapi.FileID(msg.Document.FileID), messageText)
			return
		}
		b.handleAdminBroadcast(msg.Chat.ID, messageText)
	case "sub_set":
		if len(parts) < 4 {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin sub_set <user_id> <mini|start|pro> <days>")
			return
		}
		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.sendErrorMessage(msg.Chat.ID, "Некорректный user_id")
			return
		}
		plan := strings.ToLower(strings.TrimSpace(parts[2]))
		days, err := strconv.Atoi(parts[3])
		if err != nil || days <= 0 {
			b.sendErrorMessage(msg.Chat.ID, "Некорректные days")
			return
		}
		if plan != "mini" && plan != "start" && plan != "pro" {
			b.sendErrorMessage(msg.Chat.ID, "План должен быть mini, start или pro")
			return
		}
		if err := b.userService.SetSubscription(userID, plan, days); err != nil {
			b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Не удалось выдать подписку: %v", err))
			return
		}
		b.sendText(msg.Chat.ID, fmt.Sprintf("✅ Подписка %s выдана пользователю %d на %d дней", strings.Title(plan), userID, days))
	case "sub_remove":
		if len(parts) < 2 {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin sub_remove <user_id>")
			return
		}
		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.sendErrorMessage(msg.Chat.ID, "Некорректный user_id")
			return
		}
		if err := b.userService.ResetSubscription(userID); err != nil {
			b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Не удалось убрать подписку: %v", err))
			return
		}
		b.sendText(msg.Chat.ID, fmt.Sprintf("✅ Подписка удалена у пользователя %d", userID))
	case "top_users":
		b.handleAdminTopUsers(msg.Chat.ID)
	case "photo_discount":
		b.handleAdminPhotoDiscountStatus(msg.Chat.ID)
	case "photo_discount_set":
		// Usage: /admin photo_discount_set <percent> <duration_seconds>
		if len(parts) < 3 {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin photo_discount_set <percent> <duration_seconds>\nПример: /admin photo_discount_set 50 3600 (50% скидка на 1 час)")
			return
		}
		percent, err := strconv.Atoi(parts[1])
		if err != nil || percent <= 0 || percent >= 100 {
			b.sendErrorMessage(msg.Chat.ID, "Процент скидки должен быть от 1 до 99")
			return
		}
		durationSec, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || durationSec <= 0 {
			b.sendErrorMessage(msg.Chat.ID, "Длительность должна быть положительным числом секунд")
			return
		}
		endTime := time.Now().Add(time.Duration(durationSec) * time.Second)
		if err := b.redisClient.SetPhotoDiscount(percent, endTime); err != nil {
			b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Ошибка установки скидки: %v", err))
			return
		}
		b.sendText(msg.Chat.ID, fmt.Sprintf("✅ Скидка %d%% на фото установлена до %s", percent, endTime.Format("02.01.2006 15:04:05")))
	case "photo_discount_remove":
		if err := b.redisClient.RemovePhotoDiscount(); err != nil {
			b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Ошибка удаления скидки: %v", err))
			return
		}
		b.sendText(msg.Chat.ID, "✅ Скидка на фото удалена")
	case "help":
		b.handleAdminHelp(msg.Chat.ID)
	default:
		b.sendErrorMessage(msg.Chat.ID, "Неизвестная админ-команда. Используйте /admin help")
	}
}

func (b *Bot) handleAdminPhotoDiscountStatus(chatID int64) {
	discount, err := b.redisClient.GetPhotoDiscount()
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка получения скидки: %v", err))
		return
	}
	if discount == nil || discount.Percent <= 0 {
		b.sendText(chatID, "📊 Скидка на фото: не установлена")
		return
	}
	endTime := time.Unix(discount.EndTime, 0)
	remaining := time.Until(endTime)
	b.sendText(chatID, fmt.Sprintf("📊 Скидка на фото: %d%%\n⏱️ До окончания: %s\n📅 Окончание: %s",
		discount.Percent,
		formatDuration(remaining),
		endTime.Format("02.01.2006 15:04:05")))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "истекла"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("%dд %dч %dм %dс", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dч %dм %dс", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dм %dс", minutes, seconds)
	}
	return fmt.Sprintf("%dс", seconds)
}

func (b *Bot) handleAdminBroadcast(chatID int64, text string) {
	const (
		batchSize  = 500
		numWorkers = 20
	)

	b.sendText(chatID, "📣 Запускаю рассылку...")
	b.goLimited(func() {
		// Collect all users first
		var allUsers []int64
		for offset := 0; ; offset += batchSize {
			users, err := b.userService.GetAllUsers(batchSize, offset)
			if err != nil {
				b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка при получении пользователей: %v", err))
				return
			}
			if len(users) == 0 {
				break
			}
			for _, user := range users {
				if user != nil && user.TelegramID != 0 {
					allUsers = append(allUsers, user.TelegramID)
				}
			}
		}

		total := len(allUsers)
		if total == 0 {
			b.sendText(chatID, "❌ Нет пользователей для рассылки")
			return
		}

		b.sendText(chatID, fmt.Sprintf("📊 Найдено %d пользователей, начинаю рассылку...", total))

		// Create channels for worker pool
		jobs := make(chan int64, total)
		results := make(chan bool, total)

		// Start workers
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for userID := range jobs {
					if err := b.sendMarkdownLong(userID, text); err != nil {
						log.Printf("broadcast send to %d failed: %v", userID, err)
						results <- false
					} else {
						results <- true
					}
					time.Sleep(35 * time.Millisecond)
				}
			}()
		}

		// Send jobs to workers
		go func() {
			for _, userID := range allUsers {
				jobs <- userID
			}
			close(jobs)
		}()

		// Wait for all workers to finish
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		sent := 0
		failed := 0
		for success := range results {
			if success {
				sent++
			} else {
				failed++
			}
		}

		b.sendText(chatID, fmt.Sprintf("✅ Рассылка завершена\nПолучателей: %d\nУспешно: %d\nОшибки: %d", total, sent, failed))
	})
}

func (b *Bot) handleAdminStats(chatID int64) {
	stats, err := b.generationService.GetGenerationStats()
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении статистики")
		return
	}
	b.sendAdminStatsMessage(chatID, "за всё время", stats)
}

func (b *Bot) handleAdminStatsPeriod(chatID int64, period string) {
	var from time.Time
	label := ""
	now := time.Now().UTC()
	switch period {
	case "day":
		label = "за день"
		from = now.Add(-24 * time.Hour)
	case "week":
		label = "за неделю"
		from = now.Add(-7 * 24 * time.Hour)
	case "month":
		label = "за месяц"
		from = now.Add(-30 * 24 * time.Hour)
	default:
		b.handleAdminStats(chatID)
		return
	}

	stats, err := b.generationService.GetGenerationStatsSince(from)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении статистики")
		return
	}
	b.sendAdminStatsMessage(chatID, label, stats)
}

func (b *Bot) sendAdminStatsMessage(chatID int64, periodLabel string, stats map[string]any) {

	total := numberAsInt(stats["total_requests"])
	completed := numberAsInt(stats["completed_requests"])
	failed := numberAsInt(stats["failed_requests"])
	processing := numberAsInt(stats["processing_requests"])
	successRate := numberAsFloat(stats["success_rate"])
	avgTime := numberAsFloat(stats["avg_processing_time_seconds"])

	text := fmt.Sprintf(`📊 Статистика бота (%s):

🎨 Всего запросов: %d
✅ Успешных: %d
❌ Ошибок: %d
🔄 В процессе: %d
📈 Успешность: %.1f%%`,
		periodLabel,
		total,
		completed,
		failed,
		processing,
		successRate,
	)

	if avgTime > 0 {
		text += fmt.Sprintf("\n⏱️ Среднее время: %.1f сек", avgTime)
	}

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("📅 День", "admin:stats:day"),
			newInlineKeyboardButtonData("📆 Неделя", "admin:stats:week"),
			newInlineKeyboardButtonData("🗓️ Месяц", "admin:stats:month"),
		),
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("♾️ Всё время", "admin:stats:all"),
			newBackButton("Назад", "admin:menu"),
		),
	)

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = keyboard
	_, err := b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send admin stats: %v", err)
	}
}

func (b *Bot) handleAdminUsers(chatID int64) {
	// Получаем список последних пользователей
	users, err := b.userService.GetAllUsers(10, 0) // Последние 10 пользователей
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении списка пользователей")
		return
	}

	if len(users) == 0 {
		text := "👥 Пользователей не найдено"
		msg := newMessageConfig(chatID, text)
		b.sendMsg(msg)
		return
	}

	text := "👥 Последние пользователи:\n\n"
	for i, user := range users {
		adminStatus := ""
		if user.IsAdmin {
			adminStatus = " 👑"
		}
		premiumStatus := ""
		if user.IsPremium {
			premiumStatus = " ⭐"
		}

		quotaSummary := "-"
		if quota, err := b.userService.GetQuota(user.TelegramID); err == nil && quota != nil {
			quotaSummary = fmt.Sprintf("T:%d(+%d) P:%d(+%d) M:%d(+%d) V:%d(+%d)",
				quota.TextDaily, quota.TextExtra,
				quota.ImageWeekly, quota.ImageExtra,
				quota.MusicWeekly, quota.MusicExtra,
				quota.VideoWeekly, quota.VideoExtra,
			)
		}

		text += fmt.Sprintf("%d. %s (@%s)%s%s - %s\n",
			i+1,
			user.FirstName,
			user.Username,
			adminStatus,
			premiumStatus,
			quotaSummary,
		)
	}

	msg := newMessageConfig(chatID, text)
	_, err = b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send admin users: %v", err)
	}
}

func (b *Bot) handleAdminUsersCount(chatID int64) {
	total, err := b.userService.CountUsers()
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении количества пользователей")
		return
	}
	b.sendText(chatID, fmt.Sprintf("👤 Всего пользователей: %d", total))
}

func (b *Bot) handleAdminTopUsers(chatID int64) {
	topUsers, err := b.generationService.GetTopUsersByDailyGenerations(20)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка при получении топа пользователей: %v", err))
		return
	}

	if len(topUsers) == 0 {
		b.sendText(chatID, "📈 Нет генераций за последние 24 часа")
		return
	}

	var text strings.Builder
	text.WriteString("📈 Топ пользователей за 24 часа:\n\n")

	for i, user := range topUsers {
		username := user.Username
		if username == "" {
			username = user.FirstName
		}
		if username == "" {
			username = fmt.Sprintf("ID:%d", user.UserID)
		}

		text.WriteString(fmt.Sprintf("%d. @%s\n", i+1, username))
		text.WriteString(fmt.Sprintf("   🎯 Всего: %d | 🖼️ Фото: %d | 🎬 Видео: %d | 🎵 Музыка: %d | 💬 Текст: %d\n",
			user.TotalGenerations,
			user.PhotoGenerations,
			user.VideoGenerations,
			user.MusicGenerations,
			user.TextGenerations,
		))
		text.WriteString(fmt.Sprintf("   💰 Потрачено запросов: %d\n", user.TokensSpent))
		if user.PhotoTokensSpent > 0 {
			text.WriteString(fmt.Sprintf("   💸 Фото запросы: %d (≈ %.2f ₽)\n", user.PhotoTokensSpent, user.PhotoRubles))
		}
		text.WriteString("\n")
	}

	keyboard := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newBackButton("Назад", "admin:menu"),
		),
	)

	msg := newMessageConfig(chatID, text.String())
	msg.ReplyMarkup = keyboard
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send top users: %v", err)
	}
}

func (b *Bot) handleAdminHelp(chatID int64) {
	text := `👑 Доступные админ-команды:

/admin stats - Статистика генераций
/admin top_users - Топ пользователей за день
/admin users - Список последних пользователей
/admin categories - Управление доступностью категорий
/admin payments - Управление платежами
/admin nano - Переключение Nano Banana API
/admin broadcast <текст> - Рассылка сообщения всем пользователям
/admin sub_set <user_id> <mini|start|pro> <days> - Выдать подписку
/admin sub_remove <user_id> - Убрать подписку
/admin help - Эта справка

/admin photo_discount_set 50 3600     # 50% на 1 час
/admin photo_discount_set 30 86400    # 30% на 24 часа
/admin photo_discount_set 25 604800   # 25% на неделю
/admin photo_discount_remove          # Удалить скидку
/admin photo_discount                 # Статус скидки
`

	msg := newMessageConfig(chatID, text)
	_, err := b.sendMsg(msg)
	if err != nil {
		log.Printf("Failed to send admin help: %v", err)
	}
}

// numberAsInt безопасно приводит значение к int
func numberAsInt(v interface{}) int {
	switch n := v.(type) {
	case nil:
		return 0
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

// numberAsFloat безопасно приводит значение к float64
func numberAsFloat(v interface{}) float64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func (b *Bot) sendGenerationStatus(chatID int64, req *models.GenerationRequest) {
	if req != nil && req.Status == "failed" && req.ErrorMsg != nil {
		msg := strings.TrimSpace(*req.ErrorMsg)
		if strings.Contains(msg, "prompt is required") {
			loc := b.getLocalization(req.UserID)
			text := loc.PhotoReceived + "\n\n" + loc.PhotoAddCaption
			b.sendText(chatID, text)
			return
		}
		if strings.Contains(msg, "Request successful, but the official returned empty content") ||
			strings.Contains(msg, "Произошла ошибка возможно вы нарушили правила бота.") {
			loc := b.getLocalization(req.UserID)
			b.sendText(chatID, loc.ErrDefAPIEmptyBilled)
			return
		}
	}

	statusEmoji, statusText := b.statusInfo(req.Status)
	modelLabel := ""
	modelHint := ""
	if req != nil {
		if opt, ok := findModelOption(req.Model); ok && opt.Label != "" {
			modelLabel = opt.Label
		} else {
			modelLabel = req.Model
		}
	}
	baseText := fmt.Sprintf("%s Статус генерации:\n\n<tg-emoji emoji-id=\"5271912827869737544\">🤖</tg-emoji> Модель: %s\n🔄 Статус: %s\n%s",
		statusEmoji,
		modelLabel,
		statusText,
		func() string {
			if modelHint == "" {
				return ""
			}
			return modelHint
		}(),
	)

	// Если есть картинка в data URL — отправляем как фото, чтобы не ловить "Request Entity Too Large"
	if req.Status == "completed" && req.Output != nil && *req.Output != "" {
		output := *req.Output
		caption := truncate(baseText, 900) // подпись к фото

		// Кнопка "Оживить фото" для nano-banana, nano-banana-pro и nano-banana-2
		var animateMarkup *tgmodels.InlineKeyboardMarkup
		if req.Model == "google/nano-banana" || req.Model == "google/nano-banana-pro" ||
			req.Model == "nano-banana" || req.Model == "kie/nano-banana-edit" || req.Model == "kie/nano-banana-pro" ||
			req.Model == "nano-banana-2" {
			kb := newInlineKeyboardMarkup(
				newInlineKeyboardRow(
					newInlineKeyboardButtonData("🎬 Оживить фото", "animate_photo"),
				),
			)
			animateMarkup = &kb
		}

		// Пытаемся отправить картинку
		if strings.HasPrefix(output, "data:image") {
			photoBytes, fileName, err := decodeDataURL(output)
			if err != nil {
				log.Printf("Failed to decode data URL: %v", err)
				b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
				return
			}
			if _, err := b.sendPhoto(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(photoBytes)}, caption, "HTML", animateMarkup); err != nil {
				log.Printf("Failed to send generation photo: %v", err)
				b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
			}
			return
		}

		if strings.HasPrefix(output, "http") {
			// Видео (.mp4) — отправляем как видео
			isVideo := strings.HasSuffix(strings.ToLower(strings.SplitN(output, "?", 2)[0]), ".mp4")
			if !isVideo {
				if opt, ok := findModelOption(req.Model); ok && opt.Category == ModelCategoryVideo {
					isVideo = true
				}
			}
			if isVideo {
				if _, err := b.sendVideo(chatID, &tgmodels.InputFileString{Data: output}, caption, "HTML"); err != nil {
					log.Printf("Failed to send generation video by URL: %v", err)
					if videoBytes, fileName, dlErr := downloadFileToBytes(output, "mp4"); dlErr == nil {
						if _, sendErr := b.sendDocument(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(videoBytes)}, caption); sendErr != nil {
							log.Printf("Failed to send generation video bytes: %v", sendErr)
							b.sendText(chatID, truncate(baseText+"\n\n🎬 Видео: "+output, 3800))
						}
					} else {
						log.Printf("Failed to download generation video url: %v", dlErr)
						b.sendText(chatID, truncate(baseText+"\n\n🎬 Видео: "+output, 3800))
					}
				}
				return
			}

			if _, err := b.sendPhoto(chatID, &tgmodels.InputFileString{Data: output}, caption, "HTML", animateMarkup); err != nil {
				log.Printf("Failed to send generation photo by URL: %v", err)
				if photoBytes, fileName, dlErr := downloadFileToBytes(output, "png"); dlErr == nil {
					if _, sendErr := b.sendPhoto(chatID, &tgmodels.InputFileUpload{Filename: fileName, Data: bytes.NewReader(photoBytes)}, caption, "HTML", animateMarkup); sendErr != nil {
						log.Printf("Failed to send generation photo bytes: %v", sendErr)
						b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
					}
				} else {
					log.Printf("Failed to download generation photo url: %v", dlErr)
					b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
				}
			}
			return
		}

		// Иной формат — отправляем текстом без ссылки
		b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
		return
	}

	if req.Status == "failed" && req.ErrorMsg != nil && *req.ErrorMsg != "" {
		friendly := friendlyGenerationError(fmt.Errorf(*req.ErrorMsg))
		// Возврат запросов в те же бакеты, откуда списали
		skipRefund := strings.Contains(*req.ErrorMsg, "Request successful, but the official returned empty content") ||
			strings.Contains(*req.ErrorMsg, "Произошла ошибка возможно вы нарушили правила бота.")
		if !skipRefund && req.TokensUsed > 0 && req.UserID != 0 {
			if err := b.userService.RefundQuota(req.UserID, models.QuotaCategoryImage, req.TokensPrimaryUsed, req.TokensExtraUsed); err != nil {
				log.Printf("refund on failed generation error: %v", err)
			}
			if resetErr := b.generationService.ResetRequestTokensUsed(req.ID); resetErr != nil {
				log.Printf("ResetRequestTokensUsed(image failed) error: %v", resetErr)
			}
		}
		baseText += fmt.Sprintf("\n\n❌ Ошибка: %s", friendly)
	}

	b.sendText(chatID, truncate(baseText, 3800))
}

// sendText отправляет сообщение и логирует ошибку, если она случилась
func (b *Bot) sendText(chatID int64, text string) {
	msg := newMessageConfig(chatID, text)
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

// sendLongText разбивает длинное сообщение на части, чтобы не упереться в лимит Telegram (~4096)
func (b *Bot) sendLongText(chatID int64, text string) {
	const maxChunkBytes = 3800
	text = strings.TrimSpace(text)
	for len(text) > 0 {
		if len(text) <= maxChunkBytes {
			msg := newMessageConfig(chatID, text)
			if _, err := b.sendMsg(msg); err != nil {
				log.Printf("Failed to send long message chunk: %v", err)
			}
			return
		}

		cut := maxChunkBytes
		if cut > len(text) {
			cut = len(text)
		}

		if idx := strings.LastIndex(text[:cut], "\n"); idx > 0 && idx >= cut-600 {
			cut = idx
		} else if idx := strings.LastIndex(text[:cut], " "); idx > 0 && idx >= cut-300 {
			cut = idx
		}

		for cut > 0 && !utf8.ValidString(text[:cut]) {
			cut--
		}
		if cut == 0 {
			cut = maxChunkBytes
			for cut > 0 && !utf8.RuneStart(text[cut-1]) {
				cut--
			}
			if cut == 0 {
				cut = len(text)
			}
		}

		chunk := strings.TrimSpace(text[:cut])
		text = strings.TrimSpace(text[cut:])

		if chunk == "" {
			continue
		}
		msg := newMessageConfig(chatID, chunk)
		if _, err := b.sendMsg(msg); err != nil {
			log.Printf("Failed to send long message chunk: %v", err)
		}
	}
}

func (b *Bot) notifyPaymentSuccess(userID int64, category string, qty int, amount float64, paymentID string) {
	if strings.HasPrefix(category, "subscription:") {
		plan := strings.TrimPrefix(category, "subscription:")
		plan = strings.Title(plan)
		text := fmt.Sprintf("✅ Подписка %s активирована!\nСрок: %d дней", plan, qty)
		b.sendText(userID, text)
		b.notifyAdminsAboutPurchase(userID, fmt.Sprintf("Подписка %s", plan), qty, amount, paymentID)
		return
	}
	label := categoryLabelByKey(category)
	if label == "" {
		label = category
	}
	text := fmt.Sprintf("✅ Оплата зачислена!\nКатегория: %s\nКоличество: %d", label, qty)
	b.sendText(userID, text)
	b.notifyAdminsAboutPurchase(userID, label, qty, amount, paymentID)
}

func (b *Bot) notifyAdminsAboutPurchase(userID int64, label string, qty int, amount float64, paymentID string) {
	buyer, err := b.userService.GetUserByTelegramID(userID)
	if err != nil {
		log.Printf("notifyAdminsAboutPurchase get user error: %v", err)
	}
	username := ""
	firstName := ""
	lastName := ""
	if buyer != nil {
		username = buyer.Username
		firstName = buyer.FirstName
		lastName = buyer.LastName
	}
	nameParts := strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
	if nameParts == "" {
		nameParts = "-"
	}
	userLabel := fmt.Sprintf("ID: %d", userID)
	if username != "" {
		userLabel += fmt.Sprintf(" (@%s)", username)
	}

	text := fmt.Sprintf("💳 Новая покупка\nСумма: %.2f ₽\nТариф: %s\nКоличество: %d\nПлатёж: %s\nПокупатель: %s\nИмя: %s", amount, label, qty, paymentID, userLabel, nameParts)
	admins, err := b.userService.GetAdminUsers()
	if err != nil {
		log.Printf("notifyAdminsAboutPurchase get admins error: %v", err)
		return
	}
	for _, admin := range admins {
		if admin == nil || admin.TelegramID == 0 {
			continue
		}
		b.sendText(admin.TelegramID, text)
	}
}

// sendSubscriptionInfo показывает инфо о текущей подписке и кнопки управления

func (b *Bot) sendSubscriptionInfo(chatID int64, userID int64) {
	lbl, ok, subEnd := b.userService.GetSubscriptionInfo(userID)
	if !ok || lbl == "" || lbl == "Free" {
		b.sendErrorMessage(chatID, "Активная подписка не найдена")
		return
	}
	endStr := "-"
	if subEnd != nil {
		msk := time.FixedZone("MSK", 3*3600)
		endStr = subEnd.In(msk).Format("02.01.2006 15:04")
	}
	text := fmt.Sprintf("🪄 Подписка: %s\n📅 Действует до: %s", lbl, endStr)
	kb := newInlineKeyboardMarkup(
		newInlineKeyboardRow(
			newInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = kb
	b.sendMsg(msg)
}

// statusInfo возвращает emoji и текст статуса
func (b *Bot) statusInfo(status string) (string, string) {
	switch status {
	case "pending":
		return "⏳", "В очереди"
	case "processing":
		return "🔄", "Генерируется"
	case "completed":
		return "<tg-emoji emoji-id=\"5206607081334906820\">✔️</tg-emoji>", "Завершено"
	case "failed":
		return "<tg-emoji emoji-id=\"5210952531676504517\">❌</tg-emoji>", "Ошибка"
	default:
		return "ℹ️", status
	}
}

func decodeDataURL(dataURL string) ([]byte, string, error) {
	decoded, fileName, _, err := decodeDataURLGeneric(dataURL, "jpg")
	return decoded, fileName, err
}

func decodeDataURLAudio(dataURL string) ([]byte, string, error) {
	decoded, fileName, _, err := decodeDataURLGeneric(dataURL, "mp3")
	return decoded, fileName, err
}

func downloadFileToBytes(fileURL string, defaultExt string) ([]byte, string, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(fileURL)
	if err != nil {
		return nil, "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read failed: %w", err)
	}

	fileName := "result." + defaultExt
	if parsed, err := url.Parse(fileURL); err == nil {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			fileName = base
			if !strings.Contains(base, ".") {
				fileName = base + "." + defaultExt
			}
		}
	}

	return body, fileName, nil
}

// truncate обрезает строку до maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (b *Bot) NotifyGenerationStatus(chatID int64, req *models.GenerationRequest) {
	b.sendGenerationStatus(chatID, req)
}
