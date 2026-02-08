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
	SubsNoChannel      string
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
	WelcomeText         string
	StartPromoTitle     string
	StartPromoCountdown string
	BackBtn             string
	MenuBtn             string
	RequestUnit         string
	PhotoUnit           string
	VideoUnit           string
	TrackUnit           string
	QueryUnit           string

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
	CmdPrivacy:          "Политика конфиденциальности",
	CmdSettings:         "Настройки",
	CmdAdmin:            "Админ",
	MenuTitle:           "🍌  Ваш айди: %d\n⭐  Тип подписки: %s\n🧠  Текущая ИИ модель: %s %s %s",
	MenuYourID:          "Ваш айди: %d",
	MenuSubscription:    "Тип подписки: %s",
	MenuCurrentModel:    "Текущая ИИ модель: %s",
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

	// Правила бота
	RulesTitle:   "📜 Правила сервиса:",
	RulesContent: "1) Контент 18+ запрещен к генерации и отправке в том числе фото в нижнем белье и купальниках.\n2) Контент не должен содержать обнаженной натуры, насилия, спама или незаконного контента.\n3) Контент не должен нарушать авторские права или персональные данные.\n4) Запрещены оскорбления в адрес третьих лиц в песнях, фотографиях, чате. При запросах нарушающих правила сервиса, возврат средств не производится.",

	// Buy menu
	BuyTitle:             "💳 Купить",
	BuySelectAction:      "Выберите, что хотите купить:",
	BuyConsentNote:       "Оплачивая, вы принимаете Политику конфиденциальности и Пользовательское соглашение (/privacy).",
	BuySubscriptionBtn:   "⭐ Подписку",
	BuyExtrasBtn:         "💰 Генерации",
	BuyBackBtn:           "◀️ Назад",
	BuyMenuBtn:           "🏠 Меню",
	BuyPackageTitle:      "✅ Вы выбрали пакет %s: %d шт.\nПерейдите по ссылке для оплаты:\n%s",
	BuyPackageLabelText:  "текстовых запросов",
	BuyPackageLabelImage: "генераций фото",
	BuyPackageLabelMusic: "генераций музыки",
	BuyPackageLabelVideo: "генераций видео",
	BuyPackageBackBtn:    "◀️ Назад к категориям",

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
	SubsNoChannel:      "Без обязательной подписки на канал",
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
	InviteExample:  "Например, пользователь купил 10 доп. запросов, вы получите 2 доп. запроса бесплатно! Не действует на подписки.",
	InviteLink:     "**Ваша личная реферальная ссылка**:",
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
	ModelNanoBanana:    "Cреднее качество, быстрая генерация, плохо работает с текстом на фото. Среднее время ожидания 10-30 сек.",
	ModelNanoBananaPro: "Новейшая модель, лучшая проработка. Среднее время ожидания 30сек-2 минуты.",
	ModelHugVideo:      "Видео: оживление фото с обнимашками.",
	ModelSunoMusic:     "Генерация песни. Ожидание 2-10 минут.",
	ModelGeminiFlash:   "Текст: быстрые ответы. Доступно с подпиской Start+.",
	ModelGPT5Mini:      "Текст: быстрые ответы. Доступно с подпиской Mini+.",
	ModelGPT5Nano:      "Текст: дольше ответы. Доступно с подпиской Mini+.",
	ModelGPT41Mini:     "Текст: быстрые ответы на текстовые запросы.",

	// Photo instructions
	PhotoReceived:   "📷 Фото получено!",
	PhotoAddCaption: "Пожалуйста, отправьте фото ещё раз, но с подписью — опишите, что хотите изменить.",
	PhotoExamples:   "Примеры:",
	PhotoExample1:   "\"Сделай короткую стрижку полубокс\"",
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
	MusicSelectModel: "Выберите музыкальную модель:",

	ErrPrefix:                   "❌ Ошибка: %s",
	ErrAccessDenied:             "Доступ только для админов.",
	ErrNotSubscribed:            "Чтобы использовать бота, подпишитесь на @AIFaceApps.",
	ErrCategoryDisabled:         "Категория временно недоступна",
	ErrInsufficientQuota:        "Недостаточно %s. Нужно %d.",
	ErrNeedQuota:                "Недостаточно %s",
	ErrServiceError:             "Сервис вернул ошибку. Прочитайте правила нашего бота /rules и попробуйте переформулировать ваш запрос и отправьте его снова.",
	ErrDefAPIEmptyBilled:        "❌ Ошибка генерации: возможно, вы нарушили правила бота. Попробуйте отправить запрос снова.",
	ErrUnknownCommand:           "❓ Неизвестная команда. Используйте /help для получения списка доступных команд.",
	ErrEmptyRequest:             "Пустой запрос",
	ErrNoImage:                  "Отправьте изображение или фото",
	ErrPaymentFailed:            "Платёж не удался",
	ErrPaymentNotSetup:          "Платёжный сервис не настроен",
	ErrCategoryCheck:            "Ошибка проверки категории",
	ErrPaymentsCheck:            "Ошибка проверки платежей",
	ErrPaymentsDisabled:         "Платежи временно отключены администратором.",
	ErrAdminCheck:               "Ошибка при проверке прав администратора",
	ErrAdminDenied:              "У вас нет прав администратора",
	ErrSubscriptionsUnavailable: "Подписки временно недоступны",
	ErrInvalidPackage:           "Неверный пакет",
	ErrUnknownModel:             "Неизвестная модель",
	ErrModelRequiresMini:        "Эта модель требует подписку Start или выше",

	CooldownWait:   "⏳ Подождите ещё %d секунд перед следующим запросом.",
	CooldownBuySub: "Или купите любую подписку.",

	SubCheckTitle:   "📢 Чтобы пользоваться ботом, подпишитесь на наш канал:",
	SubCheckJoin:    "📲 Подписаться",
	SubCheckDone:    "✅ Я подписался",
	SubCheckMessage: "Чтобы пользоваться ботом, нужно подписаться на @AIFaceApps.\nНажмите «Подписаться», затем вернитесь и повторите команду.",

	PrivacyPolicy: "🔒 Политика конфиденциальности:",
	PrivacyTerms:  "📜 Пользовательское соглашение:",

	HelpTitle:    "📋 Доступные команды:",
	HelpCommands: "/start - Начать\n/menu - Главное меню\n/buy - Купить доп. запросы\n/rules - Правила использования\n/invite - Пригласить друзей",
	HelpUsage:    "📸 Как пользоваться:\n1. Отправьте фото\n2. Опишите желаемые изменения\n3. Дождитесь результата",
	HelpCost:     "💰 Стоимость: 1 запрос",

	AdminPanel:            "👑 Админ панель",
	AdminSelectAction:     "Выберите действие:",
	AdminStats:            "📊 Статистика",
	AdminUsers:            "👥 Пользователи",
	AdminCategories:       "⚙️ Категории",
	AdminPayments:         "💳 Платежи",
	AdminBalance:          "💰 Баланс API",
	AdminNanoAPI:          "🍌 Nano Banana API",
	AdminSubs:             "🔒 Подписки",
	AdminHelp:             "ℹ️ Помощь",
	AdminMenu:             "🏠 Меню",
	AdminCategoriesTitle:  "⚙️ Категории:",
	AdminCategoryEnabled:  "включено",
	AdminCategoryDisabled: "выключено",
	PaymentsStatusTitle:   "💳 Платежи: %s",
	PaymentsEnabled:       "✅ Включены",
	PaymentsDisabled:      "❌ Выключены",
	PaymentsEnableBtn:     "Включить платежи",
	PaymentsDisableBtn:    "Выключить платежи",
	SubsStatusTitle:       "🔒 Подписки: %s",
	SubsEnabled:           "✅ Включены",
	SubsDisabled:          "❌ Выключены",
	SubsEnableBtn:         "Включить подписки",
	SubsDisableBtn:        "Выключить подписки",

	// Common
	WelcomeText:         "Добро пожаловать! Используйте /menu, чтобы выбрать модель и начать.",
	StartPromoTitle:     "🔥 В данный момент действует скидка 50%% на фото генерации",
	StartPromoCountdown: "⏳ До конца акции: %dд %dч %dм",
	BackBtn:             "⬅️ Назад",
	MenuBtn:             "🏠 Меню",
	RequestUnit:         "запрос",
	PhotoUnit:           "фото",
	VideoUnit:           "видео",
	TrackUnit:           "трек",
	QueryUnit:           "запрос",

	// ... (other fields remain the same)
	ErrAdminRights:               "У вас нет прав администратора",
	ErrCheckSubscription:         "Не удалось проверить подписку. Попробуйте ещё раз.",
	ErrPaymentNotConfigured:      "Платёжный сервис не настроен",
	ErrCreatePayment:             "Не удалось создать платёж: %v",
	ErrGetStats:                  "Ошибка при получении статистики",
	ErrAllCategoriesDisabled:     "Все категории отключены администратором",
	ErrInvalidUserID:             "Некорректный user_id",
	ErrInvalidDays:               "Некорректные days",
	ErrInvalidPlan:               "План должен быть mini, start или pro",
	ErrSetSubscription:           "Не удалось выдать подписку: %v",
	ErrRemoveSubscription:        "Не удалось убрать подписку: %v",
	ErrUnknownAdminCommand:       "Неизвестная админ-команда. Используйте /admin help",
	ErrModelSubscriptionRequired: "Модель доступна только с подпиской Mini или выше",
	ErrChatResponse:              "Не удалось ответить: %s",
	AdminSubSetUsage:             "Использование: /admin sub_set <user_id> <mini|start|pro> <days>",
	AdminSubRemoveUsage:          "Использование: /admin sub_remove <user_id>",
	SubscriptionGranted:          "✅ Подписка %s выдана пользователю %d на %d дней",
	SubscriptionRemoved:          "✅ Подписка удалена у пользователя %d",
	AdminStatsTitle:              "📊 Статистика бота:",
	AdminStatsTotal:              "🎨 Всего запросов: %d",
	AdminStatsCompleted:          "✅ Успешных: %d",
	AdminStatsFailed:             "❌ Ошибок: %d",
	AdminStatsProcessing:         "🔄 В процессе: %d",
	AdminStatsSuccessRate:        "📈 Успешность: %.1f%%",
	AdminStatsAvgTime:            "⏱️ Среднее время: %.1f сек",
	ModelSelected:                "Модель выбрана: %s",
	ModelCategory:                "Категория: %s",
	ModelCost:                    "Стоимость: %d запрос(ов) за 1 %s",
	ModelsTitle:                  "🤖 Текущая модель: %s\n\nВыберите категорию:",
	ModelsNotSelected:            "Не выбрана",
	SubscribeBtn:                 "Подписаться",
	SubscribeRequired:            "Чтобы пользоваться ботом, нужно подписаться на @AIFaceApps.\nНажмите \"Подписаться\", затем вернитесь и повторите команду.",
	Thinking:                     "Думаю...",
	UserPrefix:                   "Пользователь:",
	BotPrefix:                    "Бот:",
	ContextPrefix:                "Вот ваши предыдущие запросы и мои ответы:\n",
	GenServiceError:              "Сервис вернул ошибку. Прочитайте правила /rules и попробуйте переформулировать запрос.",
	QuotaTextRequests:            "текстовые запросы",
	QuotaImageRequests:           "генерации изображений",
	QuotaMusicRequests:           "генерации музыки",
	QuotaVideoRequests:           "видео-запросы",
	QuotaRequests:                "запросы",
	BuyExtraQuota:                "💰 Купить доп. запросы",
	BackToSubscriptions:          "◀️ Назад к подпискам",
	SubscriptionSelected:         "✅ Вы выбрали подписку %s.\nПерейдите по ссылке для оплаты:\n%s",
	PaymentLink:                  "Перейдите по ссылке для оплаты:\n%s",
}

var locEN = Localization{
	// ... (other fields remain the same)
	ErrDefAPIEmptyBilled: "❌ Generation error: An error occurred; you may have violated the bot rules.",
	// ... (other fields remain the same)
	ErrModelRequiresMini: "This model requires a Start subscription or higher",

	CmdStart:    "Start",
	CmdMenu:     "Main menu",
	CmdAccount:  "Account and limits",
	CmdBuy:      "Buy (subscription, requests)",
	CmdInvite:   "Invite friends",
	CmdRules:    "Terms of use",
	CmdPrivacy:  "Privacy",
	CmdSettings: "Settings",
	CmdAdmin:    "Admin",

	// Main menu
	MenuBuyBtn:          "💰 Buy",
	MenuInviteBtn:       "👥 Invite friends",
	MenuSelectModelBtn:  "🧠 Select model",
	MenuGenPhotoBtn:     "🖼️ Generate photo",
	MenuGenMusicBtn:     "🎸 Generate music",
	MenuInviteFriendBtn: "👥 Invite a friend",
	MenuAccountBtn:      "🪪 My account",
	MenuSettingsBtn:     "⚙️ Settings",
	MenuHelpBtn:         "🆘 Help",

	// Account
	AccountUserID:       "User ID: %d",
	AccountSubscription: "⭐ Subscription: %s",
	AccountValidUntil:   "📅 Valid until: %s",
	AccountTextDaily:    "📝 Text requests (24h): %d",
	AccountImagesWeekly: "🖼️ Photos left: %d",
	AccountMusicWeekly:  "🎵 Music: %d",
	AccountVideoWeekly:  "🎬 Video: %d",
	AccountExtraText:    "📝 Extra text requests: %d",
	AccountExtraImages:  "🖼️ Extra images: %d",
	AccountExtraMusic:   "🎵 Extra music: %d",
	AccountExtraVideo:   "🎬 Extra video: %d",

	// Buy menu
	BuyTitle:           "💳 Buy",
	BuySelectAction:    "Choose what you want to buy:",
	BuyConsentNote:     "By paying, you accept the Privacy Policy and Terms of Service (/privacy).",
	BuySubscriptionBtn: "⭐ Subscription",
	BuyExtrasBtn:       "💰 Extra requests",
	BuyBackBtn:         "◀️ Back",
	BuyMenuBtn:         "🏠 Menu",

	// Subscriptions
	SubsTitle:          "⭐ Subscriptions",
	SubsMini:           "✨ Mini — %d ₽ per week",
	SubsStart:          "🚀 Start — %d ₽ per week",
	SubsPro:            "👑 Pro — %d ₽ per week",
	SubsPerWeek:        "per week",
	SubsTextDaily:      "texts/24h",
	SubsImages:         "images",
	SubsSongs:          "songs",
	SubsVideos:         "videos",
	SubsDiscount:       "%d%% discount on extra requests",
	SubsContext:        "x%d context",
	SubsNoAds:          "no ads",
	SubsNoChannel:      "no mandatory channel subscription",
	SubsChatStyles:     "%d GPT chat styles",
	SubsTextModelsMini: "Text models: GPT-5 mini, GPT-5 nano",
	SubsTextModelsHi:   "Text models: Gemini 3 Flash, GPT-5 mini, GPT-5 nano",
	SubsUnavailable:    "Subscriptions are temporarily unavailable",
	SubsPaymentCreated: "✅ You selected %s subscription.\nFollow the link to pay:\n%s",
	SubsBackToSubs:     "◀️ Back to subscriptions",

	// Extras
	ExtrasTitle:       "💰 Buy extra requests",
	ExtrasSelectCat:   "Select a category.",
	ExtrasText:        "📝 Text",
	ExtrasImages:      "🖼️ Images",
	ExtrasMusic:       "🎵 Music",
	ExtrasVideo:       "🎬 Video",
	ExtrasTexts:       "📝 Text requests",
	ExtrasImage:       "🖼️ Image generations",
	ExtrasMusico:      "🎵 Music generations",
	ExtrasVideos:      "🎬 Extra video",
	ExtrasAllDisabled: "All categories are disabled by administrator",

	// Invite
	InviteTitle:    "🎁 You will receive 20% from your referrals' purchases!",
	InviteExample:  "For example, a user bought 10 extra requests — you'll get 2 extra requests for free! Does not apply to subscriptions.",
	InviteLink:     "Your personal referral link:",
	InviteCopyHint: "Tap to copy",
	InviteCount:    "👥 Invited users:",

	// Chat styles
	StyleNormal:           "🙂 Normal",
	StyleFormal:           "📘 Formal",
	StyleHumor:            "😂 Humorous",
	StyleInformal:         "😎 Informal",
	StyleFriendly:         "🤝 Friendly",
	StyleExpert:           "🧠 Expert",
	StyleEmpathetic:       "❤️ Empathetic",
	StyleSelectTitle:      "Select a chat style:",
	StyleCurrentPrefix:    "Current style:",
	StylePromptNormal:     "Answer neutrally and concisely.",
	StylePromptFormal:     "Keep a businesslike and respectful tone.",
	StylePromptHumor:      "Use light friendly humor without sarcasm or insults.",
	StylePromptInformal:   "Write in a relaxed conversational tone, without rudeness.",
	StylePromptFriendly:   "Be warm and supportive, but not verbose.",
	StylePromptExpert:     "Give clear, structured, expert answers.",
	StylePromptEmpathetic: "Respond with empathy and support.",
	ChatSystemPrompt:      "You are a smart Telegram bot assistant. Answer briefly and to the point.",
	ChatSystemLangHint:    "If the user writes in Russian, reply in Russian. If in English, reply in English.",
	ChatStyleProOnly:      "Chat styles are available only with a Pro subscription or higher.",

	// Settings
	SettingsTitle:     "⚙️ Settings",
	SettingsChatStyle: "💬 Chat style",
	SettingsLanguage:  "🌐 Language",
	SettingsBackBtn:   "◀️ Menu",
	SettingsSelect:    "Select a settings section.",

	// Language
	LangTitle:   "🌐 Select language",
	LangRussian: "🇷🇺 Русский",
	LangEnglish: "🇬🇧 English",
	LangChanged: "✅ Language changed to English",

	MenuYourID:                  "Your ID: %d",
	MenuSubscription:            "Subscription: %s",
	MenuCurrentModel:            "Current AI model: %s",
	MenuTitle:                   "🍌 Your ID: %d\n⭐ Subscription: %s\n🧠 Current AI model: %s %s %s",
	MenuLimit:                   "Current model limit: no data",
	MenuLimitFormat:             "Limit: %d (%d extra)",
	ModelsCategory:              "%s",
	ModelsSelect:                "Select a model:",
	ModelsCurrent:               "Current",
	ModelsCost:                  "Cost: %d %s",
	ModelsMaxPhotos:             "Max photos: %d",
	ModelsDescription:           "",
	ModelsCatPhoto:              "📸 Photo",
	ModelsCatVideo:              "🎬 Video",
	ModelsCatMusic:              "🎵 Music",
	ModelsCatChat:               "💬 Text",
	InstrNanoBanana:             "In one message, send 1 or more photos. In the caption, describe what you want to change.",
	InstrTextModels:             "Send any message to the chat.",
	InstrSunoMusic:              "Send a description of the music you want to create.",
	InstrHugVideo:               "Send 1 photo with two or more people and wait.",
	ModelNanoBanana:             "Medium quality. Works poorly with two photos. Average wait time ~1 minute.",
	ModelNanoBananaPro:          "Newest model, best quality. Average wait time 1–2 minutes.",
	ModelHugVideo:               "Video: animate photo with hugging.",
	ModelSunoMusic:              "Music generation. Wait time 5–10 minutes.",
	ModelGeminiFlash:            "Text: fast replies. Available with Start+ subscription.",
	ModelGPT5Mini:               "Text: fast replies. Available with Mini+ subscription.",
	ModelGPT5Nano:               "Text: slower replies. Available with Mini+ subscription.",
	ModelGPT41Mini:              "Text: fast replies for text requests.",
	PhotoReceived:               "📷 Photo received!",
	PhotoAddCaption:             "Please resend the photo with a caption — describe what you want to change.",
	PhotoExamples:               "Examples:",
	PhotoExample1:               "\"Make a short haircut\"",
	PhotoExample2:               "\"Try on a red dress\"",
	PhotoExample3:               "\"Change hair color to blonde\"",
	PhotoExample4:               "\"Remove a certain object from the photo\"",
	GenStarted:                  "🔄 Generation started",
	GenModel:                    "Model: %s",
	GenDeducted:                 "Deducted: %d request(s)",
	GenWaiting:                  "Waiting until %s",
	GenReady:                    "✅ Done!",
	GenFailed:                   "❌ Generation failed",
	GenUnavailable:              "🖼️ Result is unavailable",
	MusicStarted:                "🔄 Voiceover started",
	MusicMode:                   "Mode:",
	MusicModeVocal:              "with vocal",
	MusicModeInstr:              "instrumental (no vocal)",
	MusicVoice:                  "Voice:",
	MusicVoiceMale:              "male",
	MusicVoiceFemale:            "female",
	MusicReady:                  "🔊 Audio is ready",
	MusicVariants:               "Here are 2 different variants of the song:",
	MusicFailed:                 "Failed to generate song",
	MusicSelectModel:            "Select a music model:",
	ErrPrefix:                   "❌ Error: %s",
	ErrAccessDenied:             "Access is allowed only for admins.",
	ErrNotSubscribed:            "To use the bot, subscribe to @AIFaceApps.",
	ErrCategoryDisabled:         "Category is temporarily unavailable",
	ErrInsufficientQuota:        "Not enough %s. Need %d.",
	ErrNeedQuota:                "Not enough %s",
	ErrServiceError:             "Service returned an error. Read the bot rules /rules and try to rephrase your request.",
	ErrUnknownCommand:           "❓ Unknown command. Use /help to see available commands.",
	ErrEmptyRequest:             "Empty request",
	ErrNoImage:                  "Send an image or a photo",
	ErrPaymentFailed:            "Payment failed",
	ErrPaymentNotSetup:          "Payment service is not configured",
	ErrCategoryCheck:            "Category check failed",
	ErrPaymentsCheck:            "Payments check failed",
	ErrPaymentsDisabled:         "Payments are temporarily disabled by administrator.",
	ErrAdminCheck:               "Admin check failed",
	ErrAdminDenied:              "You do not have administrator rights",
	ErrSubscriptionsUnavailable: "Subscriptions are temporarily unavailable",
	ErrInvalidPackage:           "Invalid package",
	ErrUnknownModel:             "Unknown model",

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

	WelcomeText:         "Welcome! Use /menu to select a model and get started.",
	StartPromoTitle:     "🔥 50%% discounts until %s",
	StartPromoCountdown: "⏳ Offer ends in: %dd %dh %dm",

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
