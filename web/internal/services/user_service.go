package services

import (
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
