package handlers

import (
	"net/http"
	"strconv"

	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	userService       *services.UserService
	generationService *services.GenerationService
	paymentService    *services.PaymentService
}

func NewAdminHandler(
	userService *services.UserService,
	generationService *services.GenerationService,
	paymentService *services.PaymentService,
) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		generationService: generationService,
		paymentService:    paymentService,
	}
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	userStats, err := h.userService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user stats"})
		return
	}

	generationStats, err := h.generationService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generation stats"})
		return
	}

	paymentStats, err := h.paymentService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payment stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":       userStats,
		"generations": generationStats,
		"payments":    paymentStats,
	})
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	users, total, err := h.userService.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		IsAdmin   *bool `json:"is_admin"`
		IsBlocked *bool `json:"is_blocked"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	if req.IsBlocked != nil {
		user.IsBlocked = *req.IsBlocked
	}

	if err := h.userService.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) GetGenerations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	generations, total, err := h.generationService.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get generations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   generations,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) GetPayments(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	payments, total, err := h.paymentService.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   payments,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AdminHandler) GetCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Category management via web is not yet implemented",
	})
}

func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Category management via web is not yet implemented",
	})
}
