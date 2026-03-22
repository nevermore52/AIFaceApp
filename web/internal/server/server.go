package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"telegram-ai-face-bot/web/internal/config"
	"telegram-ai-face-bot/web/internal/handlers"
	"telegram-ai-face-bot/web/internal/middleware"
	"telegram-ai-face-bot/web/internal/repository"
	"telegram-ai-face-bot/web/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg        *config.Config
	httpServer *http.Server
	router     *gin.Engine
}

func New(cfg *config.Config, db *sql.DB) *Server {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL, "https://telegram.org"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Telegram-Init-Data"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	userRepo := repository.NewUserRepository(db)
	quotaRepo := repository.NewQuotaRepository(db)
	generationRepo := repository.NewGenerationRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	authService := services.NewAuthService(cfg, userRepo, sessionRepo)
	userService := services.NewUserService(userRepo, quotaRepo)
	generationService := services.NewGenerationService(generationRepo)
	paymentService := services.NewPaymentService(paymentRepo, userRepo, quotaRepo)

	authTokenRepo := repository.NewAuthTokenRepository(db)
	authHandler := handlers.NewAuthHandler(authService, cfg, authTokenRepo)
	userHandler := handlers.NewUserHandler(userService)
	generationHandler := handlers.NewGenerationHandler(generationService)
	paymentHandler := handlers.NewPaymentHandler(paymentService)
	adminHandler := handlers.NewAdminHandler(userService, generationService, paymentService)

	authMiddleware := middleware.NewAuthMiddleware(authService)

	api := router.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		auth := api.Group("/auth")
		{
			auth.POST("/telegram", authHandler.TelegramLogin)
			auth.POST("/telegram/miniapp", authHandler.TelegramMiniAppLogin)
			auth.POST("/telegram/web-token", authHandler.CreateWebToken)
			auth.GET("/telegram/web-token/:token/status", authHandler.GetWebTokenStatus)
			auth.POST("/telegram/web-token/confirm", authHandler.ConfirmWebToken)
			auth.GET("/google", authHandler.GoogleLogin)
			auth.GET("/google/callback", authHandler.GoogleCallback)
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authMiddleware.RequireAuth(), authHandler.Logout)
		}

		protected := api.Group("")
		protected.Use(authMiddleware.RequireAuth())
		{
			protected.GET("/me", userHandler.GetCurrentUser)
			protected.PUT("/me", userHandler.UpdateProfile)
			protected.GET("/me/quota", userHandler.GetQuota)
			protected.GET("/me/history", generationHandler.GetUserHistory)

			protected.GET("/generations", generationHandler.GetUserGenerations)
			protected.GET("/generations/:id", generationHandler.GetGeneration)

			protected.GET("/payments/packages", paymentHandler.GetPackages)
			protected.GET("/payments/subscriptions", paymentHandler.GetSubscriptions)
			protected.POST("/payments/create", paymentHandler.CreatePayment)
			protected.GET("/payments/history", paymentHandler.GetPaymentHistory)
		}

		admin := api.Group("/admin")
		admin.Use(authMiddleware.RequireAuth(), authMiddleware.RequireAdmin())
		{
			admin.GET("/stats", adminHandler.GetStats)
			admin.GET("/users", adminHandler.GetUsers)
			admin.GET("/users/:id", adminHandler.GetUser)
			admin.PUT("/users/:id", adminHandler.UpdateUser)
			admin.GET("/generations", adminHandler.GetGenerations)
			admin.GET("/payments", adminHandler.GetPayments)
			admin.GET("/categories", adminHandler.GetCategories)
			admin.PUT("/categories/:category", adminHandler.UpdateCategory)
		}
	}

	return &Server{
		cfg:    cfg,
		router: router,
		httpServer: &http.Server{
			Addr:         cfg.ServerHost + ":" + cfg.ServerPort,
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
}
