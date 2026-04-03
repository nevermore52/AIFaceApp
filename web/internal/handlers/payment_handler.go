package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService    *services.PaymentService
	webPaymentService *services.WebPaymentService
}

func NewPaymentHandler(paymentService *services.PaymentService, webPaymentService *services.WebPaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService:    paymentService,
		webPaymentService: webPaymentService,
	}
}

func (h *PaymentHandler) GetPackages(c *gin.Context) {
	packages := h.paymentService.GetPackages()
	c.JSON(http.StatusOK, packages)
}

func (h *PaymentHandler) GetPhotoDiscount(c *gin.Context) {
	if h.webPaymentService == nil {
		c.JSON(http.StatusOK, gin.H{"percent": 0, "end_time": 0})
		return
	}
	percent, endTime := h.webPaymentService.GetPhotoDiscount()
	c.JSON(http.StatusOK, gin.H{"percent": percent, "end_time": endTime})
}

func (h *PaymentHandler) GetSubscriptions(c *gin.Context) {
	if h.webPaymentService != nil {
		c.JSON(http.StatusOK, h.webPaymentService.GetSubscriptions())
		return
	}
	c.JSON(http.StatusOK, h.paymentService.GetSubscriptions())
}

type CreatePaymentRequest struct {
	Category string `json:"category" binding:"required"`
	Qty      int    `json:"qty" binding:"required"`
}

type CreateSubscriptionPaymentRequest struct {
	SubscriptionName string `json:"subscription_name" binding:"required"`
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	if h.webPaymentService == nil || !h.webPaymentService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Payment service not configured"})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)
	if u.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram account not linked"})
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	paymentReq := services.CreatePaymentRequest{
		UserID:           *u.TelegramID,
		Category:         req.Category,
		Qty:              req.Qty,
		Username:         u.Username,
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		SubscriptionType: u.SubscriptionType,
	}

	resp, err := h.webPaymentService.CreatePayment(paymentReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   resp.PaymentID,
		"checkout_url": resp.CheckoutURL,
		"amount":       resp.Amount,
	})
}

func (h *PaymentHandler) CreateSubscriptionPayment(c *gin.Context) {
	if h.webPaymentService == nil || !h.webPaymentService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Payment service not configured"})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)
	if u.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram account not linked"})
		return
	}

	var req CreateSubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	paymentReq := services.CreatePaymentRequest{
		UserID:    *u.TelegramID,
		Category:  "subscription:" + req.SubscriptionName,
		Qty:       7,
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}

	resp, err := h.webPaymentService.CreatePayment(paymentReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   resp.PaymentID,
		"checkout_url": resp.CheckoutURL,
		"amount":       resp.Amount,
	})
}

func (h *PaymentHandler) GetPaymentHistory(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	u := user.(*models.User)
	if u.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram account not linked"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	payments, total, err := h.paymentService.GetByUserID(*u.TelegramID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payment history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   payments,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *PaymentHandler) HandleYooKassaWebhook(c *gin.Context) {
	if h.webPaymentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Payment service not configured"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	data, err := h.webPaymentService.ParseWebhook(body)
	if err != nil {
		log.Printf("YooKassa webhook parse error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.webPaymentService.AddQuota(data.UserID, data.Category, data.Qty); err != nil {
		log.Printf("Failed to add quota for user %d: %v", data.UserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add quota"})
		return
	}

	h.webPaymentService.AddReferralBonus(data.UserID, data.Category, data.Qty)

	if err := h.webPaymentService.RecordPayment(data); err != nil {
		log.Printf("Failed to record payment: %v", err)
	}

	log.Printf("YooKassa payment processed: user=%d category=%s qty=%d amount=%.2f",
		data.UserID, data.Category, data.Qty, data.Amount)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
