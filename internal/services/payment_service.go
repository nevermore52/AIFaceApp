package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"telegram-ai-face-bot/internal/models"
	"telegram-ai-face-bot/internal/payments"
)

type PaymentService struct {
	provider    *payments.PaymentProvider
	userService *UserService
	priceTable  map[string]map[int]int
	subPrices   map[string]int
	notifier    func(userID int64, category string, qty int)
}

func NewPaymentService(provider *payments.PaymentProvider, userService *UserService) *PaymentService {
	return &PaymentService{
		provider:    provider,
		userService: userService,
		priceTable: map[string]map[int]int{
			"image": {
				10:  50,
				50:  240,
				100: 460,
				250: 999,
				500: 1910,
			},
			"text": {
				10:  10,
				50:  50,
				100: 100,
				250: 250,
				500: 500,
			},
			"music": {
				1:   45,
				5:   219,
				10:  429,
				50:  1999,
				100: 3899,
			},
			"video": {
				1:   45,
				5:   219,
				10:  429,
				50:  1999,
				100: 3899,
			},
		},
		subPrices: map[string]int{
			"mini":  1,
			"start": 2,
			"pro":   3,
		},
	}
}

func (s *PaymentService) CreateExtrasPayment(userID int64, category string, qty int) (*payments.PaymentResponse, error) {
	price, err := s.priceFor(category, qty)
	if err != nil {
		return nil, err
	}

	discount := s.userService.SubscriptionDiscount(userID)
	if discount < 1.0 {
		price = int(float64(price) * discount)
	}

	req := payments.PaymentRequest{
		UserID:   userID,
		Amount:   float64(price),
		Currency: "RUB",
		Metadata: map[string]any{
			"user_id":  userID,
			"category": category,
			"qty":      qty,
		},
	}

	return s.provider.CreatePayment(req)
}

func (s *PaymentService) CreateSubscriptionPayment(userID int64, plan string, days int) (*payments.PaymentResponse, error) {
	plan = strings.ToLower(strings.TrimSpace(plan))
	price, ok := s.subPrices[plan]
	if !ok {
		return nil, fmt.Errorf("unknown subscription plan: %s", plan)
	}
	if days < 1 {
		days = 7
	}
	req := payments.PaymentRequest{
		UserID:   userID,
		Amount:   float64(price),
		Currency: "RUB",
		Metadata: map[string]any{
			"user_id":  userID,
			"category": "subscription:" + plan,
			"qty":      days,
		},
	}
	return s.provider.CreatePayment(req)
}

func (s *PaymentService) priceFor(category string, qty int) (int, error) {
	if catPrices, ok := s.priceTable[category]; ok {
		if price, ok := catPrices[qty]; ok {
			return price, nil
		}
	}
	return 0, fmt.Errorf("неизвестный пакет: %s x %d", category, qty)
}

func (s *PaymentService) PriceListText() string {
	var lines []string
	for cat, prices := range s.priceTable {
		lines = append(lines, fmt.Sprintf("%s:", cat))
		for qty, price := range prices {
			lines = append(lines, fmt.Sprintf("  %d — %d руб", qty, price))
		}
	}
	return strings.Join(lines, "\n")
}

func (s *PaymentService) ProcessSuccessfulPayment(userID int64, category string, qty int) error {
	if _, err := s.userService.GetOrCreateUser(userID, "", "", "", ""); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	if strings.HasPrefix(category, "subscription:") {
		plan := strings.TrimPrefix(category, "subscription:")
		if err := s.userService.SetSubscription(userID, plan, qty); err != nil {
			return err
		}
		if s.notifier != nil {
			s.notifier(userID, category, qty)
		}
		return nil
	}
	var qCat models.QuotaCategory
	switch category {
	case "text":
		qCat = models.QuotaCategoryText
	case "image":
		qCat = models.QuotaCategoryImage
	case "music":
		qCat = models.QuotaCategoryMusic
	case "video":
		qCat = models.QuotaCategoryVideo
	default:
		return fmt.Errorf("unknown category %s", category)
	}
	if err := s.userService.AddExtraQuota(userID, qCat, qty); err != nil {
		return err
	}
	if s.notifier != nil {
		s.notifier(userID, category, qty)
	}
	return nil
}

func ParseQty(s string) (int, error) {
	return strconv.Atoi(s)
}
func (s *PaymentService) HandleYooKassaWebhook(body []byte, authHeader string) error {
	data, err := s.provider.ParseYooKassaWebhook(body, authHeader)
	if err != nil {
		return err
	}
	log.Printf("yookassa webhook ok: payment_id=%s user=%d category=%s qty=%d", data.PaymentID, data.UserID, data.Category, data.Qty)
	return s.ProcessSuccessfulPayment(data.UserID, data.Category, data.Qty)
}

func (s *PaymentService) SetNotifier(fn func(userID int64, category string, qty int)) {
	s.notifier = fn
}
