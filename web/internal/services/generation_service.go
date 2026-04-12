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

// Gallery Ideas Management

func (s *GenerationService) CreateGalleryIdea(model, output, prompt string, priority *int) (*repository.GalleryIdea, error) {
	return s.repo.CreateGalleryIdea(model, output, prompt, priority)
}

func (s *GenerationService) GetGalleryIdeas(limit, offset int) ([]*repository.GalleryIdea, int, error) {
	return s.repo.GetGalleryIdeas(limit, offset)
}

func (s *GenerationService) GetGalleryIdeasSorted(sort string, limit, offset int) ([]*repository.GalleryIdea, int, error) {
	return s.repo.GetGalleryIdeasSorted(sort, limit, offset)
}

func (s *GenerationService) GetOccupiedPriorities(excludeID int64) ([]int, error) {
	return s.repo.GetOccupiedPriorities(excludeID)
}

func (s *GenerationService) UpdateGalleryIdea(id int64, model, output, prompt string, priority *int) error {
	return s.repo.UpdateGalleryIdea(id, model, output, prompt, priority)
}

func (s *GenerationService) DeleteGalleryIdea(id int64) error {
	return s.repo.DeleteGalleryIdea(id)
}
