package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-ai-face-bot/internal/bot"
	"telegram-ai-face-bot/internal/config"
	"telegram-ai-face-bot/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	tgBot, err := bot.NewBot(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	go func() {
		log.Println("Bot is starting...")
		if err := tgBot.Start(); err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down bot...")
	tgBot.Stop()
}
