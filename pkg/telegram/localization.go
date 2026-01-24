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
	MenuTitle          string
	MenuYourID         string
	MenuSubscription   string
	MenuCurrentModel   string
	MenuLimit          string
	MenuLimitFormat    string
	MenuBuyBtn         string
	MenuInviteBtn      string
	MenuSelectModelBtn string

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
	CmdStart:    "Начать",
	CmdMenu:     "Главное меню",
	CmdAccount:  "Аккаунт и лимиты",
	CmdBuy:      "Купить (подписку,доп.запросы)",
	CmdInvite:   "Пригласить друзей",
	CmdRules:    "Правила использования",
	CmdPrivacy:  "Политика и Польз. соглашение",
	CmdSettings: "Настройки",
	CmdAdmin:    "Админ-панель",

	// Main menu
	MenuTitle:          "🍌 Ваш айди: %d\n⭐ Тип подписки: %s\n🧠 Текущая модель: %s %s — %s",
	MenuYourID:         "🍌 Ваш айди",
	MenuSubscription:   "⭐ Тип подписки",
	MenuCurrentModel:   "🧠 Текущая модель",
	MenuLimit:          "Лимит текущей модели: нет данных",
	MenuLimitFormat:    "Лимит: %d (%d доп)",
	MenuBuyBtn:         "💰 Покупка",
	MenuInviteBtn:      "👥 Пригласить друзей",
	MenuSelectModelBtn: "🧠 Выбрать модель",

	// Account
	AccountUserID:       "ID Пользователя: %d",
	AccountSubscription: "⭐ Тип подписки: %s",
	AccountValidUntil:   "📅 Действует до: %s",
	AccountTextDaily:    "📝 Текстовые генерации (24 ч): %d",
	AccountImagesWeekly: "🖼️ Картинок осталось: %d",
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
	BuyExtrasBtn:       "💰 Доп. запросы",
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
	ExtrasTexts:       "📝 Текст",
	ExtrasImage:       "🖼️ Доп. запросы изображений",
	ExtrasMusico:      "🎵 Доп. музыка",
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
	ModelsCost:        "Расход: %d запрос",
	ModelsMaxPhotos:   "Максимум фото: %d",
	ModelsDescription: "Категории моделей:\n📸 Фото — редактирование и генерация изображений.\n🎵 Песни — генерация треков.\n💬 Текст — ответы на вопросы и сопровождение.",
	ModelsCatPhoto:    "📸 Фото",
	ModelsCatVideo:    "🎬 Видео",
	ModelsCatMusic:    "🎵 Музыка",
	ModelsCatChat:     "💬 Текст",
	InstrNanoBanana:   "В 1 сообщении отправьте 1 или более фото, в подписи укажите что хотите изменить (промпт).",
	InstrTextModels:   "Отправьте любое сообщение в чат.",
	InstrSunoMusic:    "Отправьте в чат описание музыки, которую хотите создать.",
	InstrHugVideo:     "Отправьте 1 фото с двумя или более людьми и ожидайте.",

	// Photo models descriptions
	ModelNanoBanana:    "Фото: Старая модель, среднее качество, плохо работает с большими запросами и двумя фотографиями. Среднее время ожидания 1-2 минуты, максимум до 10 минут",
	ModelNanoBananaPro: "Фото: Новейшая модель, лучшее качество. Среднее время ожидания 1-2 минуты, максимум до 10 минут",
	ModelHugVideo:      "Видео: оживление фото с обнимашками.",
	ModelSunoMusic:     "Музыка: генерация песни. До 10-15 минут.",
	ModelGeminiFlash:   "Текст: быстрые ответы. Доступно с подпиской Start+.",
	ModelGPT5Mini:      "Текст: быстрые ответы. Доступно с подпиской Mini+.",
	ModelGPT5Nano:      "Текст: дольше ответы. Доступно с подпиской Mini+.",
	ModelGPT41Mini:     "Чат-бот: быстрые ответы на текстовые запросы.",

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
	MusicSelectModel: "Выберите музыкальную модель в меню моделей, чтобы сгенерировать песню.",

	// Errors
	ErrPrefix:                   "❌ Ошибка:",
	ErrAccessDenied:             "Доступ только для админов.",
	ErrNotSubscribed:            "Подпишитесь на канал",
	ErrCategoryDisabled:         "Категория отключена администратором",
	ErrInsufficientQuota:        "❌ Недостаточно %s. Нужно %d.\n%s\n\nИспользуйте /buy чтобы докупить доп. запросы.",
	ErrNeedQuota:                "Нужно %d",
	ErrServiceError:             "Сервис вернул ошибку. Прочитайте правила нашего бота /rules и попробуйте переформулировать ваш запрос и отправьте его снова.",
	ErrUnknownCommand:           "❓ Неизвестная команда. Используйте /help для получения списка доступных команд.",
	ErrEmptyRequest:             "Пустой запрос для озвучки",
	ErrNoImage:                  "Не удалось получить изображение",
	ErrPaymentFailed:            "Не удалось создать платеж: %v",
	ErrPaymentNotSetup:          "Платёжный сервис не настроен",
	ErrCategoryCheck:            "Ошибка проверки категории",
	ErrPaymentsCheck:            "Ошибка проверки платежей",
	ErrPaymentsDisabled:         "Платежи временно отключены администратором.",
	ErrAdminCheck:               "Ошибка при проверке прав администратора",
	ErrAdminDenied:              "У вас нет прав администратора",
	ErrSubscriptionsUnavailable: "Подписки временно недоступны",
	ErrInvalidPackage:           "Неверный пакет",
	ErrUnknownModel:             "Неизвестная модель",
	ErrModelRequiresMini:        "Модель доступна только с подпиской start и выше",

	// Cooldown
	CooldownWait:   "⏳ Подождите ещё %d секунд перед следующим запросом.",
	CooldownBuySub: "Или купите любую подписку.",

	// Subscription check
	SubCheckTitle:   "📢 Для использования бота подпишитесь на наш канал:",
	SubCheckJoin:    "📲 Подписаться",
	SubCheckDone:    "✅ Я подписался",
	SubCheckMessage: "Для использования бота нужна подписка на канал @AIFaceApps.\nНажмите «Подписаться», затем вернитесь и повторите команду.",

	// Rules
	RulesTitle:   "📜 Правила бота:",
	RulesContent: "1) 18+ контент запрещён к генерации и отправке.\n2) Нельзя загружать обнажёнку, жестокость, спам или нелегальный контент.\n3) Не нарушайте авторские права и чужие личные данные.\n4) Соблюдайте лимиты запросов и уважайте инфраструктуру.\n5) Используйте /menu для выбора модели и следуйте её инструкции.\n6) Запрещены любые оскорбления третьих лиц в песнях, фото, чат.\n\nНарушения могут привести к блокировке. Без возврата средств.",

	// Privacy
	PrivacyPolicy: "🔒 Политика конфиденциальности:",
	PrivacyTerms:  "📜 Пользовательское соглашение:",

	// Help
	HelpTitle:    "📋 Доступные команды:",
	HelpCommands: "/start - Начать\n/menu - Главное меню\n/buy - Купить доп. запросы\n/rules - Правила использования\n/invite - Пригласить друзей",
	HelpUsage:    "📸 Как использовать:\n1. Отправьте фото\n2. Опишите желаемые изменения\n3. Дождитесь результата",
	HelpCost:     "💰 Расход: 1 запрос",

	// Admin
	AdminPanel:            "👑 Админ-панель",
	AdminSelectAction:     "Выберите действие:",
	AdminStats:            "📊 Статистика",
	AdminUsers:            "👥 Пользователи",
	AdminCategories:       "⚙️ Категории",
	AdminPayments:         "💳 Платежи",
	AdminBalance:          "💰 Баланс API",
	AdminNanoAPI:          "🍌 Nano Banana API",
	AdminSubs:             "🔒 Подписки",
	AdminHelp:             "ℹ️ Справка",
	AdminMenu:             "🏠 Меню",
	AdminCategoriesTitle:  "⚙️ Категории:",
	AdminCategoryEnabled:  "включена",
	AdminCategoryDisabled: "выключена",
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
	BuyPackageTitle:       "✅ Вы выбрали пакет %s: %d шт.\nПерейдите по ссылке для оплаты:\n%s",
	BuyPackageLabelText:   "текстовые запросы",
	BuyPackageLabelImage:  "запросы на изображения",
	BuyPackageLabelMusic:  "музыкальные запросы",
	BuyPackageLabelVideo:  "видео-запросы",
	BuyPackageBackBtn:     "◀️ Назад к категориям",

	// Aspect ratio
	AspectTitle:     "📐 Формат",
	AspectLandscape: "Пейзаж 16:9",
	AspectPortrait:  "Портрет 9:16",
	AspectSquare:    "Аватар 1:1",

	// Common
	BackBtn:     "◀️ Назад",
	MenuBtn:     "🏠 Меню",
	RequestUnit: "запрос(ов)",
	PhotoUnit:   "фото",
	VideoUnit:   "видео",
	TrackUnit:   "трек",
	QueryUnit:   "запрос",

	// Additional fields for localization
	ErrAdminRights:               "У вас нет прав администратора",
	ErrCheckSubscription:         "Не удалось проверить подписку. Попробуйте ещё раз.",
	ErrPaymentNotConfigured:      "Платёжный сервис не настроен",
	ErrCreatePayment:             "Не удалось создать платеж: %v",
	ErrGetStats:                  "Ошибка при получении статистики",
	ErrAllCategoriesDisabled:     "Все категории отключены администратором",
	ErrInvalidUserID:             "Некорректный user_id",
	ErrInvalidDays:               "Некорректные days",
	ErrInvalidPlan:               "План должен быть mini, start и выше",
	ErrSetSubscription:           "Не удалось выдать подписку: %v",
	ErrRemoveSubscription:        "Не удалось убрать подписку: %v",
	ErrUnknownAdminCommand:       "Неизвестная админ-команда. Используйте /admin help",
	ErrModelSubscriptionRequired: "Модель доступна только с подпиской Mini и выше",
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
	ModelCost:                    "Расход: %d запрос",
	ModelsCategory:               "Категория: %s",
	ModelsTitle:                  "🤖 Текущая модель: %s\n\nВыберите категорию:",
	ModelsNotSelected:            "Не выбрана",
	SubscribeBtn:                 "Подписаться",
	SubscribeRequired:            "Для использования бота нужна подписка на канал @AIFaceApps.\nНажмите «Подписаться», затем вернитесь и повторите команду.",
	Thinking:                     "Думаю над ответом...",
	UserPrefix:                   "Пользователь:",
	BotPrefix:                    "Бот:",
	ContextPrefix:                "Вот прошлые запросы и твои ответы:\n",
	GenServiceError:              "Сервис вернул ошибку. Прочитайте правила нашего бота /rules и попробуйте переформулировать ваш запрос и отправьте его снова.",
	QuotaTextRequests:            "текстовых запросов",
	QuotaImageRequests:           "запросов на изображения",
	QuotaMusicRequests:           "музыкальные запросы",
	QuotaVideoRequests:           "видео-запросов",
	QuotaRequests:                "запросов",
	BuyExtraQuota:                "💰 Купить доп. запросы",
	BackToSubscriptions:          "◀️ Назад к подпискам",
	SubscriptionSelected:         "✅ Вы выбрали подписку %s.\nПерейдите по ссылке для оплаты:\n%s",
	PaymentLink:                  "Перейдите по ссылке для оплаты:\n%s",
}

var locEN = Localization{
	// Commands
	CmdStart:    "Start",
	CmdMenu:     "Main menu",
	CmdAccount:  "Account and limits",
	CmdBuy:      "Buy (subscription, extra requests)",
	CmdInvite:   "Invite friends",
	CmdRules:    "Terms of use",
	CmdPrivacy:  "Privacy Policy & Terms",
	CmdSettings: "Settings",
	CmdAdmin:    "Admin panel",

	// Main menu
	MenuTitle:          "🍌 Your ID: %d\n⭐ Subscription: %s\n🧠 Current model: %s %s — %s",
	MenuYourID:         "🍌 Your ID",
	MenuSubscription:   "⭐ Subscription",
	MenuCurrentModel:   "🧠 Current model",
	MenuLimit:          "Current model limit: no data",
	MenuLimitFormat:    "Limit: %d (%d extra)",
	MenuBuyBtn:         "💰 Purchase",
	MenuInviteBtn:      "👥 Invite friends",
	MenuSelectModelBtn: "🧠 Select model",

	// Account
	AccountUserID:       "User ID: %d",
	AccountSubscription: "⭐ Subscription: %s",
	AccountValidUntil:   "📅 Valid until: %s",
	AccountTextDaily:    "📝 Text generations (24h): %d",
	AccountImagesWeekly: "🖼️ Images left (week): %d",
	AccountMusicWeekly:  "🎵 Music (week): %d",
	AccountVideoWeekly:  "🎬 Video (week): %d",
	AccountExtraText:    "📝 Extra text generations: %d",
	AccountExtraImages:  "🖼️ Extra images: %d",
	AccountExtraMusic:   "🎵 Extra music: %d",
	AccountExtraVideo:   "🎬 Extra video: %d",

	// Buy menu
	BuyTitle:           "💰 Purchases",
	BuySelectAction:    "Select what you want to buy:",
	BuyConsentNote:     "By paying, you agree to the Privacy Policy and Terms (/privacy).",
	BuySubscriptionBtn: "⭐ Subscription",
	BuyExtrasBtn:       "📦 Extra requests",
	BuyBackBtn:         "◀️ Back",
	BuyMenuBtn:         "🏠 Menu",

	// Subscriptions
	SubsTitle:          "⭐ Subscriptions",
	SubsMini:           "✨ Mini — %d ₽ per week",
	SubsStart:          "🚀 Start — %d ₽ per week",
	SubsPro:            "👑 Pro — %d ₽ per week",
	SubsPerWeek:        "per week",
	SubsTextDaily:      "text/24h",
	SubsImages:         "images",
	SubsSongs:          "songs",
	SubsVideos:         "videos",
	SubsDiscount:       "%d%% discount on extra requests",
	SubsContext:        "x%d context",
	SubsNoAds:          "no ads",
	SubsChatStyles:     "%d GPT chat styles",
	SubsTextModelsMini: "Text models: GPT-5 mini, GPT-5 nano",
	SubsTextModelsHi:   "Text models: Gemini 3 Flash, GPT-5 mini, GPT-5 nano",
	SubsUnavailable:    "Subscriptions are temporarily unavailable",
	SubsPaymentCreated: "✅ You selected %s subscription.\nFollow the link to pay:\n%s",
	SubsBackToSubs:     "◀️ Back to subscriptions",

	// Extras
	ExtrasTitle:       "💰 Buy extra requests",
	ExtrasSelectCat:   "Select category.",
	ExtrasText:        "📝 Text",
	ExtrasImages:      "🖼️ Images",
	ExtrasMusic:       "🎵 Music",
	ExtrasVideo:       "🎬 Video",
	ExtrasAllDisabled: "All categories are disabled by administrator",
	ExtrasTexts:       "📝 Text",
	ExtrasImage:       "🖼️ Images",
	ExtrasMusico:      "🎵 Music",
	ExtrasVideos:      "🎬 Video",

	// Invite
	InviteTitle:    "🎁 You will receive 20% of your referrals' purchases!",
	InviteExample:  "For example, if a user buys 10 extra requests, you get 2 extra requests for free!",
	InviteLink:     "Your personal referral link:",
	InviteCopyHint: "*Click to copy*",
	InviteCount:    "👥 Users invited:",

	// Settings
	SettingsTitle:     "⚙️ Settings",
	SettingsChatStyle: "💬 Chat style",
	SettingsLanguage:  "🌐 Language",
	SettingsBackBtn:   "◀️ Menu",
	SettingsSelect:    "Select settings section.",

	// Language
	LangTitle:   "🌐 Select language / Выберите язык",
	LangRussian: "🇷🇺 Русский",
	LangEnglish: "🇬🇧 English",
	LangChanged: "✅ Language changed to English",

	// Chat styles
	StyleNormal:           "🙂 Normal",
	StyleFormal:           "📘 Formal",
	StyleHumor:            "😂 Humorous",
	StyleInformal:         "😎 Informal",
	StyleFriendly:         "🤝 Friendly",
	StyleExpert:           "🧠 Expert",
	StyleEmpathetic:       "❤️ Empathetic",
	StyleSelectTitle:      "Select GPT chat style:",
	StyleCurrentPrefix:    "Current style:",
	StylePromptNormal:     "Respond neutrally and concisely.",
	StylePromptFormal:     "Keep a businesslike and respectful tone.",
	StylePromptHumor:      "Respond with light friendly humor without sarcasm or insults.",
	StylePromptInformal:   "Write in a relaxed conversational tone, no bureaucracy and no rudeness.",
	StylePromptFriendly:   "Be warm and supportive, but not verbose.",
	StylePromptExpert:     "Provide clear, structured, expert-level answers.",
	StylePromptEmpathetic: "Respond with empathy and support.",
	ChatSystemPrompt:      "You are a smart assistant in a Telegram bot. Keep replies brief and to the point.",
	ChatSystemLangHint:    "If the user writes in Russian, reply in Russian. If in English — reply in English.",
	ChatStyleProOnly:      "Chat styles are available only with Pro subscription and above.",

	// Models
	ModelsCategory:    "Category: %s",
	ModelsSelect:      "Select a model. It will be used by default for your requests.",
	ModelsCurrent:     "Current:",
	ModelsCost:        "Cost: %d request",
	ModelsMaxPhotos:   "Max photos: %d",
	ModelsDescription: "Model categories:\n📸 Photo — image editing and generation.\n🎵 Songs — track generation.\n💬 Text — answers and assistance.",
	ModelsCatPhoto:    "📸 Photo",
	ModelsCatVideo:    "🎬 Video",
	ModelsCatMusic:    "🎵 Music",
	ModelsCatChat:     "💬 Text",
	InstrNanoBanana:   "Send 1 or more photos in a single message and add a caption with the changes you want (prompt).",
	InstrTextModels:   "Send any message in chat.",
	InstrSunoMusic:    "Send in chat a description of the music you want to create.",
	InstrHugVideo:     "Send 1 photo with two or more people and wait.",

	// Photo models descriptions
	ModelNanoBanana:    "Photo: Older model, medium quality, works poorly with large requests and two photos. Average wait 1-2 min, max 10 min",
	ModelNanoBananaPro: "Photo: Latest model, best quality. Average wait 1-2 min, max 10 min",
	ModelHugVideo:      "Video: animate photo with hugging effect.",
	ModelSunoMusic:     "Music: song generation. Up to 10-15 min.",
	ModelGeminiFlash:   "Text: fast responses. Available with Start+ subscription.",
	ModelGPT5Mini:      "Text: fast responses. Available with Mini+ subscription.",
	ModelGPT5Nano:      "Text: slower responses. Available with Mini+ subscription.",
	ModelGPT41Mini:     "Chat bot: fast text responses.",

	// Photo instructions
	PhotoReceived:   "📷 Photo received!",
	PhotoAddCaption: "Please send the photo again with a caption — describe what you want to change.",
	PhotoExamples:   "Examples:",
	PhotoExample1:   "\"Make a short haircut\"",
	PhotoExample2:   "\"Try on a red dress\"",
	PhotoExample3:   "\"Change hair color to blonde\"",
	PhotoExample4:   "\"Remove a specific object from photo\"",

	// Generation
	GenStarted:     "🔄 Generation started",
	GenModel:       "Model: %s",
	GenDeducted:    "Deducted: %d request(s)",
	GenWaiting:     "Wait up to %s",
	GenReady:       "✅ Done!",
	GenFailed:      "❌ Generation failed",
	GenUnavailable: "🖼️ Result unavailable",

	// Music
	MusicStarted:     "🔄 Music generation started",
	MusicMode:        "Mode:",
	MusicModeVocal:   "with vocals",
	MusicModeInstr:   "instrumental (no vocals)",
	MusicVoice:       "Voice:",
	MusicVoiceMale:   "male",
	MusicVoiceFemale: "female",
	MusicReady:       "🔊 Audio ready",
	MusicVariants:    "Here are 2 different song variants:",
	MusicFailed:      "Failed to generate song",
	MusicSelectModel: "Select a music model in the models menu to generate a song.",

	// Errors
	ErrPrefix:                   "❌ Error:",
	ErrAccessDenied:             "Access for admins only.",
	ErrNotSubscribed:            "Subscribe to the channel",
	ErrCategoryDisabled:         "Category disabled by administrator",
	ErrInsufficientQuota:        "❌ Insufficient %s. Need %d.\n%s\n\nUse /buy to purchase extra requests.",
	ErrNeedQuota:                "Need %d",
	ErrServiceError:             "Service returned an error. Read our bot rules /rules and try rephrasing your request.",
	ErrUnknownCommand:           "❓ Unknown command. Use /help to see available commands.",
	ErrEmptyRequest:             "Empty request for audio",
	ErrNoImage:                  "Failed to get image",
	ErrPaymentFailed:            "Failed to create payment: %v",
	ErrPaymentNotSetup:          "Payment service not configured",
	ErrCategoryCheck:            "Failed to check category",
	ErrPaymentsCheck:            "Failed to check payments",
	ErrPaymentsDisabled:         "Payments are temporarily disabled by administrator.",
	ErrAdminCheck:               "Failed to check admin permissions",
	ErrAdminDenied:              "You do not have admin rights",
	ErrSubscriptionsUnavailable: "Subscriptions are temporarily unavailable",
	ErrInvalidPackage:           "Invalid package",
	ErrUnknownModel:             "Unknown model",
	ErrModelRequiresMini:        "This model requires a Start subscription or higher",

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
