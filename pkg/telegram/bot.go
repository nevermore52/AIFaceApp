package telegram

import (
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
	"time"

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

func (b *Bot) extrasPrice(category string, qty int) (int, bool) {
	if b.paymentService == nil {
		return 0, false
	}
	return b.paymentService.ExtrasPrice(category, qty)
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
	enabled, err := b.userService.IsNanoBananaDefAPIEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось получить состояние Nano Banana API")
		return
	}
	state := "PIAPI"
	toggleLabel := "Переключить на DefAPI"
	if enabled {
		state = "DefAPI"
		toggleLabel = "Переключить на PiAPI"
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
	enabled, err := b.userService.IsNanoBananaDefAPIEnabled()
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось проверить Nano Banana API")
		return
	}
	if err := b.userService.SetNanoBananaDefAPIEnabled(!enabled); err != nil {
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
	RequestCost int
	ModelLabel  string
	Prompt      string
	CreatedAt   time.Time
}

func (b *Bot) Config() *config.Config {
	return b.cfg
}

func (b *Bot) sendBuyExtrasCategory(chatID int64, title string, callbackPrefix string) {
	packs := []int{10, 50, 100, 250, 500}
	if strings.Contains(callbackPrefix, "music") || strings.Contains(callbackPrefix, "video") {
		packs = []int{1, 5, 10, 50, 100}
	}
	category := strings.TrimPrefix(callbackPrefix, "buy_package:")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💰 %s\n\nС подпиской скидка до 20%%\nЦены указаны без учета скидки\n\nВыберите пакет:\n", title))
	for _, p := range packs {
		if price, ok := b.extrasPrice(category, p); ok {
			sb.WriteString(fmt.Sprintf("• %d запросов — %d ₽\n", p, price))
		} else {
			sb.WriteString(fmt.Sprintf("• %d запросов\n", p))
		}
	}
	text := sb.String()

	// Формируем кнопки по два в ряд
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
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "buy:extras"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard

	if _, err := b.api.Send(reply); err != nil {
		log.Printf("Failed to send buy extras category menu: %v", err)
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

	resp, err := b.paymentService.CreateExtrasPayment(userID, category, qty)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Не удалось создать платеж: %v", err))
		return
	}

	label := "запросы"
	switch category {
	case "text":
		label = "текстовые запросы"
	case "image":
		label = "запросы на изображения"
	case "music":
		label = "музыкальные запросы"
	case "video":
		label = "видео-запросы"
	}

	text := fmt.Sprintf(`✅ Вы выбрали пакет %s: %d шт.
Перейдите по ссылке для оплаты:
%s`, label, qty, resp.CheckoutURL)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к категориям", "buy:extras"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
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
💬 Чат-бот — ответы на вопросы и сопровождение.`

const chatSystemPrompt = "Отвечай чётко и по делу, избегай излишних предисловий и не нарушай правила бота (никакого 18+, насилия, нелегала). Не ссылайся на ограничения объёма или «формата ответа» — если запрос слишком большой или непонятный, попроси пользователя кратко уточнить суть одним предложением."

type chatStyle struct {
	ID     string
	Label  string
	Prompt string
}

var chatStyles = []chatStyle{
	{ID: "normal", Label: "✅ Обычный", Prompt: "Отвечай нейтрально и лаконично."},
	{ID: "formal", Label: "📘 Формальный", Prompt: "Держи деловой и уважительный тон."},
	{ID: "humor", Label: "😂 Юмористический", Prompt: "Отвечай с лёгким дружелюбным юмором без сарказма и оскорблений."},
	{ID: "informal", Label: "😎 Неформальный", Prompt: "Пиши в расслабленном разговорном тоне, без канцелярита и без грубости."},
	{ID: "friendly", Label: "🤝 Дружеский", Prompt: "Будь тёплым и поддерживающим, но не многословным."},
	{ID: "expert", Label: "🧠 Экспертный", Prompt: "Давай чёткие, структурированные и экспертные ответы по делу."},
	{ID: "empathetic", Label: "❤️ Сочувствующий", Prompt: "Отвечай заботливо и поддерживающе, избегай резкости."},
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
}

// isUserAllowed проверяет доступ по белому списку (ADMIN_TELEGRAM_IDS)
func (b *Bot) isUserAllowed(userID int64) bool {
	if len(b.cfg.AdminIDs) == 0 {
		// Белый список не задан — доступ открыт
		return true
	}
	for _, id := range b.cfg.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// Доступные модели для выбора
var modelOptions = []ModelOption{
	{ID: "google/nano-banana", Label: "🚀 Nano Banana", Desc: "Фото: Старая модель, среднее качество, плохо работает с большими запросами и двумя фотографиями. Среднее время ожидания 1-2 минуты, максимум до 10 минут", Category: ModelCategoryPhoto, RequestCost: 1},
	{ID: "google/nano-banana-pro", Label: "🌟 Nano Banana Pro", Desc: "Фото: Новейшая модель, лучшее качество. Среднее время ожидания 1-2 минуты, максимум до 10 минут", Category: ModelCategoryPhoto, RequestCost: 3},
	{ID: "hug-video", ApiModel: "Qubico/hug-video", Label: "🤗 Обнимашки", Desc: "Видео: оживление фото с обнимашками.", Category: ModelCategoryVideo, RequestCost: 1, TaskType: "image_to_video"},
	{ID: "music-suno", ApiModel: "suno", Label: "🎵 Suno Music", Desc: "Музыка: генерация песни. До 10-15 минут.", Category: ModelCategoryMusic, RequestCost: 1, TaskType: "music"},
	{ID: "chat-gpt-4.1mini", ApiModel: "gpt-4.1-mini", Label: "💬 GPT-4.1 mini", Desc: "Чат-бот: быстрые ответы на текстовые запросы.", Category: ModelCategoryChat, RequestCost: 1, TaskType: "chat"},
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

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		// Копируем update в новую переменную, чтобы горутины не делили одну ссылку
		upd := update
		// Обрабатываем каждое событие асинхронно, чтобы не блокировать очередь
		b.goLimited(func() {
			b.safeHandleUpdate(upd)
		})
	}

	return nil
}

// safeHandleUpdate оборачивает обработку апдейта с восстановлением после паники
func (b *Bot) safeHandleUpdate(update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in update handler: %v", r)
		}
	}()

	if update.Message != nil {
		b.handleMessage(update.Message)
		return
	}
	if update.CallbackQuery != nil {
		b.handleCallback(update.CallbackQuery)
		return
	}
}

// goLimited запускает функцию в горутине с ограничением параллелизма
func (b *Bot) goLimited(fn func()) {
	select {
	case b.concurrencySem <- struct{}{}:
		go func() {
			defer func() {
				<-b.concurrencySem
				if r := recover(); r != nil {
					log.Printf("panic in goroutine: %v", r)
				}
			}()
			fn()
		}()
	default:
		// Если лимит исчерпан, обработаем синхронно, но с защитой
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
			// Проверка доступа и подписки
			if !b.isUserAllowed(user.ID) {
				b.sendErrorMessage(msg.Chat.ID, "Доступ только для админов.")
				return
			}
			if !b.ensureSubscribed(msg.Chat.ID, user.ID) {
				return
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

	// Проверяем белый список админов
	if !b.isUserAllowed(user.ID) {
		b.sendErrorMessage(msg.Chat.ID, "Доступ только для админов.")
		return
	}

	// Не режем альбомы кулдауном, чтобы не терять кадры одной медиагруппы
	if msg.MediaGroupID == "" || msg.Photo == nil {
		if !b.checkCooldown(msg.Chat.ID, user.ID) {
			return
		}
	}

	// Проверяем обязательную подписку
	if !b.ensureSubscribed(msg.Chat.ID, user.ID) {
		return
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
		b.sendBuyMenu(msg.Chat.ID)
	case "invite":
		b.sendInviteInfo(msg.Chat.ID, msg.From.ID)
	case "help":
		b.sendHelpMessage(msg.Chat.ID)
	case "rules":
		b.sendRulesMessage(msg.Chat.ID)
	case "privacy":
		b.sendPrivacyMessage(msg.Chat.ID)
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
	if strings.EqualFold(id, "nano-banana-pro") {
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

func categoryLabel(cat ModelCategory) string {
	switch cat {
	case ModelCategoryPhoto:
		return "📸 Фото"
	case ModelCategoryVideo:
		return "🎬 Видео"
	case ModelCategoryMusic:
		return "🎵 Музыка"
	case ModelCategoryChat:
		return "💬 Чат-бот LLM"
	default:
		return string(cat)
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

Примеры:
• "Убери шум и сделай ярче"
• "Слегка разгладь кожу"
• "Добавь лёгкий теплый фильтр"

Используйте /menu для выбора другой модели.`
	case "google/nano-banana-pro":
		return base + `Принимает фото с подписью и возвращает улучшенное изображение (лучшее качество, но дольше).

Можно отправить до 4 фото за запрос.

Как отправить:
1) Пришлите фото.
2) В подписи опишите, что нужно изменить или улучшить.

Примеры:
• "Сделай кинематографичный цвет"
• "Сделай кожу чище, фон размытым"
• "Убери бликов на лице"

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
` + chatSystemPrompt + `

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

	// Дополнительная проверка доступа и подписки
	if !b.isUserAllowed(userID) {
		b.sendErrorMessage(chatID, "Доступ только для админов.")
		return
	}
	if !b.ensureSubscribed(chatID, userID) {
		return
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

	if err := b.userService.ConsumeQuota(userID, models.QuotaCategoryMusic, requestCost); err != nil {
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
				_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryMusic, requestCost)
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
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryMusic, requestCost)
			b.sendErrorMessage(chatID, "Музыкальная генерация доступна только через Suno.")
			return
		} else {
			resultURL, err = b.generationService.GenerateAudio(text, apiModel, modelOpt.TaskType, "")
		}
		if err != nil {
			// Возврат запроса при ошибке
			_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryMusic, requestCost)
			b.sendErrorMessage(chatID, fmt.Sprintf("Не удалось озвучить: %s", friendlyGenerationError(err)))
			return
		}

		if resultURL != "" {
			caption := fmt.Sprintf("🔊 Аудио готово\nМодель: %s\nСписано: %d музыкальных запрос(ов)", modelOpt.Label, requestCost)
			b.sendAudioResult(chatID, caption, resultURL)
			return
		}

		if taskID != "" {
			b.registerSunoTask(taskID, sunoTask{
				ChatID:      chatID,
				UserID:      userID,
				RequestCost: requestCost,
				ModelLabel:  modelOpt.Label,
				Prompt:      text,
				CreatedAt:   time.Now(),
			})
			return
		}

		// На всякий случай, если нет ни URL, ни taskId
		b.sendErrorMessage(chatID, "Сервис не вернул ссылку и taskId. Попробуйте позже.")
		_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryMusic, requestCost)
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

	b.sunoMu.Lock()
	task, ok := b.sunoTasks[taskID]
	if ok {
		delete(b.sunoTasks, taskID)
	}
	b.sunoMu.Unlock()

	if !ok {
		log.Printf("Suno callback unknown taskID=%s", taskID)
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

	if len(valid) > 1 {
		b.sendText(task.ChatID, "Вот 2 разных варианта песни:")
	}

	for i, url := range valid {
		caption := fmt.Sprintf("🔊 Аудио готово (%d/%d)\nМодель: %s\nСписано: %d музыкальных запрос(ов)", i+1, len(valid), task.ModelLabel, task.RequestCost)
		b.sendAudioResult(task.ChatID, caption, url)
	}
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

func (b *Bot) buildChatSystemPrompt(userID int64) string {
	styleID := b.getUserChatStyle(userID)
	stylePrompt := ""
	for _, st := range chatStyles {
		if st.ID == styleID {
			stylePrompt = st.Prompt
			break
		}
	}
	if stylePrompt == "" {
		return chatSystemPrompt
	}
	return chatSystemPrompt + " " + stylePrompt
}

func (b *Bot) sendSettingsMenu(chatID int64, userID int64) {
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("🗣️ Стиль общения GPT", "settings:style"),
		},
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "menu"),
	))

	text := "⚙️ Настройки\n\nВыберите раздел настроек."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send settings menu: %v", err)
	}
}

func (b *Bot) sendChatStyleMenu(chatID int64, userID int64) {
	current := b.getUserChatStyle(userID)
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(chatStyles); i += 2 {
		btns := []tgbotapi.InlineKeyboardButton{}
		for j := i; j < len(chatStyles) && j < i+2; j++ {
			st := chatStyles[j]
			label := st.Label
			if st.ID == current {
				label = "✅ " + label
			}
			btns = append(btns, tgbotapi.NewInlineKeyboardButtonData(label, "set_style:"+st.ID))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btns...))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "settings"),
	))

	text := "🗣️ Стиль общения GPT\n\nВыберите тон, который будет применяться в системном промпте."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send chat style menu: %v", err)
	}
}

// processVideoGeneration обрабатывает видео-генерацию из фото (image_to_video)
func (b *Bot) processVideoGeneration(chatID int64, userID int64, photoURL string, modelOpt ModelOption) {
	if !b.isUserAllowed(userID) {
		b.sendErrorMessage(chatID, "Бот находится в разработке некоторое время. Доступ только для админов. Подпишитесь на наш канал @AIFaceApps, Там администратор оповестит о запуске бота.")
		return
	}
	if !b.ensureSubscribed(chatID, userID) {
		return
	}

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
		b.sendBuyMenu(chatID)
	case "suno_instr_toggle":
		cur := b.getSunoInstrumental(userID)
		b.setSunoInstrumental(userID, !cur)
		b.sendModelMenu(chatID, userID, ModelCategoryMusic)
	case "suno_voice_toggle":
		cur := b.getSunoVoice(userID)
		if cur == "f" {
			b.setSunoVoice(userID, "m")
		} else {
			b.setSunoVoice(userID, "f")
		}
		b.sendModelMenu(chatID, userID, ModelCategoryMusic)
	case "buy:extras":
		if !b.ensurePaymentsEnabled(chatID) {
			return
		}
		b.sendBuyExtrasMenu(chatID)
	case "buy:sub":
		if !b.ensurePaymentsEnabled(chatID) {
			return
		}
		if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
			b.sendErrorMessage(chatID, "Подписки временно недоступны")
			return
		}
		b.sendBuySubscription(chatID)
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
		b.sendBuyExtrasCategory(chatID, "Текстовые запросы", "buy_package:text")
	case "buy:image":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryPhoto) {
			return
		}
		b.sendBuyExtrasCategory(chatID, "Запросы на изображения", "buy_package:image")
	case "buy:music":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryMusic) {
			return
		}
		b.sendBuyExtrasCategory(chatID, "Музыкальные запросы", "buy_package:music")
	case "buy:video":
		if !b.ensureCategoryEnabled(chatID, ModelCategoryVideo) {
			return
		}
		b.sendBuyExtrasCategory(chatID, "Видео-запросы", "buy_package:video")
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
	case "settings":
		b.sendSettingsMenu(chatID, userID)
	case "settings:style":
		b.sendChatStyleMenu(chatID, userID)
	case "aspect_menu":
		b.sendAspectRatioMenu(chatID, userID)
	case "set_style":
		// default style
		b.setUserChatStyle(userID, "normal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:normal":
		b.setUserChatStyle(userID, "normal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:formal":
		b.setUserChatStyle(userID, "formal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:humor":
		b.setUserChatStyle(userID, "humor")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:informal":
		b.setUserChatStyle(userID, "informal")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:friendly":
		b.setUserChatStyle(userID, "friendly")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:expert":
		b.setUserChatStyle(userID, "expert")
		b.sendChatStyleMenu(chatID, userID)
	case "set_style:empathetic":
		b.setUserChatStyle(userID, "empathetic")
		b.sendChatStyleMenu(chatID, userID)
	case "invite":
		b.sendInviteInfo(chatID, userID)
	case "models_menu":
		b.sendModelMenu(chatID, userID, ModelCategoryPhoto)
	default:
		// Обрабатываем confirm_generation
		if strings.HasPrefix(data, "confirm_generation:") {
			parts := strings.Split(data, ":")
			if len(parts) >= 2 {
				b.confirmGeneration(chatID, userID, parts[1])
			}
		} else if strings.HasPrefix(data, "aspect_set:") {
			ratio := strings.TrimPrefix(data, "aspect_set:")
			b.setUserAspectRatio(userID, ratio)
			b.sendAspectRatioMenu(chatID, userID)
		} else if strings.HasPrefix(data, "models_menu:") {
			cat := ModelCategory(strings.TrimPrefix(data, "models_menu:"))
			b.sendModelMenu(chatID, userID, cat)
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
			b.setUserModel(userID, model)

			info := fmt.Sprintf("Модель выбрана: %s\nКатегория: %s", opt.Label, categoryLabel(opt.Category))
			if opt.Desc != "" {
				info += "\n" + opt.Desc
			}
			if opt.RequestCost > 0 {
				info += fmt.Sprintf("\nРасход: %d запрос(ов) за 1 %s", opt.RequestCost, categoryUnit(opt.Category))
			}
			info += "\n\n" + modelsDescription
			b.sendText(chatID, info)
			b.sendModelMenu(chatID, userID, opt.Category)
		}
	}

	// Отвечаем на callback
	// (пусто — ответили мгновенно в начале)
}

// ensureSubscribed проверяет подписку пользователя на обязательный канал
func (b *Bot) ensureSubscribed(chatID int64, userID int64) bool {
	member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: requiredChannelUsername,
			UserID:             userID,
		},
	})
	if err != nil {
		log.Printf("ensureSubscribed error: %v", err)
		b.sendErrorMessage(chatID, "Не удалось проверить подписку. Попробуйте ещё раз.")
		return false
	}

	status := member.Status
	if status == "creator" || status == "administrator" || status == "member" || (status == "restricted" && member.IsMember) {
		return true
	}

	btn := tgbotapi.NewInlineKeyboardButtonURL("Подписаться", requiredChannelLink)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)
	text := "Для использования бота нужна подписка на канал @AIFaceApps.\nНажмите «Подписаться», затем вернитесь и повторите команду."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("ensureSubscribed send message error: %v", err)
	}
	return false
}

// processGeneration обрабатывает генерацию изображения
func (b *Bot) processGeneration(chatID int64, userID int64, photoURLs []string, genType string, prompt string) {
	// Дополнительная проверка белого списка (защита от прямых вызовов)
	if !b.isUserAllowed(userID) {
		b.sendErrorMessage(chatID, "Бот находится в разработке некоторое время. Доступ только для админов. Подпишитесь на наш канал @AIFaceApps, Там администратор оповестит о запуске бота.")
		return
	}

	// Проверяем обязательную подписку
	if !b.ensureSubscribed(chatID, userID) {
		return
	}

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
	if err := b.userService.ConsumeQuota(userID, models.QuotaCategoryImage, requestCost); err != nil {
		b.sendInsufficientQuotaMessage(chatID, models.QuotaCategoryImage, requestCost, err)
		return
	}

	// Используем photoURL как base64 (в реальной реализации нужно скачать и конвертировать)
	imageList := photoURLs

	// Запускаем генерацию через сервис
	opts := services.GenerationOptions{
		Type:        genType,
		InputImages: imageList,
		InputImage:  imageList[0],
		Prompt:      prompt,
		TokensCost:  requestCost,
		ChatID:      chatID,
		Model:       modelOpt.ID,
	}
	if modelOpt.ID == "google/nano-banana" || modelOpt.ID == "google/nano-banana-pro" {
		opts.AspectRatio = b.getUserAspectRatio(userID)
	}
	if useDef, err := b.userService.IsNanoBananaDefAPIEnabled(); err == nil {
		opts.UseDefAPI = useDef
	}

	req, err := b.generationService.StartGeneration(userID, opts)
	if err != nil {
		// Возвращаем запрос при ошибке
		_ = b.userService.AddExtraQuota(userID, models.QuotaCategoryImage, requestCost)
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
	// Сохраняем сообщение в контекст (до 5 последних)
	b.saveMessageToContext(msg.From.ID, msg.Text)

	modelID := b.getUserModel(msg.From.ID)
	modelOpt, ok := findModelOption(modelID)

	// Аудио-модель: озвучиваем текст сразу
	if ok && modelOpt.Category == ModelCategoryMusic {
		b.processAudioMessage(msg, modelOpt)
		return
	}

	// Чат-модель: выдаём ответ с учётом системного промпта (пока локально)
	if ok && modelOpt.Category == ModelCategoryChat {
		requestCost := modelOpt.RequestCost
		if requestCost < 1 {
			requestCost = 1
		}

		if !b.ensureCategoryEnabled(msg.Chat.ID, ModelCategoryChat) {
			return
		}

		if err := b.userService.ConsumeQuota(msg.From.ID, models.QuotaCategoryText, requestCost); err != nil {
			b.sendInsufficientQuotaMessage(msg.Chat.ID, models.QuotaCategoryText, requestCost, err)
			return
		}

		apiModel := modelOpt.ApiModel
		if apiModel == "" {
			apiModel = modelOpt.ID
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Думаю над ответом...")
		if _, err := b.api.Send(reply); err != nil {
			log.Printf("Failed to send typing message: %v", err)
		}

		b.goLimited(func() {
			messages := []map[string]string{
				{"role": "system", "content": b.buildChatSystemPrompt(msg.From.ID)},
			}
			// добавляем контекст пользователя из redis, если есть
			if ctx, err := b.redisClient.GetContext(msg.From.ID); err == nil && ctx != nil && len(ctx.Messages) > 0 {
				for _, m := range ctx.Messages {
					messages = append(messages, map[string]string{"role": "user", "content": m})
				}
			}
			// текущий запрос
			messages = append(messages, map[string]string{"role": "user", "content": msg.Text})

			resp, err := b.generationService.GenerateChat(apiModel, messages)
			if err != nil {
				// Возврат списанного запроса
				_ = b.userService.AddExtraQuota(msg.From.ID, models.QuotaCategoryText, requestCost)
				b.sendErrorMessage(msg.Chat.ID, fmt.Sprintf("Не удалось ответить: %s", friendlyGenerationError(err)))
				return
			}
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

// handleStart обрабатывает команду /start
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	if isAdmin, err := b.userService.IsUserAdmin(msg.From.ID); err == nil {
		b.setChatCommands(msg.Chat.ID, isAdmin)
	}
	b.sendMainMenu(msg.Chat.ID, msg.From.ID)
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
	b.sendMainMenu(msg.Chat.ID, msg.From.ID)
}

// sendMainMenu отправляет главное меню
func (b *Bot) sendMainMenu(chatID int64, userID int64) {
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
	currentCategory := ""
	var limitLine = "Лимит текущей модели: нет данных"
	if opt, ok := findModelOption(currentModel); ok {
		currentCategory = categoryLabel(opt.Category)
		if currentCategory != "" {
			currentCategory = "(" + currentCategory + ")"
		}

		if quota != nil {
			switch opt.Category {
			case ModelCategoryPhoto:
				limitLine = fmt.Sprintf("Лимит: %d (%d доп)", quota.ImageWeekly, quota.ImageExtra)
			case ModelCategoryVideo:
				limitLine = fmt.Sprintf("Лимит: %d (%d доп)", quota.VideoWeekly, quota.VideoExtra)
			case ModelCategoryMusic:
				limitLine = fmt.Sprintf("Лимит: %d (%d доп)", quota.MusicWeekly, quota.MusicExtra)
			case ModelCategoryChat:
				limitLine = fmt.Sprintf("Лимит: %d (%d доп)", quota.TextDaily, quota.TextExtra)
			}
		}
	}

	text := fmt.Sprintf(`🍌 Ваш айди: %d
⭐ Тип подписки: %s
🧠 Текущая модель: %s %s — %s`,
		userID,
		subLabel,
		currentCategory,
		modelLabel(currentModel),
		limitLine,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Покупка", "buy"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Пригласить друзей", "invite"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧠 Выбрать модель", "models_menu"),
		),
	)
	// Админ-кнопка только для админов
	if isAdmin, err := b.userService.IsUserAdmin(userID); err == nil && isAdmin {
		b.setChatCommands(chatID, true)
	}

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard

	if _, err := b.api.Send(reply); err != nil {
		log.Printf("Failed to send main menu: %v", err)
	}
}

// sendAccount отправляет карточку аккаунта с лимитами (как на скриншоте)
func (b *Bot) sendAccount(chatID int64, userID int64) {
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
		`ID Пользователя: %d
⭐ Тип подписки: %s
📅 Действует до: %s
------------------------------
📝 Текстовые генерации (24 ч): %d
🖼️ Картинок осталось (нед): %d
🎵 Музыка (нед): %d
🎬 Видео(нед): %d
------------------------------
📝 Доп. текстовые генерации: %d
🖼️ Доп. запросы изображений: %d
🎵 Доп. музыка: %d
🎬 Доп. видео: %d`,
		userID,
		subType,
		subEndStr,
		textDaily,
		imageWeekly,
		musicWeekly,
		videoWeekly,
		textExtra,
		imageExtra,
		musicExtra,
		videoExtra,
	)

	msg := tgbotapi.NewMessage(chatID, accountText)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send account card: %v", err)
	}
}

// sendBuyMenu отправляет меню покупки доп. запросов
func (b *Bot) sendBuyMenu(chatID int64) {
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}

	subsEnabled, _ := b.userService.IsSubscriptionsEnabled()

	text := `💰 Покупки

Выберите, что хотите приобрести:`

	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}
	if subsEnabled {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("⭐ Подписка", "buy:sub"))
	}
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("📦 Запросы отдельно", "buy:extras"))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Меню", "menu"),
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
func (b *Bot) sendBuyExtrasMenu(chatID int64) {
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}

	enabled := b.getEnabledCategories(false)
	if len(enabled) == 0 {
		b.sendErrorMessage(chatID, "Все категории отключены администратором")
		return
	}

	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, cat := range enabled {
		switch cat {
		case ModelCategoryChat:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📝 Текстовые", "buy:text")))
		case ModelCategoryPhoto:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🖼️ Изображения", "buy:image")))
		case ModelCategoryMusic:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🎵 Музыка", "buy:music")))
		case ModelCategoryVideo:
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🎬 Видео", "buy:video")))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "buy"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
	))

	text := `💰 Купить доп. запросы

Выберите категорию.`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	reply := tgbotapi.NewMessage(chatID, text)
	reply.ReplyMarkup = keyboard

	if _, err := b.api.Send(reply); err != nil {
		log.Printf("Failed to send buy extras menu: %v", err)
	}
}

func (b *Bot) sendBuySubscription(chatID int64) {
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}
	if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
		b.sendErrorMessage(chatID, "Подписки временно недоступны")
		return
	}

	miniPrice := b.subscriptionPrice("mini")
	startPrice := b.subscriptionPrice("start")
	proPrice := b.subscriptionPrice("pro")

	text := fmt.Sprintf(`⭐ Подписки

✨ Mini — %d ₽ в неделю
• 50 текстовых/24ч
• 30 изображений
• 5 песен
• Скидка 10%% на доп. запросы

🚀 Start — %d ₽ в неделю
• 100 текстовых/24ч
• 70 изображений
• 10 песен
• x2 контекст
• 3 видео
• Скидка 20%% на доп. запросы

👑 Pro — %d ₽ в неделю
• 300 текстовых/24ч
• 150 изображений
• 15 песен
• 7 видео
• 6 стилей общения GPT, x3 контекст, без рекламы
• Скидка 25%% на доп. запросы`, miniPrice, startPrice, proPrice)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐ Mini", "buy_sub:mini"),
			tgbotapi.NewInlineKeyboardButtonData("🚀 Start", "buy_sub:start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Pro", "buy_sub:pro"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "buy"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
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
	if b.paymentService == nil {
		b.sendErrorMessage(chatID, "Платёжный сервис не настроен")
		return
	}
	if !b.ensurePaymentsEnabled(chatID) {
		return
	}
	if ok, _ := b.userService.IsSubscriptionsEnabled(); !ok {
		b.sendErrorMessage(chatID, "Подписки временно недоступны")
		return
	}

	resp, err := b.paymentService.CreateSubscriptionPayment(userID, plan, 7)
	if err != nil {
		b.sendErrorMessage(chatID, fmt.Sprintf("Не удалось создать платеж: %v", err))
		return
	}

	label := strings.Title(plan)
	text := fmt.Sprintf("✅ Вы выбрали подписку %s.\nПерейдите по ссылке для оплаты:\n%s", label, resp.CheckoutURL)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к подпискам", "buy:sub"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "menu"),
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
	user, err := b.userService.GetUserByTelegramID(userID)
	if err != nil {
		b.sendErrorMessage(chatID, "Ошибка при получении данных")
		return
	}

	botUsername := b.api.Self.UserName
	referralLink := fmt.Sprintf("https://t.me/%s?start=%s", botUsername, user.ReferralCode)

	text := fmt.Sprintf(`🎁 Вы будете получать 20%% от покупок ваших рефералов!
Например, пользователь купил 10 доп. запросов, вы получите 2 доп. запроса бесплатно!

Ваша личная реферальная ссылка:
%s
*Нажмите, чтобы скопировать*

👥 Приглашено пользователей: %d`,
		referralLink,
		user.ReferralsCount,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Меню", "menu"),
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
func (b *Bot) sendModelMenu(chatID int64, userID int64, category ModelCategory) {
	enabledCats := b.getEnabledCategories(false)
	if len(enabledCats) == 0 {
		b.sendErrorMessage(chatID, "Все категории отключены администратором")
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
				current = m.ID
				break
			}
		}
	}

	rows := [][]tgbotapi.InlineKeyboardButton{}

	// Ряд с категориями (только включённые)
	catButtons := []tgbotapi.InlineKeyboardButton{}
	for _, cat := range enabledCats {
		label := categoryLabel(cat)
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
		label := m.Label
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
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "menu"),
	))

	text := fmt.Sprintf("Категория: %s\n\nВыберите модель. Она будет применяться по умолчанию для ваших запросов.", categoryLabel(category))
	currentDesc := modelDescription(current)
	if currentDesc != "" {
		text += "\n\nТекущая: " + modelLabel(current) + " — " + currentDesc
		if cost := modelRequestCost(current); cost > 0 {
			unitCategory := category
			if opt, ok := findModelOption(current); ok {
				unitCategory = opt.Category
			}
			text += fmt.Sprintf("\nРасход: %d запрос(ов) за 1 %s", cost, categoryUnit(unitCategory))
		}
		if current == "google/nano-banana" {
			text += "\nМаксимум фото: 2"
		} else if current == "google/nano-banana-pro" {
			text += "\nМаксимум фото: 4"
		}
	}
	if category == ModelCategoryMusic {
		if opt, ok := findModelOption(current); ok && (opt.ID == "music-suno" || strings.Contains(strings.ToLower(opt.ApiModel), "suno")) {
			instrOn := b.getSunoInstrumental(userID)
			instrText := "режим: с голосом"
			instrBtn := "🎹 Режим: с голосом"
			if instrOn {
				instrText = "режим: инструментал (без голоса)"
				instrBtn = "🎹 Режим: инструментал"
			}
			voice := b.getSunoVoice(userID)
			voiceText := "голос: мужской"
			voiceBtn := "🗣️ Голос: мужской"
			if voice == "f" {
				voiceText = "голос: женский"
				voiceBtn = "🗣️ Голос: женский"
			}
			text += "\n" + instrText + "\n" + voiceText
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(instrBtn, "suno_instr_toggle"),
			))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(voiceBtn, "suno_voice_toggle"),
			))
		}
	}
	if category == ModelCategoryPhoto && (current == "google/nano-banana" || current == "google/nano-banana-pro") {
		ratio := b.getUserAspectRatio(userID)
		label := "📐 Формат: " + ratio
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "aspect_menu"),
		))
	}
	text += "\n\n" + modelsDescription

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send model menu: %v", err)
	}
}

func (b *Bot) sendHelpMessage(chatID int64) {
	text := `📋 Доступные команды:

/start - Начать
/menu - Главное меню
/buy - Купить доп. запросы
/rules - Правила использования
/invite - Пригласить друзей

📸 Как использовать:
1. Отправьте фото
2. Опишите желаемые изменения
3. Дождитесь результата

💰 Расход: 1 запрос`

	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

func (b *Bot) sendAspectRatioMenu(chatID int64, userID int64) {
	current := b.getUserAspectRatio(userID)
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			aspectOptionButton("Пейзаж 16:9", "16:9", current),
			aspectOptionButton("Портрет 9:16", "9:16", current),
		},
		{
			aspectOptionButton("Аватар 1:1", "1:1", current),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "models_menu"),
		},
	}
	text := "📐 Выберите соотношение сторон для Nano Banana"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
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

// sendPrivacyMessage отправляет ссылку на политику конфиденциальности
func (b *Bot) sendPrivacyMessage(chatID int64) {
	text := "🔒 Политика конфиденциальности: https://telegra.ph/Politika-Konfidencialnosti-01-14-87\n\n📜 Пользовательское соглашение: https://telegra.ph/Polzovatelskoe-soglashenie-Usloviya-EHkspluatacii-i-Obsluzhivaniya-01-14"
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send privacy message: %v", err)
	}
}

func (b *Bot) sendRulesMessage(chatID int64) {
	text := `📜 Правила бота:

1) 18+ контент запрещён к генерации и отправке.
2) Нельзя загружать обнажёнку, жестокость, спам или нелегальный контент.
3) Не нарушайте авторские права и чужие личные данные.
4) Соблюдайте лимиты запросов и уважайте инфраструктуру.
5) Используйте /menu для выбора модели и следуйте её инструкции.
6) Запрещены любые оскорбления третьих лиц в песнях, фото, чат.

Нарушения могут привести к блокировке. Без возврата средств.`
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

	total := numberAsInt(stats["total_requests"])
	completed := numberAsInt(stats["completed_requests"])
	failed := numberAsInt(stats["failed_requests"])
	processing := numberAsInt(stats["processing_requests"])
	successRate := numberAsFloat(stats["success_rate"])
	avgTime := numberAsFloat(stats["avg_processing_time_seconds"])

	text := fmt.Sprintf(`📊 Статистика бота:

🎨 Всего запросов: %d
✅ Успешных: %d
❌ Ошибок: %d
🔄 В процессе: %d
📈 Успешность: %.1f%%`,
		total,
		completed,
		failed,
		processing,
		successRate,
	)

	if avgTime > 0 {
		text += fmt.Sprintf("\n⏱️ Среднее время: %.1f сек", avgTime)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	_, err = b.api.Send(msg)
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
	statusEmoji, statusText := b.statusInfo(req.Status)
	typeLabel := "Редактирование фото"
	displayTokens := req.TokensUsed
	if req.Status == "failed" {
		displayTokens = 0 // показываем 0 при ошибке, даже если списание было
	}
	baseText := fmt.Sprintf(`%s Статус генерации:

📝 Тип: %s
🔄 Статус: %s
🎫 Использовано запросов: %d`,
		statusEmoji,
		typeLabel,
		statusText,
		displayTokens,
	)

	// Если есть картинка в data URL — отправляем как фото, чтобы не ловить "Request Entity Too Large"
	if req.Status == "completed" && req.OutputImage != nil && *req.OutputImage != "" {
		output := *req.OutputImage
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
				// запасной вариант — текстом без ссылки
				b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
			}
			return
		}

		// Иной формат — отправляем текстом без ссылки
		b.sendText(chatID, truncate(baseText+"\n\n🖼️ Результат недоступен", 3800))
		return
	}

	if req.Status == "failed" && req.ErrorMsg != nil && *req.ErrorMsg != "" {
		friendly := friendlyGenerationError(fmt.Errorf(*req.ErrorMsg))
		// Возврат запросов при неудачной генерации
		if req.TokensUsed > 0 && req.UserID != 0 {
			if err := b.userService.AddExtraQuota(req.UserID, models.QuotaCategoryImage, req.TokensUsed); err != nil {
				log.Printf("refund on failed generation error: %v", err)
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
	const chunkSize = 4000
	for len(text) > 0 {
		chunk := text
		if len(chunk) > chunkSize {
			chunk = text[:chunkSize]
			text = text[chunkSize:]
		} else {
			text = ""
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
