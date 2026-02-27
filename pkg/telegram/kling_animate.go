package telegram

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// sendKlingAnimateMenu отправляет меню с фильтрами Kling 2.6 для оживления фото
func (b *Bot) sendKlingAnimateMenu(chatID int64, userID int64, messageID int) {
	duration := b.getUserVideoDuration(userID)
	// Kling supports only 5 and 10 seconds
	if duration != "5" && duration != "10" {
		duration = "5"
		b.setUserVideoDuration(userID, "5") // Set default to 5 seconds
	}
	sound := b.getUserVideoSound(userID)
	// Ensure default is without sound
	if sound == "" {
		sound = "false"
		b.setUserVideoSound(userID, "false")
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	// Duration buttons
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		klingAnimateDurationButton("⚡ 5 сек", "5", duration),
		klingAnimateDurationButton("🎬 10 сек", "10", duration),
	})

	// Sound buttons
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		klingAnimateSoundButton("🔊 Со звуком", "true", sound),
		klingAnimateSoundButton("🔇 Без звука", "false", sound),
	})

	// Calculate total cost
	totalCost := b.getKlingVideoCost(userID)

	// Animate button with total cost
	animateLabel := fmt.Sprintf("💰 Оживить (%d ген.)", totalCost)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(animateLabel, "kling_animate_start"),
	})

	text := "🎬 Настройки оживления фото (Kling 2.6)\n\nВыберите параметры:"
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		edited := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, markup)
		if _, err := b.api.Send(edited); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			log.Printf("Failed to edit kling animate menu: %v", err)
		}
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send kling animate menu: %v", err)
	}
}

func klingAnimateDurationButton(label, duration, current string) tgbotapi.InlineKeyboardButton {
	if duration == current {
		label = "✅ " + label
	}
	return tgbotapi.NewInlineKeyboardButtonData(label, "kling_animate_duration:"+duration)
}

func klingAnimateSoundButton(label, sound, current string) tgbotapi.InlineKeyboardButton {
	if sound == current {
		label = "✅ " + label
	}
	return tgbotapi.NewInlineKeyboardButtonData(label, "kling_animate_sound:"+sound)
}

// handleKlingAnimateStart обрабатывает нажатие кнопки "Оживить" в меню Kling 2.6
func (b *Bot) handleKlingAnimateStart(chatID int64, userID int64, callback *tgbotapi.CallbackQuery) {
	if callback.Message == nil || callback.Message.Photo == nil || len(callback.Message.Photo) == 0 {
		b.sendErrorMessage(chatID, "Не удалось найти фото в сообщении.")
		return
	}

	// Берём самое большое фото из сообщения
	photo := callback.Message.Photo[len(callback.Message.Photo)-1]
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		log.Printf("kling_animate_start: failed to get file: %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить фото для оживления.")
		return
	}
	photoURL := file.Link(b.cfg.TelegramToken)

	klingOpt, ok := findModelOption("kling-2.6/image-to-video")
	if !ok {
		b.sendErrorMessage(chatID, "Модель Kling 2.6 не найдена.")
		return
	}

	b.processVideoGeneration(chatID, userID, photoURL, "оживи фото", klingOpt)
}
