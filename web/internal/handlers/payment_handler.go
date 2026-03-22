package handlers

import (
	"net/http"
	"strconv"

	"telegram-ai-face-bot/web/internal/models"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService *services.PaymentService
}

func NewPaymentHandler(paymentService *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) GetPackages(c *gin.Context) {
	packages := h.paymentService.GetPackages()
	c.JSON(http.StatusOK, packages)
}

func (h *PaymentHandler) GetSubscriptions(c *gin.Context) {
	subscriptions := h.paymentService.GetSubscriptions()
	c.JSON(http.StatusOK, subscriptions)
}

type CreatePaymentRequest struct {
	Category string `json:"category" binding:"required"`
	Qty      int    `json:"qty" binding:"required"`
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	price, ok := h.paymentService.GetPrice(req.Category, req.Qty)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid package"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": req.Category,
		"qty":      req.Qty,
		"price":    price,
		"message":  "Payment creation via web is not yet implemented. Please use Telegram bot.",
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
