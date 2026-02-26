package services

import (
	"database/sql"
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
	notifier    func(userID int64, category string, qty int, amount float64, paymentID string)
}

func NewPaymentService(provider *payments.PaymentProvider, userService *UserService) *PaymentService {
	return &PaymentService{
		provider:    provider,
		userService: userService,
		priceTable: map[string]map[int]int{
			"image": {
				10:  49,
				50:  225,
				100: 425,
				250: 880,
				500: 1710,
			},
			"text": {
				10:  10,
				50:  45,
				100: 80,
				250: 200,
				500: 345,
			},
			"music": {
				1:   25,
				5:   119,
				10:  202,
				50:  812,
				100: 1599,
			},
			"video": {
				1:	 99,
				5:   379,
				10:  699,
				25:  1650,
				50:  3100,
				100: 5900,
			},
		},
		subPrices: map[string]int{
			"mini":  299,
			"start": 499,
			"pro":   799,
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

	username := ""
	firstName := ""
	lastName := ""
	if buyer, err := s.userService.GetUserByTelegramID(userID); err == nil && buyer != nil {
		username = buyer.Username
		firstName = buyer.FirstName
		lastName = buyer.LastName
	}

	req := payments.PaymentRequest{
		UserID:   userID,
		Amount:   float64(price),
		Currency: "RUB",
		Metadata: map[string]any{
			"user_id":    userID,
			"category":   category,
			"qty":        qty,
			"username":   username,
			"first_name": firstName,
			"last_name":  lastName,
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

	username := ""
	firstName := ""
	lastName := ""
	if buyer, err := s.userService.GetUserByTelegramID(userID); err == nil && buyer != nil {
		username = buyer.Username
		firstName = buyer.FirstName
		lastName = buyer.LastName
	}
	req := payments.PaymentRequest{
		UserID:   userID,
		Amount:   float64(price),
		Currency: "RUB",
		Metadata: map[string]any{
			"user_id":    userID,
			"category":   "subscription:" + plan,
			"qty":        days,
			"username":   username,
			"first_name": firstName,
			"last_name":  lastName,
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

func (s *PaymentService) ExtrasPrice(category string, qty int) (int, bool) {
	price, err := s.priceFor(category, qty)
	if err != nil {
		return 0, false
	}
	return price, true
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

func (s *PaymentService) SubscriptionPrice(plan string) (int, bool) {
	plan = strings.ToLower(strings.TrimSpace(plan))
	price, ok := s.subPrices[plan]
	return price, ok
}

func (s *PaymentService) ProcessSuccessfulPayment(userID int64, category string, qty int, amount float64, paymentID string, username string, firstName string, lastName string) error {
	buyer, err := s.userService.GetUserByTelegramID(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			if _, err := s.userService.CreateUser(userID, "", "", "", "", nil); err != nil {
				return fmt.Errorf("ensure user: %w", err)
			}
			buyer, _ = s.userService.GetUserByTelegramID(userID)
		} else {
			return fmt.Errorf("get user: %w", err)
		}
	}

	// Записываем оплаченный платёж в таблицу completed_payments
	storedUsername := ""
	storedFirstName := ""
	storedLastName := ""
	if buyer != nil {
		storedUsername = buyer.Username
		storedFirstName = buyer.FirstName
		storedLastName = buyer.LastName
	}
	if storedUsername == "" {
		storedUsername = username
	}
	if storedFirstName == "" {
		storedFirstName = firstName
	}
	if storedLastName == "" {
		storedLastName = lastName
	}

	if buyer != nil && (storedUsername != buyer.Username || storedFirstName != buyer.FirstName || storedLastName != buyer.LastName) {
		if _, err := s.userService.UpdateUserInfo(userID, storedUsername, storedFirstName, storedLastName, buyer.LanguageCode); err != nil {
			log.Printf("UpdateUserInfo error: user=%d err=%v", userID, err)
		}
	}

	if err := s.userService.RecordPayment(userID, storedUsername, storedFirstName, storedLastName, paymentID, category, qty, amount); err != nil {
		log.Printf("RecordPayment error: user=%d payment=%s err=%v", userID, paymentID, err)
	}

	if strings.HasPrefix(category, "subscription:") {
		plan := strings.TrimPrefix(category, "subscription:")
		if err := s.userService.SetSubscription(userID, plan, qty); err != nil {
			return err
		}
		if s.notifier != nil {
			s.notifier(userID, category, qty, amount, paymentID)
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
	buyer, err = s.userService.GetUserByTelegramID(userID)
	if err == nil && buyer.ReferrerID != nil {
		if err := s.userService.AddReferralPurchaseBonus(*buyer.ReferrerID, qCat, qty); err != nil {
			log.Printf("failed to add referral purchase bonus: buyer=%d referrer=%d category=%s qty=%d err=%v", userID, *buyer.ReferrerID, category, qty, err)
		}
	}
	if s.notifier != nil {
		s.notifier(userID, category, qty, amount, paymentID)
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
	log.Printf("yookassa webhook ok: payment_id=%s user=%d category=%s qty=%d amount=%.2f", data.PaymentID, data.UserID, data.Category, data.Qty, data.Amount)
	return s.ProcessSuccessfulPayment(data.UserID, data.Category, data.Qty, data.Amount, data.PaymentID, data.Username, data.FirstName, data.LastName)
}

func (s *PaymentService) SetNotifier(fn func(userID int64, category string, qty int, amount float64, paymentID string)) {
	s.notifier = fn
}
