package services

import (
	"time"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/repository"
)

type PaymentService struct {
	paymentRepo *repository.PaymentRepository
	userRepo    *repository.UserRepository
	quotaRepo   *repository.QuotaRepository
	priceTable  map[string]map[int]int
	subPrices   map[string]int
}

func NewPaymentService(paymentRepo *repository.PaymentRepository, userRepo *repository.UserRepository, quotaRepo *repository.QuotaRepository) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		userRepo:    userRepo,
		quotaRepo:   quotaRepo,
		priceTable: map[string]map[int]int{
			"image": {10: 99, 50: 389, 100: 669, 250: 1499, 500: 2899},
			"text":  {10: 10, 50: 45, 100: 80, 250: 200, 500: 345},
			"music": {1: 25, 5: 119, 10: 202, 50: 812, 100: 1599},
			"video": {10: 99, 50: 379, 100: 699, 250: 1650, 500: 3100, 1000: 5900},
		},
		subPrices: map[string]int{
			"mini":  299,
			"start": 499,
			"pro":   799,
		},
	}
}

func (s *PaymentService) GetPackages() []models.PricePackage {
	var packages []models.PricePackage
	for category, prices := range s.priceTable {
		for qty, price := range prices {
			packages = append(packages, models.PricePackage{
				Category: category,
				Qty:      qty,
				Price:    price,
			})
		}
	}
	return packages
}

func (s *PaymentService) GetSubscriptions() []models.SubscriptionPlan {
	return []models.SubscriptionPlan{
		{Name: "mini", Price: 299, TextDaily: 50, ImageWeekly: 25, MusicWeekly: 3, VideoWeekly: 0, Discount: 10},
		{Name: "start", Price: 499, TextDaily: 100, ImageWeekly: 40, MusicWeekly: 5, VideoWeekly: 20, Discount: 15},
		{Name: "pro", Price: 799, TextDaily: 200, ImageWeekly: 90, MusicWeekly: 10, VideoWeekly: 50, Discount: 20},
	}
}

func (s *PaymentService) GetPrice(category string, qty int) (int, bool) {
	if prices, ok := s.priceTable[category]; ok {
		if price, ok := prices[qty]; ok {
			return price, true
		}
	}
	return 0, false
}

func (s *PaymentService) GetSubscriptionPrice(plan string) (int, bool) {
	price, ok := s.subPrices[plan]
	return price, ok
}

func (s *PaymentService) GetByUserID(userID int64, limit, offset int) ([]*models.Payment, int, error) {
	return s.paymentRepo.GetByUserID(userID, limit, offset)
}

func (s *PaymentService) GetAll(limit, offset int) ([]*models.Payment, int, error) {
	return s.paymentRepo.GetAll(limit, offset)
}

func (s *PaymentService) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	now := time.Now().UTC()

	dayCount, dayAmount, err := s.paymentRepo.GetStats(now.Add(-24 * time.Hour))
	if err != nil {
		return nil, err
	}
	stats["day_count"] = dayCount
	stats["day_amount"] = dayAmount

	weekCount, weekAmount, err := s.paymentRepo.GetStats(now.Add(-7 * 24 * time.Hour))
	if err != nil {
		return nil, err
	}
	stats["week_count"] = weekCount
	stats["week_amount"] = weekAmount

	totalCount, totalAmount, err := s.paymentRepo.GetStatsAll()
	if err != nil {
		return nil, err
	}
	stats["total_count"] = totalCount
	stats["total_amount"] = totalAmount

	return stats, nil
}
