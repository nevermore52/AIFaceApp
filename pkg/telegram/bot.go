package telegram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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
)

type Bot struct {
	api               *tgbotapi.BotAPI
	userService       *services.UserService
	generationService *services.GenerationService
	paymentService    *services.PaymentService
	redisClient       *redis.Client
	cfg               *config.Config
	concurrencySem    chan struct{}
	updates           tgbotapi.UpdatesChannel
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
	sunoInstrumental  map[int64]bool // userID -> true (instrumental), false (with vocal)
	sunoVoiceMu       sync.Mutex
	sunoVoice         map[int64]string // userID -> m/f
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

func (b *Bot) sendStartTrialMenu(chatID int64) {
	btn := tgbotapi.NewInlineKeyboardButtonData("Проверить подписку", "trial:check")
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)
	text := fmt.Sprintf("Чтобы получить 1 пробную генерацию фото — подпишитесь на канал %s\n%s и нажмите «Проверить подписку»", requiredChannelUsername, requiredChannelLink)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
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

func extrasDiscountActiveForCategory(category string) bool {
	if category != "image" {
		return false
	}
	deadline := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	return time.Now().UTC().Before(deadline)
}

func formatExtrasPriceMarkdownV2(category string, currentPrice int) string {
	if currentPrice < 0 {
		currentPrice = 0
	}
	if extrasDiscountActiveForCategory(category) {
		oldPrice := currentPrice * 2
		// MarkdownV2 strikethrough uses single-tilde: ~text~
		// Required format: discounted price first, old price struck on the right.
		return fmt.Sprintf("%d ₽ ~%d ₽~", currentPrice, oldPrice)
	}
	return fmt.Sprintf("%d ₽", currentPrice)
}

func escapeMarkdownV2(s string) string {
	// Telegram MarkdownV2 reserved characters: _ * [ ] ( ) ~ ` > # + - = | { } . !
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
	case "16:9", "9:16", "1:1":
		return ratio
	default:
		return "1:1"
	}
}

func (b *Bot) setUserAspectRatio(userID int64, ratio string) {
	switch ratio {
	case "16:9", "9:16", "1:1":
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
		b.sendErrorMessage(chatID, "Не удалось получить состояние Nano Banana API")
		return
	}
	state := "KieAPI"
	toggleLabel := "Переключить на DefAPI"
	if strings.EqualFold(provider, "defapi") {
		state = "DefAPI"
		toggleLabel = "Переключить на KieAPI"
	}
	text := fmt.Sprintf("🍌 Nano Banana API: %s", state)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, "admin:nano_api_toggle"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send nano banana api status: %v", err)
	}
}

func (b *Bot) toggleNanoBananaAPI(chatID int64) {
	provider, err := b.userService.GetNanoBananaProvider()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось проверить Nano Banana API")
		return
	}
	next := "defapi"
	if strings.EqualFold(provider, "defapi") {
		next = "kieapi"
	}
	if err := b.userService.SetNanoBananaProvider(next); err != nil {
		b.sendErrorMessage(chatID, "Не удалось переключить Nano Banana API")
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
	photo tgbotapi.PhotoSize
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

	if category == "music" || category == "video" {
		packs = []int{1, 5, 10, 50, 100}
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
	sb.WriteString(escapeMarkdownV2(header) + "\n\n" + escapeMarkdownV2(loc.BuySelectAction) + "\n")
	escapedUnit := escapeMarkdownV2(unit)
	for _, p := range packs {
		if price, ok := b.extrasPrice(category, p); ok {
			sb.WriteString(fmt.Sprintf("• %d %s — %s\n", p, escapedUnit, formatExtrasPriceMarkdownV2(category, price)))
		} else {
			sb.WriteString(fmt.Sprintf("• %d %s\n", p, escapedUnit))
		}
	}
	text := sb.String() + "\n" + escapeMarkdownV2(loc.BuyConsentNote)

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(packs); i += 2 {
		btn1 := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", packs[i]), fmt.Sprintf("%s:%d", callbackPrefix, packs[i]))
		if i+1 < len(packs) {
			btn2 := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", packs[i+1]), fmt.Sprintf("%s:%d", callbackPrefix, packs[i+1]))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn1, btn2))
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn1))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "buy:extras"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	reply := tgbotapi.NewMessage(chatID, text)
	reply.ParseMode = "MarkdownV2"
	reply.ReplyMarkup = keyboard

	if _, err := b.api.Send(reply); err != nil {
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.BuyPackageBackBtn, "buy:extras"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
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
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, s := range settings {
		state := "❌"
		if s.Enabled {
			state = "✅"
		}
		label := fmt.Sprintf("%s %s", state, categoryLabelByKey(s.Category))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "admin:cat:"+s.Category),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
	))

	for _, s := range settings {
		state := "выключена"
		if s.Enabled {
			state = "включена"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", categoryLabelByKey(s.Category), state))
	}

	text := strings.Join(lines, "\n")
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
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
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, "admin:payments_toggle"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send payments status: %v", err)
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
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, "admin:subs_toggle"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
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
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "admin:stats"),
			tgbotapi.NewInlineKeyboardButtonData("👥 Пользователи", "admin:users"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Категории", "admin:categories"),
			tgbotapi.NewInlineKeyboardButtonData("💳 Платежи", "admin:payments"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Баланс API", "admin:suno_balance"),
			tgbotapi.NewInlineKeyboardButtonData("🍌 Nano Banana API", "admin:nano_api"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔒 Подписки", "admin:subs"),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Справка", "admin:help"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
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

const piapiHealthAlertChatID int64 = 812157835

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
	{ID: "google/nano-banana", Label: "🚀 Nano Banana", Desc: locRU.ModelNanoBanana, Category: ModelCategoryPhoto, RequestCost: 1},
	{ID: "google/nano-banana-pro", Label: "🌟 Nano Banana Pro", Desc: locRU.ModelNanoBananaPro, Category: ModelCategoryPhoto, RequestCost: 4},
	{ID: "hug-video", ApiModel: "Qubico/hug-video", Label: "🤗 Обнимашки", Desc: locRU.ModelHugVideo, Category: ModelCategoryVideo, RequestCost: 1, TaskType: "image_to_video"},
	{ID: "music-suno", ApiModel: "suno", Label: "🎵 Suno Music", Desc: locRU.ModelSunoMusic, Category: ModelCategoryMusic, RequestCost: 1, TaskType: "music"},
	{ID: "google/gemini-3-flash", Label: "💬 Gemini 3 Flash", Desc: "", Category: ModelCategoryChat, RequestCost: 1},
	{ID: "openai/gpt-5-mini", Label: "💬 GPT-5 mini", Desc: locRU.ModelGPT5Mini, Category: ModelCategoryChat, RequestCost: 1},
	{ID: "openai/gpt-5-nano", Label: "💬 GPT-5 nano", Desc: locRU.ModelGPT5Nano, Category: ModelCategoryChat, RequestCost: 1},
	{ID: "chat-gpt-4.1mini", ApiModel: "gpt-4.1-mini", Label: "💬 GPT-4.1 mini", Desc: locRU.ModelGPT41Mini, Category: ModelCategoryChat, RequestCost: 1, TaskType: "chat"},
}

var modelCategories = []ModelCategory{ModelCategoryPhoto, ModelCategoryVideo, ModelCategoryMusic, ModelCategoryChat}
var adminModelCategories = []ModelCategory{ModelCategoryPhoto, ModelCategoryVideo, ModelCategoryMusic, ModelCategoryChat}

func NewBot(token string, userService *services.UserService, generationService *services.GenerationService, paymentService *services.PaymentService, redisClient *redis.Client, cfg *config.Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}
	bot := &Bot{
		api:               api,
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

	// Уведомление после успешного платежа
	paymentService.SetNotifier(bot.notifyPaymentSuccess)

	return bot, nil
}

func (b *Bot) Start() error {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	// Убираем вебхук, чтобы гарантировать работу long polling (иначе callback'и могут не приходить)
	if _, err := b.api.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		log.Printf("failed to delete webhook: %v", err)
	}

	// Устанавливаем команды меню
	b.setCommands()

	// Периодический healthcheck PiAPI
	go b.startPiAPIHealthCheck()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	b.updates = b.api.GetUpdatesChan(u)

	for update := range b.updates {
		// Копируем update в новую переменную, чтобы горутины не делили одну ссылку
		upd := update
		// Обрабатываем каждое событие асинхронно, чтобы не блокировать очередь
		b.goLimited(func() {
			b.safeHandleUpdate(upd)
		})
	}

	return nil
}

// Stop включает режим graceful shutdown:
// - помечает бота как "перезагружается" (новые запросы отклоняются)
// - останавливает long-polling
// - ждёт завершения всех уже запущенных обработчиков
func (b *Bot) Stop() {
	b.shutdownOnce.Do(func() {
		b.shuttingDown.Store(true)
		if b.api != nil {
			b.api.StopReceivingUpdates()
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
func (b *Bot) safeHandleUpdate(update tgbotapi.Update) {
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
			_, _ = b.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			if update.CallbackQuery.Message != nil {
				b.sendText(update.CallbackQuery.Message.Chat.ID, "В данный момент бот перезагружается. Попробуйте ещё через пару минут.")
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
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Начать"},
		{Command: "menu", Description: "Главное меню"},
		{Command: "account", Description: "Аккаунт и лимиты"},
		{Command: "buy", Description: "Купить (подписку,доп.запросы)"},
		{Command: "invite", Description: "Пригласить друзей"},
		{Command: "rules", Description: "Правила использования"},
		{Command: "help", Description: "Помощь"},
		{Command: "privacy", Description: "Политика и Польз. соглашение"},
		{Command: "settings", Description: "Настройки стиля общения"},
	}

	cfg := tgbotapi.NewSetMyCommands(commands...)
	_, err := b.api.Request(cfg)
	if err != nil {
		log.Printf("Failed to set commands: %v", err)
	}
}

// setChatCommands устанавливает команды для конкретного чата (например, добавить /admin только админам)
func (b *Bot) setChatCommands(chatID int64, isAdmin bool) {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Начать"},
		{Command: "menu", Description: "Главное меню"},
		{Command: "account", Description: "Аккаунт и лимиты"},
		{Command: "buy", Description: "Купить (подписку,доп.запросы)"},
		{Command: "invite", Description: "Пригласить друзей"},
		{Command: "rules", Description: "Правила использования"},
		{Command: "help", Description: "Помощь"},
		{Command: "privacy", Description: "Политика и Польз. соглашение"},
		{Command: "settings", Description: "Настройки стиля общения"},
	}
	if isAdmin {
		commands = append(commands, tgbotapi.BotCommand{Command: "admin", Description: "Админ-панель"})
	}
	scope := tgbotapi.NewBotCommandScopeChat(chatID)
	cfg := tgbotapi.NewSetMyCommandsWithScope(scope, commands...)
	if _, err := b.api.Request(cfg); err != nil {
		log.Printf("Failed to set chat commands: %v", err)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	user := msg.From
	if user == nil {
		return
	}

	// Если /start с реферальным кодом — обрабатываем раньше, чтобы не создать пользователя без referrer
	if msg.IsCommand() {
		cmd := msg.Command()
		args := msg.CommandArguments()
		if cmd == "start" && args != "" {
			// Сначала создаем/обновляем пользователя с referrer_id, а потом проверяем подписку
			if _, err := b.userService.GetOrCreateUserWithReferrer(
				user.ID,
				user.UserName,
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
		user.UserName,
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

	// Обрабатываем команды
	if msg.IsCommand() {
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

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := msg.CommandArguments()

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

func (b *Bot) handlePhoto(msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	loc := b.getLocalization(userID)

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

	// Проверяем, есть ли подпись к фото (caption) — только для фото-моделей
	caption := strings.TrimSpace(msg.Caption)
	if caption == "" && modelOpt.Category == ModelCategoryPhoto {
		// Для фото без описания просим добавить подпись
		examples := []string{loc.PhotoExample1, loc.PhotoExample2, loc.PhotoExample3, loc.PhotoExample4}
		text := loc.PhotoReceived + "\n\n" + loc.PhotoAddCaption + "\n\n" + loc.PhotoExamples + "\n• " + strings.Join(examples, "\n• ")
		reply := tgbotapi.NewMessage(chatID, text)
		_, _ = b.api.Send(reply)
		return
	}

	// Получаем самое большое фото
	photo := msg.Photo[len(msg.Photo)-1]

	// Получаем информацию о файле
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		log.Printf("Failed to get file info: %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить изображение")
		return
	}

	fileURL := file.Link(b.cfg.TelegramToken)

	// Если выбрана видео-модель — запускаем видео-генерацию без описания
	if modelOpt.Category == ModelCategoryVideo {
		b.processVideoGeneration(chatID, userID, fileURL, modelOpt)
		return
	}

	// Определяем тип генерации по описанию
	genType := b.detectGenerationType(caption)

	// Запускаем генерацию
	b.processGeneration(chatID, userID, []string{fileURL}, genType, caption)
}

// handleDocument обрабатывает документы как изображения (если это картинка)
func (b *Bot) handleDocument(msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

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
	caption := strings.TrimSpace(msg.Caption)
	if caption == "" && modelOpt.Category == ModelCategoryPhoto {
		text := `📷 Фото получено!

Пожалуйста, отправьте фото ещё раз, но с подписью — опишите, что хотите изменить.

Примеры:
• "Сделай короткую стрижку"
• "Примерь красное платье"
• "Измени цвет волос на блонд"
• "Удали 'определенный' объект с фото`
		reply := tgbotapi.NewMessage(chatID, text)
		_, _ = b.api.Send(reply)
		return
	}

	// Получаем информацию о файле
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: msg.Document.FileID})
	if err != nil {
		log.Printf("Failed to get file info (doc): %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить изображение")
		return
	}
	fileURL := file.Link(b.cfg.TelegramToken)

	// Если выбрана видео-модель — запускаем видео-генерацию без описания
	if modelOpt.Category == ModelCategoryVideo {
		b.processVideoGeneration(chatID, userID, fileURL, modelOpt)
		return
	}

	// Определяем тип генерации по описанию
	genType := b.detectGenerationType(caption)

	// Запускаем генерацию
	b.processGeneration(chatID, userID, []string{fileURL}, genType, caption)
}

// handleAlbumPhoto агрегирует альбом до 4 фото и запускает генерацию одним запросом
func (b *Bot) handleAlbumPhoto(msg *tgbotapi.Message) {
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
		fileSize = msg.Document.FileSize
	} else {
		return
	}
	log.Printf("handleAlbumPhoto: mediaGroup=%s msgID=%d fileID=%s uniqueID=%s", mediaID, msg.MessageID, fileID, uniqueID)

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
		msgID: msg.MessageID,
		photo: tgbotapi.PhotoSize{
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

	// Альбом поддерживаем только для фото-моделей
	if modelOpt.Category != ModelCategoryPhoto {
		b.sendErrorMessage(buf.chatID, "Для альбомов выберите фото-модель в /menu")
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
		file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: p.photo.FileID})
		if err != nil {
			log.Printf("Failed to get file info from album: %v", err)
			continue
		}
		fileIDs = append(fileIDs, p.photo.FileID)
		fileUniqueIDs = append(fileUniqueIDs, p.photo.FileUniqueID)
		url := file.Link(b.cfg.TelegramToken)
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
	} else if modelOpt.ID == "google/nano-banana-pro" {
		maxPhotos = 4
	}
	if len(imageURLs) > maxPhotos {
		imageURLs = imageURLs[:maxPhotos]
	}

	caption := strings.TrimSpace(buf.caption)

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
	case "hug-video":
		return base + `Принимает фото (без подписи) и оживляет его в видео с эффектом обнимашек.

Как отправить:
1) Пришлите фото (без подписи).
2) Дождитесь готового видео.

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
			return base + `Эта модель принимает фото (подпись не обязательна) и вернёт короткое видео.

Как отправить:
1) Пришлите фото (подпись не обязательна).
2) Дождитесь готового видео.

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
func (b *Bot) processAudioMessage(msg *tgbotapi.Message, modelOpt ModelOption) {
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
	case "hug-video":
		return loc.ModelHugVideo
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
		case "google/nano-banana", "google/nano-banana-pro":
			return loc.InstrNanoBanana
		case "hug-video":
			return loc.InstrHugVideo
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
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData(loc.SettingsChatStyle, "settings:style"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData(loc.SettingsLanguage, "settings:language"),
		},
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.SettingsBackBtn, "menu"),
	))

	text := loc.SettingsTitle + "\n\n" + loc.SettingsSelect
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send settings menu: %v", err)
	}
}

func (b *Bot) sendChatStyleMenu(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	if !b.ensureChatStyleAllowed(chatID, userID) {
		return
	}
	current := b.getUserChatStyle(userID)
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(chatStyles); i += 2 {
		btns := []tgbotapi.InlineKeyboardButton{}
		for j := i; j < len(chatStyles) && j < i+2; j++ {
			st := chatStyles[j]
			label := b.chatStyleLabel(st.ID, loc)
			if st.ID == current {
				label = "✅ " + label
			}
			btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(label, "set_style:"+st.ID))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btns...))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "settings"),
	))

	text := loc.StyleSelectTitle
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
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

	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData(ruLabel, "set_lang:ru"),
			tgbotapi.NewInlineKeyboardButtonData(enLabel, "set_lang:en"),
		},
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "settings"),
		tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
	))

	msg := tgbotapi.NewMessage(chatID, loc.LangTitle)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send language menu: %v", err)
	}
}

// processVideoGeneration обрабатывает видео-генерацию из фото (image_to_video)
func (b *Bot) processVideoGeneration(chatID int64, userID int64, photoURL string, modelOpt ModelOption) {
	if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
		return
	}

	if modelOpt.Category != ModelCategoryVideo {
		b.sendErrorMessage(chatID, "Выберите видео-модель в меню моделей, чтобы оживить фото.")
		return
	}

	requestCost := modelOpt.RequestCost
	if requestCost < 1 {
		requestCost = 15
	}

	if err := b.userService.ConsumeQuota(userID, models.QuotaCategoryVideo, requestCost); err != nil {
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryVideo, requestCost, err)
		return
	}

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
		msg := tgbotapi.NewAudio(chatID, tgbotapi.FileBytes{
			Name:  fileName,
			Bytes: audioBytes,
		})
		msg.Caption = caption
		if _, err := b.api.Send(msg); err != nil {
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
		audioMsg := tgbotapi.NewAudio(chatID, tgbotapi.FileBytes{
			Name:  fileName,
			Bytes: audioBytes,
		})
		audioMsg.Caption = caption
		if _, err := b.api.Send(audioMsg); err != nil {
			log.Printf("Failed to send audio (bytes): %v, trying as document", err)
			// Пытаемся как документ (например, если формат flac не поддерживается как Audio)
			docMsg := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
				Name:  fileName,
				Bytes: audioBytes,
			})
			docMsg.Caption = caption
			if _, err := b.api.Send(docMsg); err != nil {
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

		videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileBytes{
			Name:  fileName,
			Bytes: videoBytes,
		})
		videoMsg.Caption = caption
		if _, err := b.api.Send(videoMsg); err != nil {
			log.Printf("Failed to send video: %v, trying as document", err)
			docMsg := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
				Name:  fileName,
				Bytes: videoBytes,
			})
			docMsg.Caption = caption
			if _, err := b.api.Send(docMsg); err != nil {
				log.Printf("Failed to send video as document: %v", err)
				b.sendText(chatID, truncate(caption+"\n\nВидео недоступно", 3800))
			}
		}
		return
	}

	b.sendText(chatID, truncate(caption+"\n\nВидео: "+output, 3800))
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	data := callback.Data
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID

	// Мгновенно отвечаем, чтобы убрать индикатор ожидания
	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))

	// Кулдаун на любые запросы (кроме настроек Suno)
	if !strings.HasPrefix(data, "suno_instr") && !strings.HasPrefix(data, "suno_voice") {
		if !b.checkCooldown(chatID, userID) {
			return
		}
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
		b.sendModelMenu(chatID, userID, ModelCategoryMusic, callback.Message.MessageID)
	case "suno_voice_toggle":
		cur := b.getSunoVoice(userID)
		if cur == "f" {
			b.setSunoVoice(userID, "m")
		} else {
			b.setSunoVoice(userID, "f")
		}
		b.sendModelMenu(chatID, userID, ModelCategoryMusic, callback.Message.MessageID)
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
		b.sendAspectRatioMenu(chatID, userID, callback.Message.MessageID)
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
	case "invite":
		b.sendInviteInfo(chatID, userID)
	case "models_menu":
		msgID := callback.Message.MessageID
		if callback.Message != nil && callback.Message.Photo != nil && len(callback.Message.Photo) > 0 {
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
			b.sendAspectRatioMenu(chatID, userID, callback.Message.MessageID)
		} else if strings.HasPrefix(data, "models_menu:") {
			cat := ModelCategory(strings.TrimPrefix(data, "models_menu:"))
			b.sendModelMenu(chatID, userID, cat, callback.Message.MessageID)
		} else if strings.HasPrefix(data, "buy_package:") {
			parts := strings.Split(data, ":")
			if len(parts) == 3 {
				category := parts[1]
				pack := parts[2]
				b.sendBuyPackageInfo(chatID, userID, category, pack)
			}
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
			b.setUserModel(userID, model)

			// Обновляем меню моделей в том же сообщении
			b.sendModelMenu(chatID, userID, opt.Category, callback.Message.MessageID)
		}
	}

	// Отвечаем на callback
	// (пусто — ответили мгновенно в начале)
}

func (b *Bot) isChannelMember(userID int64) (bool, error) {
	member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: requiredChannelUsername,
			UserID:             userID,
		},
	})
	if err != nil {
		return false, err
	}
	status := member.Status
	if status == "creator" || status == "administrator" || status == "member" || (status == "restricted" && member.IsMember) {
		return true, nil
	}
	return false, nil
}

func (b *Bot) sendChannelTrialMenu(chatID int64) {
	checkBtn := tgbotapi.NewInlineKeyboardButtonData("Проверить подписку", "trial:check")
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(checkBtn),
	)
	text := "Подпишитесь на канал t.me/AIFaceApps, чтобы получить 1 пробную генерацию фото.\nПосле подписки нажмите «Проверить подписку» и попробуйте снова."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("sendChannelTrialMenu send message error: %v", err)
	}
}

// processGeneration обрабатывает генерацию изображения
func (b *Bot) processGeneration(chatID int64, userID int64, photoURLs []string, genType string, prompt string) {
	if !b.ensureCategoryEnabled(chatID, ModelCategoryPhoto) {
		return
	}

	if len(photoURLs) == 0 {
		b.sendErrorMessage(chatID, "Не получено ни одного изображения")
		return
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
	opts := services.GenerationOptions{
		InputImages:       imageList,
		InputImage:        imageList[0],
		Prompt:            prompt,
		TokensCost:        requestCost,
		TokensPrimaryUsed: primaryUsed,
		TokensExtraUsed:   extraUsed,
		ChatID:            chatID,
		Model:             modelOpt.ID,
		ModelType:         string(modelOpt.Category),
		Username:          username,
	}
	if modelOpt.ID == "google/nano-banana" || modelOpt.ID == "google/nano-banana-pro" {
		opts.AspectRatio = b.getUserAspectRatio(userID)
	}
	if modelOpt.ID == "google/nano-banana" || modelOpt.ID == "google/nano-banana-pro" {
		if provider, err := b.userService.GetNanoBananaProvider(); err == nil {
			opts.NanoBananaProvider = provider
		}
		opts.UseDefAPI = false
	} else {
		if useDef, err := b.userService.IsNanoBananaDefAPIEnabled(); err == nil {
			opts.UseDefAPI = useDef
		}
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
	msg := tgbotapi.NewMessage(chatID, text)
	_, err = b.api.Send(msg)
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

func (b *Bot) handleTextMessage(msg *tgbotapi.Message) {
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
		reply := tgbotapi.NewMessage(msg.Chat.ID, loc.Thinking)
		if _, err := b.api.Send(reply); err != nil {
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
	reply := tgbotapi.NewMessage(msg.Chat.ID, instruction)
	_, _ = b.api.Send(reply)
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

func startPromoText(loc *Localization) string {
	deadline := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()
	if !now.Before(deadline) {
		return ""
	}
	left := deadline.Sub(now)
	days := int(left.Hours()) / 24
	hours := int(left.Hours()) % 24
	minutes := int(left.Minutes()) % 60

	return fmt.Sprintf(loc.StartPromoTitle) + "\n" + fmt.Sprintf(loc.StartPromoCountdown, days, hours, minutes)
}

// handleStart обрабатывает команду /start
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	if isAdmin, err := b.userService.IsUserAdmin(msg.From.ID); err == nil {
		b.setChatCommands(msg.Chat.ID, isAdmin)
	}
	loc := b.getLocalization(msg.From.ID)
	text := loc.WelcomeText
	if promo := startPromoText(loc); promo != "" {
		text += "\n\n" + promo
	}
	b.sendText(msg.Chat.ID, text)
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
func (b *Bot) handleStartWithReferral(msg *tgbotapi.Message, referralCode string) {
	user := msg.From
	_, err := b.userService.GetOrCreateUserWithReferrer(
		user.ID,
		user.UserName,
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
	if promo := startPromoText(loc); promo != "" {
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

	text := fmt.Sprintf(loc.MenuTitle,
		userID,
		subLabel,
		displayCategory,
		b.modelLabelLoc(currentModel, loc),
		limitLine,
	)

	// reply-клавиатура
	replyKB := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(loc.MenuGenPhotoBtn),
			tgbotapi.NewKeyboardButton(loc.MenuGenMusicBtn),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(loc.MenuInviteFriendBtn),
			tgbotapi.NewKeyboardButton(loc.MenuBuyBtn),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(loc.MenuAccountBtn),
			tgbotapi.NewKeyboardButton(loc.MenuSettingsBtn),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(loc.MenuHelpBtn),
		),
	)
	replyKB.ResizeKeyboard = true

	// инлайн-кнопки
	inlineKB := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuBuyBtn, "buy"),
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuInviteFriendBtn, "invite"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuSelectModelBtn, "models_menu"),
		),
	)

	// Админ-кнопка только для админов
	if isAdmin, err := b.userService.IsUserAdmin(userID); err == nil && isAdmin {
		b.setChatCommands(chatID, true)
	}

	// Сообщение для установки reply-клавиатуры (краткий текст)
	reply := tgbotapi.NewMessage(chatID, loc.MenuBtn)
	reply.ReplyMarkup = replyKB
	reply.DisableNotification = true
	if _, err := b.api.Send(reply); err != nil {
		log.Printf("Failed to send main menu reply keyboard: %v", err)
	}

	// Сообщение с основным текстом и инлайн-кнопками
	inlineMsg := tgbotapi.NewMessage(chatID, text)
	inlineMsg.ReplyMarkup = inlineKB
	if _, err := b.api.Send(inlineMsg); err != nil {
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
		fmt.Sprintf(loc.AccountUserID, userID) + "\n" +
			fmt.Sprintf(loc.AccountSubscription, subType) + "\n" +
			fmt.Sprintf(loc.AccountValidUntil, subEndStr) + "\n" +
			"------------------------------\n" +
			fmt.Sprintf(loc.AccountTextDaily, textDaily) + "\n" +
			fmt.Sprintf(loc.AccountImagesWeekly, imageWeekly) + "\n" +
			fmt.Sprintf(loc.AccountMusicWeekly, musicWeekly) + "\n" +
			fmt.Sprintf(loc.AccountVideoWeekly, videoWeekly) + "\n" +
			"------------------------------\n" +
			fmt.Sprintf(loc.AccountExtraText, textExtra) + "\n" +
			fmt.Sprintf(loc.AccountExtraImages, imageExtra) + "\n" +
			fmt.Sprintf(loc.AccountExtraMusic, musicExtra) + "\n" +
			fmt.Sprintf(loc.AccountExtraVideo, videoExtra),
	)

	msg := tgbotapi.NewMessage(chatID, accountText)
	if _, err := b.api.Send(msg); err != nil {
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

	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}
	if subsEnabled {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(loc.BuySubscriptionBtn, "buy:sub"))
	}
	row = append(row, tgbotapi.NewInlineKeyboardButtonData(loc.BuyExtrasBtn, "buy:extras"))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn+" "+loc.MenuBtn, "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard

	_, err := b.api.Send(reply)
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

	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, cat := range enabled {
		switch cat {
		case ModelCategoryChat:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(loc.ExtrasTexts, "buy:text")))
		case ModelCategoryPhoto:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(loc.ExtrasImage, "buy:image")))
		case ModelCategoryMusic:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(loc.ExtrasMusico, "buy:music")))
		case ModelCategoryVideo:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(loc.ExtrasVideos, "buy:video")))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "buy"),
		tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
	))

	text := loc.ExtrasTitle + "\n\n" + loc.ExtrasSelectCat + "\n" + loc.BuyConsentNote

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard

	if _, err := b.api.Send(reply); err != nil {
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

	text := fmt.Sprintf("%s\n%s \n\n✨ Mini — %d ₽ %s\n• 50 %s\n• 30 %s\n• 5 %s\n• %s\n• %s\n• %s\n\n🚀 Start — %d ₽ %s\n• 100 %s\n• 70 %s\n• 10 %s\n• %s\n• %s \n• %s\n• %s\n\n👑 Pro — %d ₽ %s\n• 300 %s\n• 150 %s\n• 15 %s\n• %s\n• %s, %s, %s\n• %s\n• %s",
		loc.SubsTitle,
		loc.BuyConsentNote,
		miniPrice, loc.SubsPerWeek, loc.SubsTextDaily, loc.SubsImages, loc.SubsSongs, loc.SubsTextModelsMini, fmt.Sprintf(loc.SubsDiscount, 10), loc.SubsNoChannel,
		startPrice, loc.SubsPerWeek, loc.SubsTextDaily, loc.SubsImages, loc.SubsSongs, fmt.Sprintf(loc.SubsContext, 2), loc.SubsTextModelsHi, fmt.Sprintf(loc.SubsDiscount, 15), loc.SubsNoChannel,
		proPrice, loc.SubsPerWeek, loc.SubsTextDaily, loc.SubsImages, loc.SubsSongs, loc.SubsTextModelsHi, fmt.Sprintf(loc.SubsChatStyles, 6), fmt.Sprintf(loc.SubsContext, 3), loc.SubsNoAds, fmt.Sprintf(loc.SubsDiscount, 20), loc.SubsNoChannel,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐ Mini", "buy_sub:mini"),
			tgbotapi.NewInlineKeyboardButtonData("🚀 Start", "buy_sub:start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Pro", "buy_sub:pro"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "buy"),
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
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
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.SubsBackToSubs, "buy:sub"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.MenuBtn, "menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
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

	botUsername := b.api.Self.UserName
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
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.InlineKeyboardButton{Text: loc.InviteCopyHint, SwitchInlineQueryCurrentChat: &copyQuery},
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn+" "+loc.MenuBtn, "menu"),
		),
	)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard
	reply.ParseMode = "Markdown"

	_, err = b.api.Send(reply)
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

	rows := [][]tgbotapi.InlineKeyboardButton{}

	// Ряд с категориями (только включённые)
	catButtons := []tgbotapi.InlineKeyboardButton{}
	for _, cat := range enabledCats {
		label := b.categoryLabelLoc(cat, loc)
		if cat == category {
			label = "✅ " + label
		}
		catButtons = append(catButtons, tgbotapi.NewInlineKeyboardButtonData(label, "models_menu:"+string(cat)))
	}
	rows = append(rows, catButtons)

	// Ряды с моделями выбранной категории
	options := modelOptionsByCategory(category)
	row := []tgbotapi.InlineKeyboardButton{}
	for i, m := range options {
		if !b.isModelVisibleToUser(userID, m) {
			continue
		}
		label := m.Label
		if m.Category == ModelCategoryChat && !b.isChatModelAllowed(userID, m) {
			label = "🔒 " + label
		}
		if m.ID == current {
			label = "✅ " + label
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, "model_set:"+m.ID))
		if len(row) == 2 || i == len(options)-1 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "menu"),
	))

	text := fmt.Sprintf(loc.ModelsCategory, b.categoryLabelLoc(category, loc)) + "\n" + loc.ModelsSelect
	currentDesc := b.modelDescriptionLoc(current, loc)
	if currentDesc != "" {
		text += "\n" + loc.ModelsCurrent + ": " + b.modelLabelLoc(current, loc) + " — " + currentDesc
		if cost := modelRequestCost(current); cost > 0 {
			text += "\n" + fmt.Sprintf(loc.ModelsCost, cost, requestWord(cost, loc))
		}
		if current == "google/nano-banana" {
			text += "\n" + fmt.Sprintf(loc.ModelsMaxPhotos, 2) + "\n"
		} else if current == "google/nano-banana-pro" {
			text += "\n" + fmt.Sprintf(loc.ModelsMaxPhotos, 4) + "\n"
		}
		if instr := b.modelInstructionLoc(current, loc); instr != "" {
			text += "\n" + instr
		}
	}
	if category == ModelCategoryMusic {
		if opt, ok := findModelOption(current); ok && (opt.ID == "music-suno" || strings.Contains(strings.ToLower(opt.ApiModel), "suno")) {
			instrOn := b.getSunoInstrumental(userID)
			instrBtn := "🎹 " + loc.MusicMode + " " + loc.MusicModeVocal
			if instrOn {
				instrBtn = "🎹 " + loc.MusicMode + " " + loc.MusicModeInstr
			}
			voice := b.getSunoVoice(userID)
			voiceBtn := "🗣️ " + loc.MusicVoice + " " + loc.MusicVoiceMale
			if voice == "f" {
				voiceBtn = "🗣️ " + loc.MusicVoice + " " + loc.MusicVoiceFemale
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(instrBtn, "suno_instr_toggle"),
			))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(voiceBtn, "suno_voice_toggle"),
			))
		}
	}
	if category == ModelCategoryPhoto && (current == "google/nano-banana" || current == "google/nano-banana-pro" || current == "kie/nano-banana-edit" || current == "kie/nano-banana-pro") {
		ratio := b.getUserAspectRatio(userID)
		label := loc.AspectTitle + ": " + ratio
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "aspect_menu"),
		))
	}
	if desc := strings.TrimSpace(loc.ModelsDescription); desc != "" {
		text += "\n\n" + desc
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageTextOrCaption(chatID, messageID, text, keyboard); err != nil {
			log.Printf("Failed to edit model menu: %v", err)
		}
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send model menu: %v", err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	text := `Тех поддержка: @wwqeew52`

	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

func (b *Bot) sendAspectRatioMenu(chatID int64, userID int64, messageID int) {
	loc := b.getLocalization(userID)
	current := b.getUserAspectRatio(userID)
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			aspectOptionButton(loc.AspectLandscape, "16:9", current),
			aspectOptionButton(loc.AspectPortrait, "9:16", current),
		},
		{
			aspectOptionButton(loc.AspectSquare, "1:1", current),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData(loc.BackBtn, "models_menu"),
		},
	}
	text := loc.AspectTitle + ": " + current
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if previewURL := aspectPreviewURL(current); previewURL != "" {
		media := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(previewURL))
		media.Caption = text
		if messageID > 0 {
			edited := tgbotapi.EditMessageMediaConfig{
				BaseEdit: tgbotapi.BaseEdit{ChatID: chatID, MessageID: messageID, ReplyMarkup: &markup},
				Media:    media,
			}
			if _, err := b.api.Send(edited); err != nil {
				if !strings.Contains(err.Error(), "message is not modified") {
					log.Printf("Failed to edit aspect ratio media: %v", err)
				}
			}
			return
		}
		msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(previewURL))
		msg.Caption = text
		msg.ReplyMarkup = markup
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Failed to send aspect ratio photo: %v", err)
		}
		return
	}

	if messageID > 0 {
		edited := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, markup)
		if _, err := b.api.Send(edited); err != nil {
			if !strings.Contains(err.Error(), "message is not modified") {
				log.Printf("Failed to edit aspect ratio menu: %v", err)
			}
		}
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send aspect ratio menu: %v", err)
	}
}

func aspectOptionButton(label, ratio, current string) tgbotapi.InlineKeyboardButton {
	if ratio == current {
		label = "✅ " + label
	}
	return tgbotapi.NewInlineKeyboardButtonData(label, "aspect_set:"+ratio)
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

// editMessageTextOrCaption пытается отредактировать текстовое сообщение, если не удалось — подпись медиа.
func (b *Bot) editMessageTextOrCaption(chatID int64, messageID int, text string, markup tgbotapi.InlineKeyboardMarkup) error {
	editText := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, markup)
	if _, err := b.api.Send(editText); err == nil {
		return nil
	} else {
		errText := err.Error()
		if strings.Contains(errText, "message is not modified") {
			return nil
		}
		// Падаем на edit caption только если это действительно медиа-сообщение.
		if strings.Contains(errText, "there is no text in the message to edit") || strings.Contains(errText, "message to edit not found") {
			editCaption := tgbotapi.NewEditMessageCaption(chatID, messageID, text)
			editCaption.ReplyMarkup = &markup
			if _, errCap := b.api.Send(editCaption); errCap == nil {
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

// sendPrivacyMessage отправляет ссылку на политику конфиденциальности
func (b *Bot) sendPrivacyMessage(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	text := loc.PrivacyPolicy + " https://telegra.ph/Politika-Konfidencialnosti-01-14-87\n\n" + loc.PrivacyTerms + " https://telegra.ph/Polzovatelskoe-soglashenie-Usloviya-EHkspluatacii-i-Obsluzhivaniya-01-14"
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send privacy message: %v", err)
	}
}

func (b *Bot) sendRulesMessage(chatID int64, userID int64) {
	loc := b.getLocalization(userID)
	text := loc.RulesTitle + "\n\n" + loc.RulesContent
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send rules message: %v", err)
	}
}

func (b *Bot) startPiAPIHealthCheck() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		if err := b.generationService.HealthCheckPiAPI(); err != nil {
			alert := fmt.Sprintf("⚠️ PiAPI healthcheck error: %v", err)
			b.sendText(piapiHealthAlertChatID, alert)
		}
		<-ticker.C
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Меню", "menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send user stats: %v", err)
	}
	_ = user // suppress unused warning
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

	text := fmt.Sprintf("❌ Недостаточно %s. Нужно %d.\n%s\n\nИспользуйте /buy чтобы докупить доп. запросы.", label, need, friendlyGenerationError(cause))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Купить доп. запросы", "buy"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send insufficient quota message: %v", err)
	}
}

func (b *Bot) sendErrorMessage(chatID int64, errorText string) {
	text := fmt.Sprintf("❌ Ошибка: %s", errorText)
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

func friendlyGenerationError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "internal server error") ||
		strings.Contains(msg, "status code: 500") ||
		strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "task failed") ||
		strings.Contains(msg, "status code: 52") || // 520/521/522/524 от Cloudflare/серверов
		strings.Contains(msg, "status 52") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "gateway") {
		return "Сервис вернул ошибку. Прочитайте правила нашего бота /rules и попробуйте переформулировать ваш запрос и отправьте его снова."
	}
	return err.Error()
}

func (b *Bot) sendUnknownCommand(chatID int64) {
	text := "❓ Неизвестная команда. Используйте /help для получения списка доступных команд."
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send unknown command message: %v", err)
	}
}

func (b *Bot) handleAdminCommand(msg *tgbotapi.Message) {
	userID := msg.From.ID

	// Проверяем, является ли пользователь админом
	isAdmin, err := b.userService.IsUserAdmin(userID)
	if err != nil {
		b.sendErrorMessage(msg.Chat.ID, "Ошибка при проверке прав администратора")
		return
	}

	if !isAdmin {
		b.sendErrorMessage(msg.Chat.ID, "У вас нет прав администратора")
		return
	}

	// Базовые команды администратора
	args := strings.Fields(msg.Text)
	if len(args) < 2 {
		b.sendAdminMenu(msg.Chat.ID)
		return
	}

	command := args[1]
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
	case "sub_set":
		if len(args) < 5 {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin sub_set <user_id> <mini|start|pro> <days>")
			return
		}
		userID, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			b.sendErrorMessage(msg.Chat.ID, "Некорректный user_id")
			return
		}
		plan := strings.ToLower(strings.TrimSpace(args[3]))
		days, err := strconv.Atoi(args[4])
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
		if len(args) < 3 {
			b.sendErrorMessage(msg.Chat.ID, "Использование: /admin sub_remove <user_id>")
			return
		}
		userID, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			b.sendErrorMessage(msg.Chat.ID, "Некорректный user_id")
			return
		}
		if err := b.userService.ResetSubscription(userID); err != nil {
			b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Не удалось убрать подписку: %v", err))
			return
		}
		b.sendText(msg.Chat.ID, fmt.Sprintf("✅ Подписка удалена у пользователя %d", userID))
	case "help":
		b.handleAdminHelp(msg.Chat.ID)
	default:
		b.sendErrorMessage(msg.Chat.ID, "Неизвестная админ-команда. Используйте /admin help")
	}
}

func (b *Bot) handleAdminStats(chatID int64) {
	// Получаем статистику генераций
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 День", "admin:stats:day"),
			tgbotapi.NewInlineKeyboardButtonData("📆 Неделя", "admin:stats:week"),
			tgbotapi.NewInlineKeyboardButtonData("🗓️ Месяц", "admin:stats:month"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("♾️ Всё время", "admin:stats:all"),
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "admin:menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	_, err := b.api.Send(msg)
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
		msg := tgbotapi.NewMessage(chatID, text)
		b.api.Send(msg)
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

	msg := tgbotapi.NewMessage(chatID, text)
	_, err = b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send admin users: %v", err)
	}
}

func (b *Bot) handleAdminHelp(chatID int64) {
	text := `👑 Доступные админ-команды:

/admin stats - Статистика генераций
/admin users - Список последних пользователей
/admin categories - Управление доступностью категорий
/admin payments - Управление платежами
/admin nano - Переключение Nano Banana API
/admin sub_set <user_id> <mini|start|pro> <days> - Выдать подписку
/admin sub_remove <user_id> - Убрать подписку
/admin help - Эта справка
`

	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
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
		if strings.EqualFold(req.Model, "google/nano-banana") || strings.EqualFold(req.Model, "nano-banana") || strings.EqualFold(req.Model, "gemini") || strings.EqualFold(req.Model, "gemini-2.5-flash-image") || strings.EqualFold(req.Model, "kie/nano-banana-edit") {
			modelHint = "❗️ Если вас не устраивает результат, попробуйте выбрать Pro модель"
		}
	}
	baseText := fmt.Sprintf(`%s Статус генерации:

🤖 Модель: %s
🔄 Статус: %s
%s`,
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

		// Пытаемся отправить картинку
		if strings.HasPrefix(output, "data:image") {
			photoBytes, fileName, err := decodeDataURL(output)
			if err != nil {
				log.Printf("Failed to decode data URL: %v", err)
				b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
				return
			}
			msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
				Name:  fileName,
				Bytes: photoBytes,
			})
			msg.Caption = caption
			if _, err := b.api.Send(msg); err != nil {
				log.Printf("Failed to send generation photo: %v", err)
				b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
			}
			return
		}

		if strings.HasPrefix(output, "http") {
			msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(output))
			msg.Caption = caption
			if _, err := b.api.Send(msg); err != nil {
				log.Printf("Failed to send generation photo by URL: %v", err)
				// запасной вариант — скачиваем и отправляем байтами
				if photoBytes, fileName, dlErr := downloadFileToBytes(output, "png"); dlErr == nil {
					photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
						Name:  fileName,
						Bytes: photoBytes,
					})
					photoMsg.Caption = caption
					if _, sendErr := b.api.Send(photoMsg); sendErr != nil {
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
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

// sendLongText разбивает длинное сообщение на части, чтобы не упереться в лимит Telegram (~4096)
func (b *Bot) sendLongText(chatID int64, text string) {
	const maxChunkBytes = 3800
	text = strings.TrimSpace(text)
	for len(text) > 0 {
		if len(text) <= maxChunkBytes {
			msg := tgbotapi.NewMessage(chatID, text)
			if _, err := b.api.Send(msg); err != nil {
				log.Printf("Failed to send long message chunk: %v", err)
			}
			return
		}

		cut := maxChunkBytes
		if cut > len(text) {
			cut = len(text)
		}

		// Prefer splitting on a natural boundary near the end of the chunk.
		if idx := strings.LastIndex(text[:cut], "\n"); idx > 0 && idx >= cut-600 {
			cut = idx
		} else if idx := strings.LastIndex(text[:cut], " "); idx > 0 && idx >= cut-300 {
			cut = idx
		}

		// Ensure we don't cut in the middle of a UTF-8 encoded rune.
		for cut > 0 && !utf8.ValidString(text[:cut]) {
			cut--
		}
		if cut == 0 {
			// Fallback: find a safe rune boundary.
			cut = maxChunkBytes
			for cut > 0 && !utf8.RuneStart(text[cut-1]) {
				cut--
			}
			if cut == 0 {
				// Give up and send what we can (should not normally happen).
				cut = len(text)
			}
		}

		chunk := strings.TrimSpace(text[:cut])
		text = strings.TrimSpace(text[cut:])

		if chunk == "" {
			continue
		}
		msg := tgbotapi.NewMessage(chatID, chunk)
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Failed to send long message chunk: %v", err)
		}
	}
}

// notifyPaymentSuccess отправляет уведомление о зачислении запросов
func (b *Bot) notifyPaymentSuccess(userID int64, category string, qty int) {
	if strings.HasPrefix(category, "subscription:") {
		plan := strings.TrimPrefix(category, "subscription:")
		plan = strings.Title(plan)
		text := fmt.Sprintf("✅ Подписка %s активирована!\nСрок: %d дней", plan, qty)
		b.sendText(userID, text)
		return
	}
	label := categoryLabelByKey(category)
	if label == "" {
		label = category
	}
	text := fmt.Sprintf("✅ Оплата зачислена!\nКатегория: %s\nКоличество: %d", label, qty)
	b.sendText(userID, text)
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
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	b.api.Send(msg)
}

// statusInfo возвращает emoji и текст статуса
func (b *Bot) statusInfo(status string) (string, string) {
	switch status {
	case "pending":
		return "⏳", "В очереди"
	case "processing":
		return "🔄", "Генерируется"
	case "completed":
		return "✅", "Завершено"
	case "failed":
		return "❌", "Ошибка"
	default:
		return "ℹ️", status
	}
}

// decodeDataURL декодирует data:image/...;base64,.... возвращает байты и имя файла
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

// NotifyGenerationStatus используется сервисом генерации для отправки результата пользователю
func (b *Bot) NotifyGenerationStatus(chatID int64, req *models.GenerationRequest) {
	b.sendGenerationStatus(chatID, req)
}
