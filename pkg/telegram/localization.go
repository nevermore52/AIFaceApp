package telegram

type Lang string

const (
	LangRU Lang = "ru"
	LangEN Lang = "en"
)

type Localization struct {
	// Commands
	CmdStart    string
	CmdMenu     string
	CmdAccount  string
	CmdBuy      string
	CmdInvite   string
	CmdRules    string
	CmdPrivacy  string
	CmdSettings string
	CmdAdmin    string

	// Main menu
	MenuTitle           string
	MenuYourID          string
	MenuSubscription    string
	MenuCurrentModel    string
	MenuLimit           string
	MenuLimitFormat     string
	MenuBuyBtn          string
	MenuInviteBtn       string
	MenuSelectModelBtn  string
	MenuGenPhotoBtn     string
	MenuGenMusicBtn     string
	MenuInviteFriendBtn string
	MenuAccountBtn      string
	MenuSettingsBtn     string
	MenuHelpBtn         string

	// Account
	AccountUserID       string
	AccountSubscription string
	AccountValidUntil   string
	AccountTextDaily    string
	AccountImagesWeekly string
	AccountMusicWeekly  string
	AccountVideoWeekly  string
	AccountExtraText    string
	AccountExtraImages  string
	AccountExtraMusic   string
	AccountExtraVideo   string

	// Buy menu
	BuyTitle           string
	BuySelectAction    string
	BuyConsentNote     string
	BuySubscriptionBtn string
	BuyExtrasBtn       string
	BuyBackBtn         string
	BuyMenuBtn         string

	// Subscriptions
	SubsTitle          string
	SubsMini           string
	SubsStart          string
	SubsPro            string
	SubsPerWeek        string
	SubsTextDaily      string
	SubsImages         string
	SubsSongs          string
	SubsVideos         string
	SubsDiscount       string
	SubsContext        string
	SubsNoAds          string
	SubsChatStyles     string
	SubsTextModelsMini string
	SubsTextModelsHi   string
	SubsUnavailable    string
	SubsPaymentCreated string
	SubsBackToSubs     string

	// Extras
	ExtrasTitle       string
	ExtrasSelectCat   string
	ExtrasText        string
	ExtrasImages      string
	ExtrasMusic       string
	ExtrasVideo       string
	ExtrasAllDisabled string
	ExtrasTexts       string
	ExtrasImage       string
	ExtrasMusico      string
	ExtrasVideos      string

	// Invite
	InviteTitle    string
	InviteExample  string
	InviteLink     string
	InviteCopyHint string
	InviteCount    string

	// Settings
	SettingsTitle     string
	SettingsChatStyle string
	SettingsLanguage  string
	SettingsBackBtn   string
	SettingsSelect    string

	// Language
	LangTitle   string
	LangRussian string
	LangEnglish string
	LangChanged string

	// Chat styles
	StyleNormal           string
	StyleFormal           string
	StyleHumor            string
	StyleInformal         string
	StyleFriendly         string
	StyleExpert           string
	StyleEmpathetic       string
	StyleSelectTitle      string
	StyleCurrentPrefix    string
	StylePromptNormal     string
	StylePromptFormal     string
	StylePromptHumor      string
	StylePromptInformal   string
	StylePromptFriendly   string
	StylePromptExpert     string
	StylePromptEmpathetic string
	ChatSystemPrompt      string
	ChatSystemLangHint    string
	ChatStyleProOnly      string

	// Models
	ModelsCategory    string
	ModelsSelect      string
	ModelsCurrent     string
	ModelsCost        string
	ModelsMaxPhotos   string
	ModelsDescription string
	ModelsCatPhoto    string
	ModelsCatVideo    string
	ModelsCatMusic    string
	ModelsCatChat     string
	InstrNanoBanana   string
	InstrTextModels   string
	InstrSunoMusic    string
	InstrHugVideo     string

	// Photo models
	ModelNanoBanana    string
	ModelNanoBananaPro string
	ModelHugVideo      string
	ModelSunoMusic     string
	ModelGeminiFlash   string
	ModelGPT5Mini      string
	ModelGPT5Nano      string
	ModelGPT41Mini     string

	// Photo instructions
	PhotoReceived   string
	PhotoAddCaption string
	PhotoExamples   string
	PhotoExample1   string
	PhotoExample2   string
	PhotoExample3   string
	PhotoExample4   string

	// Generation
	GenStarted     string
	GenModel       string
	GenDeducted    string
	GenWaiting     string
	GenReady       string
	GenFailed      string
	GenUnavailable string

	// Music
	MusicStarted     string
	MusicMode        string
	MusicModeVocal   string
	MusicModeInstr   string
	MusicVoice       string
	MusicVoiceMale   string
	MusicVoiceFemale string
	MusicReady       string
	MusicVariants    string
	MusicFailed      string
	MusicSelectModel string

	// Errors
	ErrPrefix                   string
	ErrAccessDenied             string
	ErrNotSubscribed            string
	ErrCategoryDisabled         string
	ErrInsufficientQuota        string
	ErrNeedQuota                string
	ErrServiceError             string
	ErrDefAPIEmptyBilled        string
	ErrUnknownCommand           string
	ErrEmptyRequest             string
	ErrNoImage                  string
	ErrPaymentFailed            string
	ErrPaymentNotSetup          string
	ErrCategoryCheck            string
	ErrPaymentsCheck            string
	ErrPaymentsDisabled         string
	ErrAdminCheck               string
	ErrAdminDenied              string
	ErrSubscriptionsUnavailable string
	ErrInvalidPackage           string
	ErrUnknownModel             string
	ErrModelRequiresMini        string

	// Cooldown
	CooldownWait   string
	CooldownBuySub string

	// Subscription check
	SubCheckTitle   string
	SubCheckJoin    string
	SubCheckDone    string
	SubCheckMessage string

	// Rules
	RulesTitle   string
	RulesContent string

	// Privacy
	PrivacyPolicy string
	PrivacyTerms  string

	// Help
	HelpTitle    string
	HelpCommands string
	HelpUsage    string
	HelpCost     string

	// Admin
	AdminPanel            string
	AdminSelectAction     string
	AdminStats            string
	AdminUsers            string
	AdminCategories       string
	AdminPayments         string
	AdminBalance          string
	AdminNanoAPI          string
	AdminSubs             string
	AdminHelp             string
	AdminMenu             string
	AdminCategoriesTitle  string
	AdminCategoryEnabled  string
	AdminCategoryDisabled string
	PaymentsStatusTitle   string
	PaymentsEnabled       string
	PaymentsDisabled      string
	PaymentsEnableBtn     string
	PaymentsDisableBtn    string
	SubsStatusTitle       string
	SubsEnabled           string
	SubsDisabled          string
	SubsEnableBtn         string
	SubsDisableBtn        string
	BuyPackageTitle       string
	BuyPackageLabelText   string
	BuyPackageLabelImage  string
	BuyPackageLabelMusic  string
	BuyPackageLabelVideo  string
	BuyPackageBackBtn     string

	// Aspect ratio
	AspectTitle     string
	AspectLandscape string
	AspectPortrait  string
	AspectSquare    string

	// Common
	WelcomeText string
	BackBtn     string
	MenuBtn     string
	RequestUnit string
	PhotoUnit   string
	VideoUnit   string
	TrackUnit   string
	QueryUnit   string

	// Additional fields for localization
	ErrAdminRights               string
	ErrCheckSubscription         string
	ErrPaymentNotConfigured      string
	ErrCreatePayment             string
	ErrGetStats                  string
	ErrAllCategoriesDisabled     string
	ErrInvalidUserID             string
	ErrInvalidDays               string
	ErrInvalidPlan               string
	ErrSetSubscription           string
	ErrRemoveSubscription        string
	ErrUnknownAdminCommand       string
	ErrModelSubscriptionRequired string
	ErrChatResponse              string
	AdminSubSetUsage             string
	AdminSubRemoveUsage          string
	SubscriptionGranted          string
	SubscriptionRemoved          string
	AdminStatsTitle              string
	AdminStatsTotal              string
	AdminStatsCompleted          string
	AdminStatsFailed             string
	AdminStatsProcessing         string
	AdminStatsSuccessRate        string
	AdminStatsAvgTime            string
	ModelSelected                string
	ModelCategory                string
	ModelCost                    string
	ModelsTitle                  string
	ModelsNotSelected            string
	SubscribeBtn                 string
	SubscribeRequired            string
	Thinking                     string
	UserPrefix                   string
	BotPrefix                    string
	ContextPrefix                string
	GenServiceError              string
	QuotaTextRequests            string
	QuotaImageRequests           string
	QuotaMusicRequests           string
	QuotaVideoRequests           string
	QuotaRequests                string
	BuyExtraQuota                string
	BackToSubscriptions          string
	SubscriptionSelected         string
	PaymentLink                  string
}

var locRU = Localization{
	// Commands
	CmdStart:            "Начать",
	CmdMenu:             "Главное меню",
	CmdAccount:          "Аккаунт и лимиты",
	CmdBuy:              "Купить (подписку, генерации)",
	CmdInvite:           "Пригласить друзей",
	CmdRules:            "Правила использования",
	MenuTitle:           "🍌  Ваш айди: %d\n⭐  Тип подписки: %s\n🧠  Текущая ИИ модель: %s %s %s",
	MenuLimit:           "Лимит текущей модели: нет данных",
	MenuLimitFormat:     "Лимит: %d (%d доп)",
	MenuBuyBtn:          "💰 Покупка",
	MenuInviteBtn:       "👥 Пригласить друзей",
	MenuSelectModelBtn:  "🧠 Выбрать модель",
	MenuGenPhotoBtn:     "🖼️ Генерация фото",
	MenuGenMusicBtn:     "🎸 Генерация песни",
	MenuInviteFriendBtn: "👥 Пригласить друга",
	MenuAccountBtn:      "🪪 Мой аккаунт",
	MenuSettingsBtn:     "⚙️ Настройки",
	MenuHelpBtn:         "🆘 Помощь",
	MenuBtn:             "🏠 Меню",
	BackBtn:             "◀️ Назад",

	// Account
	AccountUserID:       "ID Пользователя: %d",
	AccountSubscription: "⭐ Тип подписки: %s",
	AccountValidUntil:   "📅 Действует до: %s",
	AccountTextDaily:    "📝 Текстовые генерации (24 ч): %d",
	AccountImagesWeekly: "🖼️ Фото осталось: %d",
	AccountMusicWeekly:  "🎵 Музыка: %d",
	AccountVideoWeekly:  "🎬 Видео: %d",
	AccountExtraText:    "📝 Доп. текстовые генерации: %d",
	AccountExtraImages:  "🖼️ Доп. изображения: %d",
	AccountExtraMusic:   "🎵 Доп. музыка: %d",
	AccountExtraVideo:   "🎬 Доп. видео: %d",

	// Buy menu
	BuyTitle:           "💳 Купить",
	BuySelectAction:    "Выберите, что хотите купить:",
	BuyConsentNote:     "Оплачивая, вы принимаете Политику конфиденциальности и Пользовательское соглашение (/privacy).",
	BuySubscriptionBtn: "⭐ Подписку",
	BuyExtrasBtn:       "💰 Генерации",
	BuyBackBtn:         "◀️ Назад",
	BuyMenuBtn:         "🏠 Меню",

	// Subscriptions
	SubsTitle:          "⭐ Подписки",
	SubsMini:           "✨ Mini — %d ₽ в неделю",
	SubsStart:          "🚀 Start — %d ₽ в неделю",
	SubsPro:            "👑 Pro — %d ₽ в неделю",
	SubsPerWeek:        "в неделю",
	SubsTextDaily:      "текстовых/24ч",
	SubsImages:         "изображений",
	SubsSongs:          "песен",
	SubsVideos:         "видео",
	SubsDiscount:       "Скидка %d%% на доп. запросы",
	SubsContext:        "x%d контекст",
	SubsNoAds:          "без рекламы",
	SubsChatStyles:     "%d стилей общения GPT",
	SubsTextModelsMini: "Текстовые модели: GPT-5 mini, GPT-5 nano",
	SubsTextModelsHi:   "Текстовые модели: Gemini 3 Flash, GPT-5 mini, GPT-5 nano",
	SubsUnavailable:    "Подписки временно недоступны",
	SubsPaymentCreated: "✅ Вы выбрали подписку %s.\nПерейдите по ссылке для оплаты:\n%s",
	SubsBackToSubs:     "◀️ Назад к подпискам",

	// Extras
	ExtrasTitle:       "💰 Купить доп. запросы",
	ExtrasSelectCat:   "Выберите категорию.",
	ExtrasText:        "📝 Текст",
	ExtrasImages:      "🖼️ Изображения",
	ExtrasMusic:       "🎵 Музыка",
	ExtrasVideo:       "🎬 Видео",
	ExtrasTexts:       "📝 Текстовые запросы",
	ExtrasImage:       "🖼️ Генерации изображений",
	ExtrasMusico:      "🎵 Генерации музыки",
	ExtrasVideos:      "🎬 Доп. видео",
	ExtrasAllDisabled: "Все категории отключены администратором",

	// Invite
	InviteTitle:    "🎁 Вы будете получать 20% от покупок ваших рефералов!",
	InviteExample:  "Например, пользователь купил 10 доп. запросов, вы получите 2 доп. запроса бесплатно!",
	InviteLink:     "Ваша личная реферальная ссылка:",
	InviteCopyHint: "*Нажмите, чтобы скопировать*",
	InviteCount:    "👥 Приглашено пользователей:",

	// Settings
	SettingsTitle:     "⚙️ Настройки",
	SettingsChatStyle: "💬 Стиль общения",
	SettingsLanguage:  "🌐 Язык / Language",
	SettingsBackBtn:   "◀️ Меню",
	SettingsSelect:    "Выберите раздел настроек.",

	// Language
	LangTitle:   "🌐 Выберите язык / Select language",
	LangRussian: "🇷🇺 Русский",
	LangEnglish: "🇬🇧 English",
	LangChanged: "✅ Язык изменён на русский",

	// Chat styles
	StyleNormal:           "🙂 Обычный",
	StyleFormal:           "📘 Формальный",
	StyleHumor:            "😂 Юмористический",
	StyleInformal:         "😎 Неформальный",
	StyleFriendly:         "🤝 Дружеский",
	StyleExpert:           "🧠 Экспертный",
	StyleEmpathetic:       "❤️ Сочувствующий",
	StyleSelectTitle:      "Выберите стиль общения с GPT:",
	StyleCurrentPrefix:    "Текущий стиль:",
	StylePromptNormal:     "Отвечай нейтрально и лаконично.",
	StylePromptFormal:     "Держи деловой и уважительный тон.",
	StylePromptHumor:      "Отвечай с лёгким дружелюбным юмором без сарказма и оскорблений.",
	StylePromptInformal:   "Пиши в расслабленном разговорном тоне, без канцелярита и без грубости.",
	StylePromptFriendly:   "Будь тёплым и поддерживающим, но не многословным.",
	StylePromptExpert:     "Давай чёткие, структурированные и экспертные ответы по делу.",
	StylePromptEmpathetic: "Отвечай с эмпатией и поддержкой.",
	ChatSystemPrompt:      "Ты — умный ассистент Telegram-бота. Отвечай кратко и по делу.",
	ChatSystemLangHint:    "Если пользователь пишет по-русски, отвечай на русском. Если по-английски — отвечай на английском.",
	ChatStyleProOnly:      "Стили общения доступны только с подпиской Pro и выше.",

	ModelsCurrent:     "Текущая",
	ModelsCategory:    "%s",
	ModelsSelect:      "Выберите модель:",
	ModelsCost:        "Расход: %d %s",
	ModelsMaxPhotos:   "Максимум фото: %d",
	ModelsDescription: "",
	ModelsCatPhoto:    "📸 Фото",
	ModelsCatVideo:    "🎬 Видео",
	ModelsCatMusic:    "🎵 Музыка",
	ModelsCatChat:     "💬 Текст",
	InstrNanoBanana:   "В 1 сообщении отправьте 1 или более фото, в подписи укажите что хотите изменить (промпт).",
	InstrTextModels:   "Отправьте любое сообщение в чат.",
	InstrSunoMusic:    "Отправьте в чат описание музыки, которую хотите создать.",
	InstrHugVideo:     "Отправьте 1 фото с двумя или более людьми и ожидайте.",

	// Aspect ratio
	AspectTitle:     "🔲 Формат",
	AspectLandscape: "Альбомный 16:9",
	AspectPortrait:  "Портретный 9:16",
	AspectSquare:    "Квадрат 1:1",

	// Photo models descriptions
	ModelNanoBanana:    "Cреднее качество, плохо работает с двумя фотографиями. Среднее время ожидания 1 минута.",
	ModelNanoBananaPro: "Новейшая модель, лучшее качество. Среднее время ожидания 1-2 минуты.",
	ModelHugVideo:      "Видео: оживление фото с обнимашками.",
	ModelSunoMusic:     "Генерация песни. Ожидание 5-10 минут.",
	ModelGeminiFlash:   "Текст: быстрые ответы. Доступно с подпиской Start+.",
	ModelGPT5Mini:      "Текст: быстрые ответы. Доступно с подпиской Mini+.",
	ModelGPT5Nano:      "Текст: дольше ответы. Доступно с подпиской Mini+.",
	ModelGPT41Mini:     "Текст: быстрые ответы на текстовые запросы.",

	// Photo instructions
	PhotoReceived:   "📷 Фото получено!",
	PhotoAddCaption: "Пожалуйста, отправьте фото ещё раз, но с подписью — опишите, что хотите изменить.",
	PhotoExamples:   "Примеры:",
	PhotoExample1:   "\"Сделай короткую стрижку\"",
	PhotoExample2:   "\"Примерь красное платье\"",
	PhotoExample3:   "\"Измени цвет волос на блонд\"",
	PhotoExample4:   "\"Удали 'определенный' объект с фото\"",

	// Generation
	GenStarted:     "🔄 Генерация запущена",
	GenModel:       "Модель: %s",
	GenDeducted:    "Списано: %d запрос(ов)",
	GenWaiting:     "Ожидание до %s",
	GenReady:       "✅ Готово!",
	GenFailed:      "❌ Ошибка генерации",
	GenUnavailable: "🖼️ Результат недоступен",

	// Music
	MusicStarted:     "🔄 Озвучка запущена",
	MusicMode:        "Режим:",
	MusicModeVocal:   "с голосом",
	MusicModeInstr:   "инструментал (без голоса)",
	MusicVoice:       "Голос:",
	MusicVoiceMale:   "мужской",
	MusicVoiceFemale: "женский",
	MusicReady:       "🔊 Аудио готово",
	MusicVariants:    "Вот 2 разных варианта песни:",
	MusicFailed:      "Не удалось сгенерировать песню",
}

var locEN = Localization{
	// ... (other fields remain the same)
	ErrDefAPIEmptyBilled: "❌ Generation error: An error occurred; you may have violated the bot rules.",
	// ... (other fields remain the same)
	ErrModelRequiresMini: "This model requires a Start subscription or higher",
	MenuTitle:            "🍌 Your ID: %d\n⭐ Subscription: %s\n🧠 Current AI model: %s %s %s",
	MenuLimit:            "Current model limit: no data",
	MenuLimitFormat:      "Limit: %d (%d extra)",
	ModelsCategory:       "%s",
	ModelsSelect:         "Select a model:",

	// Cooldown
	CooldownWait:   "⏳ Please wait %d more seconds before next request.",
	CooldownBuySub: "Or buy any subscription.",

	// Subscription check
	SubCheckTitle:   "📢 To use the bot, subscribe to our channel:",
	SubCheckJoin:    "📲 Subscribe",
	SubCheckDone:    "✅ I subscribed",
	SubCheckMessage: "To use the bot you need to subscribe to @AIFaceApps.\nClick “Subscribe”, then return and repeat the command.",

	// Rules
	RulesTitle:   "📜 Bot rules:",
	RulesContent: "1) 18+ content is prohibited.\n2) No nudity, violence, spam or illegal content.\n3) Do not violate copyrights or personal data.\n4) Respect request limits and infrastructure.\n5) Use /menu to select a model and follow its instructions.\n6) Insults towards third parties in songs, photos, chat are prohibited.\n\nViolations may result in a ban. No refunds.",

	// Privacy
	PrivacyPolicy: "🔒 Privacy Policy:",
	PrivacyTerms:  "📜 Terms of Service:",

	// Help
	HelpTitle:    "📋 Available commands:",
	HelpCommands: "/start - Start\n/menu - Main menu\n/buy - Buy extra requests\n/rules - Terms of use\n/invite - Invite friends",
	HelpUsage:    "📸 How to use:\n1. Send a photo\n2. Describe desired changes\n3. Wait for the result",
	HelpCost:     "💰 Cost: 1 request",

	// Admin
	AdminPanel:            "👑 Admin Panel",
	AdminSelectAction:     "Select action:",
	AdminStats:            "📊 Statistics",
	AdminUsers:            "👥 Users",
	AdminCategories:       "⚙️ Categories",
	AdminPayments:         "💳 Payments",
	AdminBalance:          "💰 API Balance",
	AdminNanoAPI:          "🍌 Nano Banana API",
	AdminSubs:             "🔒 Subscriptions",
	AdminHelp:             "ℹ️ Help",
	AdminMenu:             "🏠 Menu",
	AdminCategoriesTitle:  "⚙️ Categories:",
	AdminCategoryEnabled:  "enabled",
	AdminCategoryDisabled: "disabled",
	PaymentsStatusTitle:   "💳 Payments: %s",
	PaymentsEnabled:       "✅ Enabled",
	PaymentsDisabled:      "❌ Disabled",
	PaymentsEnableBtn:     "Enable payments",
	PaymentsDisableBtn:    "Disable payments",
	SubsStatusTitle:       "🔒 Subscriptions: %s",
	SubsEnabled:           "✅ Enabled",
	SubsDisabled:          "❌ Disabled",
	SubsEnableBtn:         "Enable subscriptions",
	SubsDisableBtn:        "Disable subscriptions",
	BuyPackageTitle:       "✅ You selected package %s: %d pcs.\nFollow the link to pay:\n%s",
	BuyPackageLabelText:   "text requests",
	BuyPackageLabelImage:  "image requests",
	BuyPackageLabelMusic:  "music requests",
	BuyPackageLabelVideo:  "video requests",
	BuyPackageBackBtn:     "◀️ Back to categories",
	// Aspect ratio
	AspectTitle:     "Format",
	AspectLandscape: "Landscape 16:9",
	AspectPortrait:  "Portrait 9:16",
	AspectSquare:    "Square 1:1",

	// Common
	BackBtn:     "◀️ Back",
	MenuBtn:     "🏠 Menu",
	RequestUnit: "request(s)",
	PhotoUnit:   "photo",
	VideoUnit:   "video",
	TrackUnit:   "track",
	QueryUnit:   "query",

	// Additional fields for localization
	ErrAdminRights:               "You do not have administrator rights",
	ErrCheckSubscription:         "Could not verify subscription. Try again.",
	ErrPaymentNotConfigured:      "Payment service is not configured",
	ErrCreatePayment:             "Failed to create payment: %v",
	ErrGetStats:                  "Error getting statistics",
	ErrAllCategoriesDisabled:     "All categories are disabled by administrator",
	ErrInvalidUserID:             "Invalid user_id",
	ErrInvalidDays:               "Invalid days",
	ErrInvalidPlan:               "Plan must be mini, start and higher",
	ErrSetSubscription:           "Failed to grant subscription: %v",
	ErrRemoveSubscription:        "Failed to remove subscription: %v",
	ErrUnknownAdminCommand:       "Unknown admin command. Use /admin help",
	ErrModelSubscriptionRequired: "Model available only with Mini subscription or higher",
	ErrChatResponse:              "Failed to respond: %s",
	AdminSubSetUsage:             "Usage: /admin sub_set <user_id> <mini|start|pro> <days>",
	AdminSubRemoveUsage:          "Usage: /admin sub_remove <user_id>",
	SubscriptionGranted:          "✅ Subscription %s granted to user %d for %d days",
	SubscriptionRemoved:          "✅ Subscription removed for user %d",
	AdminStatsTitle:              "📊 Bot Statistics:",
	AdminStatsTotal:              "🎨 Total requests: %d",
	AdminStatsCompleted:          "✅ Successful: %d",
	AdminStatsFailed:             "❌ Failed: %d",
	AdminStatsProcessing:         "🔄 Processing: %d",
	AdminStatsSuccessRate:        "📈 Success rate: %.1f%%",
	AdminStatsAvgTime:            "⏱️ Average time: %.1f sec",
	ModelSelected:                "Model selected: %s",
	ModelCategory:                "Category: %s",
	ModelCost:                    "Cost: %d request(s) per 1 %s",
	ModelsTitle:                  "🤖 Current model: %s\n\nSelect category:",
	ModelsNotSelected:            "Not selected",
	SubscribeBtn:                 "Subscribe",
	SubscribeRequired:            "You need to subscribe to @AIFaceApps channel to use the bot.\nClick \"Subscribe\", then return and repeat the command.",
	Thinking:                     "Thinking...",
	UserPrefix:                   "User:",
	BotPrefix:                    "Bot:",
	ContextPrefix:                "Here are your previous requests and my responses:\n",
	GenServiceError:              "Service returned an error. Read the bot rules /rules and try to rephrase your request.",
	QuotaTextRequests:            "text requests",
	QuotaImageRequests:           "image requests",
	QuotaMusicRequests:           "music requests",
	QuotaVideoRequests:           "video requests",
	QuotaRequests:                "requests",
	BuyExtraQuota:                "💰 Buy extra requests",
	BackToSubscriptions:          "◀️ Back to subscriptions",
	SubscriptionSelected:         "✅ You selected %s subscription.\nFollow the link to pay:\n%s",
	PaymentLink:                  "Follow the link to pay:\n%s",
}

func GetLocalization(lang string) *Localization {
	if lang == "en" {
		return &locEN
	}
	return &locRU
}
