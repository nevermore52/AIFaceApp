package services

import (
	"time"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"
)

type GenerationService struct {
	repo *repository.GenerationRepository
}

func NewGenerationService(repo *repository.GenerationRepository) *GenerationService {
	return &GenerationService{repo: repo}
}

func (s *GenerationService) GetByID(id int64) (*models.GenerationRequest, error) {
	return s.repo.GetByID(id)
}

func (s *GenerationService) GetByUserID(userID int64, limit, offset int) ([]*models.GenerationRequest, int, error) {
	return s.repo.GetByUserID(userID, limit, offset)
}

func (s *GenerationService) GetAll(limit, offset int) ([]*models.GenerationRequest, int, error) {
	return s.repo.GetAll(limit, offset)
}

func (s *GenerationService) GetStats() (map[string]interface{}, error) {
	return s.repo.GetStats()
}

func (s *GenerationService) GetStatsSince(since time.Time) (map[string]interface{}, error) {
	return s.repo.GetStatsSince(since)
}

func (s *GenerationService) GetTopUsers(limit int) ([]*models.TopUser, error) {
	return s.repo.GetTopUsers(limit)
}
