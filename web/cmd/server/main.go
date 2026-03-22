package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-ai-face-bot/web/internal/config"
	"telegram-ai-face-bot/web/internal/database"
	"telegram-ai-face-bot/web/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	srv := server.New(cfg, db)

	go func() {
		log.Printf("Starting web server on %s:%s", cfg.ServerHost, cfg.ServerPort)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	srv.Stop()
}
