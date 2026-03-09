package telegram

import (
	"fmt"
	"log"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
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

	var rows [][]tgmodels.InlineKeyboardButton

	// Duration buttons
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		klingAnimateDurationButton("⚡ 5 сек", "5", duration),
		klingAnimateDurationButton("🎬 10 сек", "10", duration),
	})

	// Sound buttons
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		klingAnimateSoundButton("🔊 Со звуком", "true", sound),
		klingAnimateSoundButton("🔇 Без звука", "false", sound),
	})

	// Calculate total cost
	totalCost := b.getKlingVideoCost(userID)

	// Animate button with total cost
	animateLabel := fmt.Sprintf("💰 Оживить (%d ген.)", totalCost)
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newInlineKeyboardButtonData(animateLabel, "kling_animate_start"),
	})

	text := "🎬 Настройки оживления фото (Kling 2.6)\n\nВыберите параметры:"
	markup := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		// Use editMessageTextOrCaption to handle both text and photo messages
		if err := b.editMessageTextOrCaption(chatID, messageID, text, markup); err != nil {
			log.Printf("Failed to edit kling animate menu: %v", err)
		}
		return
	}

	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send kling animate menu: %v", err)
	}
}

func klingAnimateDurationButton(label, duration, current string) tgmodels.InlineKeyboardButton {
	if duration == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "kling_animate_duration:"+duration)
}

func klingAnimateSoundButton(label, sound, current string) tgmodels.InlineKeyboardButton {
	if sound == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "kling_animate_sound:"+sound)
}

// handleKlingAnimateStart обрабатывает нажатие кнопки "Оживить" в меню Kling 2.6
func (b *Bot) handleKlingAnimateStart(chatID int64, userID int64, callback *tgmodels.CallbackQuery) {
	cbMsg := getCallbackMessage(callback)
	if cbMsg == nil || cbMsg.Photo == nil || len(cbMsg.Photo) == 0 {
		b.sendErrorMessage(chatID, "Не удалось найти фото в сообщении.")
		return
	}

	// Берём самое большое фото из сообщения
	photo := cbMsg.Photo[len(cbMsg.Photo)-1]
	file, err := b.api.GetFile(b.ctx, &tgbot.GetFileParams{FileID: photo.FileID})
	if err != nil {
		log.Printf("kling_animate_start: failed to get file: %v", err)
		b.sendErrorMessage(chatID, "Не удалось получить фото для оживления.")
		return
	}
	photoURL := b.getFileURL(file)

	klingOpt, ok := findModelOption("kling-2.6/image-to-video")
	if !ok {
		b.sendErrorMessage(chatID, "Модель Kling 2.6 не найдена.")
		return
	}

	b.processVideoGeneration(chatID, userID, photoURL, "оживи фото", klingOpt)
}
