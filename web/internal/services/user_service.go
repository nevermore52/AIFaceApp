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

func (s *UserService) AddExtraQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	return s.quotaRepo.AddExtra(telegramID, category, amount)
}

func (s *UserService) AddPrimaryQuota(telegramID int64, category models.QuotaCategory, amount int) error {
	return s.quotaRepo.AddPrimary(telegramID, category, amount)
}

func (s *UserService) ConsumeQuota(telegramID int64, category models.QuotaCategory, amount int) (primaryUsed, extraUsed int, err error) {
	return s.quotaRepo.Consume(telegramID, category, amount)
}

func (s *UserService) GetAll(limit, offset int) ([]*models.User, int, error) {
	return s.userRepo.GetAll(limit, offset)
}

func (s *UserService) GetStats() (map[string]interface{}, error) {
	return s.userRepo.GetStats()
}

func (s *UserService) IsAdmin(userID int64) (bool, error) {
	return s.userRepo.IsAdmin(userID)
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

	// Выдаём бонус: +2 к фото
	_ = s.quotaRepo.AddExtra(*user.TelegramID, models.QuotaCategoryImage, 2)

	if err := s.userRepo.MarkChannelTrialClaimed(user.ID); err != nil {
		return true, false, fmt.Errorf("mark bonus given: %w", err)
	}
	return true, false, nil
}
