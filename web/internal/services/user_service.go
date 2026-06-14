package services

import (
	"encoding/json"
	"fmt"
	"net/http"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"
)

type UserService struct {
	userRepo  *repository.UserRepository
	quotaRepo *repository.QuotaRepository
}

func NewUserService(userRepo *repository.UserRepository, quotaRepo *repository.QuotaRepository) *UserService {
	return &UserService{
		userRepo:  userRepo,
		quotaRepo: quotaRepo,
	}
}

func (s *UserService) GetByID(id int64) (*models.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) GetUserByID(id int64) (*models.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) GetByTelegramID(telegramID int64) (*models.User, error) {
	return s.userRepo.GetByTelegramID(telegramID)
}

func (s *UserService) Update(user *models.User) error {
	return s.userRepo.Update(user)
}

func (s *UserService) GetQuota(telegramID int64) (*models.UserQuota, error) {
	return s.quotaRepo.GetOrCreate(telegramID)
}

func (s *UserService) EnsureDailyReset(telegramID int64) error {
	return s.quotaRepo.EnsureDailyReset(telegramID)
}

func (s *UserService) AddExtraQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	return s.quotaRepo.AddExtra(telegramID, category, amount)
}

func (s *UserService) AddPrimaryQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	return s.quotaRepo.AddPrimary(telegramID, category, amount)
}

func (s *UserService) ConsumeQuota(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error) {
	return s.quotaRepo.Consume(telegramID, category, amount)
}

func (s *UserService) GetAll(limit, offset int, search string) ([]*models.User, int, error) {
	return s.userRepo.GetAll(limit, offset, search)
}

func (s *UserService) GetStats() (map[string]interface{}, error) {
	return s.userRepo.GetStats()
}

func (s *UserService) IsAdmin(userID int64) (bool, error) {
	return s.userRepo.IsAdmin(userID)
}

func (s *UserService) RemoveSubscription(userID int64) error {
	return s.userRepo.RemoveSubscription(userID)
}

// ClaimChannelBonus проверяет подписку на канал через Telegram Bot API
// и выдаёт бонусные запросы если ещё не выдавались.
// Возвращает (subscribed, alreadyClaimed, error).
func (s *UserService) ClaimChannelBonus(user *models.User, botToken string) (subscribed bool, alreadyClaimed bool, err error) {
	if user.ChannelTrialClaimed {
		return false, true, nil
	}
	if user.TelegramID == nil {
		return false, false, fmt.Errorf("telegram not linked")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChatMember?chat_id=@aifaceapps&user_id=%d", botToken, *user.TelegramID)
	resp, err := http.Get(url)
	if err != nil {
		return false, false, fmt.Errorf("telegram api error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, false, fmt.Errorf("parse telegram response: %w", err)
	}

	status := result.Result.Status
	subscribed = status == "member" || status == "administrator" || status == "creator"
	if !subscribed {
		return false, false, nil
	}

	// Атомарно помечаем — только первый вызов выдаёт бонус
	ok, err := s.userRepo.TryClaimChannelTrial(user.ID)
	if err != nil {
		return true, false, fmt.Errorf("mark bonus given: %w", err)
	}
	if !ok {
		return false, true, nil // кто-то уже выдал
	}

	_ = s.quotaRepo.AddExtra(*user.TelegramID, models.QuotaCategoryImage, 2)
	return true, false, nil
}

// GetUserQuotaBalance возвращает баланс генераций пользователя по всем категориям
func (s *UserService) GetUserQuotaBalance(userID int64) (map[string]interface{}, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user.TelegramID == nil {
		return nil, fmt.Errorf("user has no telegram_id")
	}

	quota, err := s.quotaRepo.GetOrCreate(*user.TelegramID)
	if err != nil {
		return nil, err
	}

	balance := map[string]interface{}{
		"image": map[string]int{
			"primary": quota.ImageWeekly,
			"extra":   quota.ImageExtra,
			"total":   quota.ImageWeekly + quota.ImageExtra,
		},
		"video": map[string]int{
			"primary": quota.VideoWeekly,
			"extra":   quota.VideoExtra,
			"total":   quota.VideoWeekly + quota.VideoExtra,
		},
		"music": map[string]int{
			"primary": quota.MusicWeekly,
			"extra":   quota.MusicExtra,
			"total":   quota.MusicWeekly + quota.MusicExtra,
		},
		"text": map[string]int{
			"primary": quota.TextDaily,
			"extra":   quota.TextExtra,
			"total":   quota.TextDaily + quota.TextExtra,
		},
	}

	return balance, nil
}

// AddUserQuota добавляет генерации пользователю
func (s *UserService) AddUserQuota(userID int64, category string, amount int) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user.TelegramID == nil {
		return fmt.Errorf("user has no telegram_id")
	}

	var quotaCategory models.QuotaCategory
	switch category {
	case "image":
		quotaCategory = models.QuotaCategoryImage
	case "video":
		quotaCategory = models.QuotaCategoryVideo
	case "music":
		quotaCategory = models.QuotaCategoryMusic
	case "text":
		quotaCategory = models.QuotaCategoryText
	default:
		return fmt.Errorf("invalid category: %s", category)
	}

	return s.quotaRepo.AddExtra(*user.TelegramID, quotaCategory, amount)
}
