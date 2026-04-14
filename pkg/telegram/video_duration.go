package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgmodels "github.com/go-telegram/bot/models"
)

func (b *Bot) getUserVideoDuration(userID int64) string {
	duration, err := b.redisClient.GetUserVideoDuration(userID)
	if err != nil {
		log.Printf("getUserVideoDuration err: %v", err)
	}
	if duration == "" {
		return "5"
	}
	switch duration {
	case "5", "10", "15":
		return duration
	default:
		return "5"
	}
}

func (b *Bot) setUserVideoDuration(userID int64, duration string) {
	switch duration {
	case "5", "10", "15":
		// valid
	default:
		duration = "5"
	}
	if err := b.redisClient.SetUserVideoDuration(userID, duration); err != nil {
		log.Printf("setUserVideoDuration err: %v", err)
	}
}

func (b *Bot) sendVideoDurationMenu(chatID int64, userID int64, messageID int) {
	current := b.getUserVideoDuration(userID)

	var rows [][]tgmodels.InlineKeyboardButton
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		durationOptionButton("⚡ 5 сек (20 токенов)", "5", current),
		durationOptionButton("🎬 10 сек (40 токенов)", "10", current),
	})
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		durationOptionButton("🎥 15 сек (60 токенов)", "15", current),
	})

	backCallback := "models_menu:video"
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newInlineKeyboardButtonData("◀️ Назад", backCallback),
	})

	costMap := map[string]int{"5": 20, "10": 40, "15": 60}
	cost := costMap[current]
	text := fmt.Sprintf("⏱️ Длительность видео: %s сек\n💰 Стоимость: %d видео токенов", current, cost)
	markup := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageText(chatID, messageID, text, &markup); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			log.Printf("Failed to edit video duration menu: %v", err)
		}
		return
	}
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send video duration menu: %v", err)
	}
}

func durationOptionButton(label, duration, current string) tgmodels.InlineKeyboardButton {
	if duration == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "duration_set:"+duration)
}

func (b *Bot) sendVideoDurationMenuKling(chatID int64, userID int64, messageID int) {
	current := b.getUserVideoDuration(userID)
	// Kling supports only 5 and 10 seconds
	if current != "5" && current != "10" {
		current = "5"
	}

	var rows [][]tgmodels.InlineKeyboardButton
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		durationOptionButtonKling("⚡ 5 сек (10 токенов)", "5", current),
		durationOptionButtonKling("🎬 10 сек (20 токенов)", "10", current),
	})

	backCallback := "models_menu:video"
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newInlineKeyboardButtonData("◀️ Назад", backCallback),
	})

	// Kling: 5s = 10 токенов, 10s = 20 токенов
	costMap := map[string]int{"5": 10, "10": 20}
	cost := costMap[current]
	if cost == 0 {
		cost = 10
	}
	sound := b.getUserVideoSound(userID)
	soundNote := ""
	if sound == "true" {
		cost *= 2
		soundNote = " (со звуком x2)"
	}
	text := fmt.Sprintf("⏱️ Длительность видео: %s сек\n💰 Стоимость: %d видео токенов%s", current, cost, soundNote)
	markup := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageText(chatID, messageID, text, &markup); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			log.Printf("Failed to edit video duration menu kling: %v", err)
		}
		return
	}
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send video duration menu kling: %v", err)
	}
}

func durationOptionButtonKling(label, duration, current string) tgmodels.InlineKeyboardButton {
	if duration == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "duration_set:"+duration)
}

func (b *Bot) getVideoDurationCost(userID int64) int {
	duration := b.getUserVideoDuration(userID)
	durationInt, err := strconv.Atoi(duration)
	if err != nil || durationInt <= 0 {
		durationInt = 5
	}
	// 5 сек = 20 видео токенов, 10 сек = 40, 15 сек = 60
	return (durationInt / 5) * 20
}

func (b *Bot) getKlingVideoCost(userID int64) int {
	duration := b.getUserVideoDuration(userID)
	sound := b.getUserVideoSound(userID)

	// Kling: 5 сек = 10 видео токенов, 10 сек = 20 токенов
	baseCost := 10
	if duration == "10" {
		baseCost = 20
	}

	// Со звуком = x2
	if sound == "true" {
		baseCost *= 2
	}

	return baseCost
}

// getVideoTotalCost returns total cost for video models considering user settings
func (b *Bot) getVideoTotalCost(userID int64, modelID string) int {
	switch modelID {
	case "wan/2-6-image-to-video":
		return b.getVideoDurationCost(userID)
	case "kling-2.6/image-to-video":
		return b.getKlingVideoCost(userID)
	default:
		if cost := modelRequestCost(modelID); cost > 0 {
			return cost
		}
		return 1
	}
}

func (b *Bot) getUserVideoResolution(userID int64) string {
	resolution, err := b.redisClient.GetUserVideoResolution(userID)
	if err != nil {
		log.Printf("getUserVideoResolution err: %v", err)
	}
	if resolution == "" {
		return "720p"
	}
	switch resolution {
	case "720p", "1080p":
		return resolution
	default:
		return "720p"
	}
}

func (b *Bot) setUserVideoResolution(userID int64, resolution string) {
	switch resolution {
	case "720p", "1080p":
		// valid
	default:
		resolution = "720p"
	}
	if err := b.redisClient.SetUserVideoResolution(userID, resolution); err != nil {
		log.Printf("setUserVideoResolution err: %v", err)
	}
}

func (b *Bot) sendVideoResolutionMenu(chatID int64, userID int64, messageID int) {
	current := b.getUserVideoResolution(userID)

	var rows [][]tgmodels.InlineKeyboardButton
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		resolutionOptionButton("📺 720p", "720p", current),
		resolutionOptionButton("🎬 1080p", "1080p", current),
	})

	backCallback := "models_menu:video"
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newInlineKeyboardButtonData("◀️ Назад", backCallback),
	})

	text := fmt.Sprintf("📺 Разрешение видео: %s", current)
	markup := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageText(chatID, messageID, text, &markup); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			log.Printf("Failed to edit video resolution menu: %v", err)
		}
		return
	}
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send video resolution menu: %v", err)
	}
}

func resolutionOptionButton(label, resolution, current string) tgmodels.InlineKeyboardButton {
	if resolution == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "resolution_set:"+resolution)
}

func (b *Bot) getUserVideoSound(userID int64) string {
	sound, err := b.redisClient.GetUserVideoSound(userID)
	if err != nil {
		log.Printf("getUserVideoSound err: %v", err)
	}
	if sound == "" {
		return "false"
	}
	switch sound {
	case "true", "false":
		return sound
	default:
		return "false"
	}
}

func (b *Bot) setUserVideoSound(userID int64, sound string) {
	switch sound {
	case "true", "false":
		// valid
	default:
		sound = "false"
	}
	if err := b.redisClient.SetUserVideoSound(userID, sound); err != nil {
		log.Printf("setUserVideoSound err: %v", err)
	}
}

func (b *Bot) sendVideoSoundMenu(chatID int64, userID int64, messageID int) {
	current := b.getUserVideoSound(userID)

	var rows [][]tgmodels.InlineKeyboardButton
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		soundOptionButton("🔊 Со звуком", "true", current),
		soundOptionButton("🔇 Без звука", "false", current),
	})

	backCallback := "models_menu:video"
	rows = append(rows, []tgmodels.InlineKeyboardButton{
		newInlineKeyboardButtonData("◀️ Назад", backCallback),
	})

	soundLabel := "Без звука"
	if current == "true" {
		soundLabel = "Со звуком"
	}
	text := fmt.Sprintf("🔊 Звук в видео: %s", soundLabel)
	markup := newInlineKeyboardMarkup(rows...)

	if messageID > 0 {
		if err := b.editMessageText(chatID, messageID, text, &markup); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "message is not modified") {
				return
			}
			log.Printf("Failed to edit video sound menu: %v", err)
		}
		return
	}
	msg := newMessageConfig(chatID, text)
	msg.ReplyMarkup = markup
	if _, err := b.sendMsg(msg); err != nil {
		log.Printf("Failed to send video sound menu: %v", err)
	}
}

func soundOptionButton(label, sound, current string) tgmodels.InlineKeyboardButton {
	if sound == current {
		label = "✅ " + label
	}
	return newInlineKeyboardButtonData(label, "sound_set:"+sound)
}

// Photo resolution helpers (for nano-banana-pro and nano-banana-2)
func (b *Bot) getUserPhotoResolution(userID int64) string {
	resolution, err := b.redisClient.GetUserPhotoResolution(userID)
	if err != nil {
		log.Printf("getUserPhotoResolution err: %v", err)
	}
	if resolution == "" {
		return "1K"
	}
	switch resolution {
	case "1K", "2K", "4K":
		return resolution
	default:
		return "1K"
	}
}

// getUserPhotoResolutionPro returns resolution for nano-banana-pro (only 2K and 4K, no 1K)
func (b *Bot) getUserPhotoResolutionPro(userID int64) string {
	resolution, err := b.redisClient.GetUserPhotoResolution(userID)
	if err != nil {
		log.Printf("getUserPhotoResolutionPro err: %v", err)
	}
	// For Pro model, only 2K and 4K are allowed, default is 2K
	switch resolution {
	case "2K", "4K":
		return resolution
	default:
		return "2K"
	}
}

func (b *Bot) setUserPhotoResolution(userID int64, resolution string) {
	switch resolution {
	case "1K", "2K", "4K":
		// valid
	default:
		resolution = "1K"
	}
	if err := b.redisClient.SetUserPhotoResolution(userID, resolution); err != nil {
		log.Printf("setUserPhotoResolution err: %v", err)
	}
}

// getPhotoResolutionCost returns the generation cost based on resolution and model
func (b *Bot) getPhotoResolutionCost(userID int64, modelID string) int {
	if modelID == "google/nano-banana-pro" {
		resolution := b.getUserPhotoResolutionPro(userID)
		// Pro: 2K=4, 4K=5 (no 1K)
		switch resolution {
		case "4K":
			return 5
		default:
			return 4
		}
	}
	resolution := b.getUserPhotoResolution(userID)
	// nano-banana-2: 1K=2, 2K=3, 4K=4
	switch resolution {
	case "2K":
		return 3
	case "4K":
		return 4
	default:
		return 2
	}
}

// Google search helpers (for nano-banana-2)
func (b *Bot) getUserGoogleSearch(userID int64) string {
	val, err := b.redisClient.GetUserGoogleSearch(userID)
	if err != nil {
		log.Printf("getUserGoogleSearch err: %v", err)
	}
	if val == "" {
		return "false"
	}
	switch val {
	case "true", "false":
		return val
	default:
		return "false"
	}
}

func (b *Bot) setUserGoogleSearch(userID int64, enabled string) {
	switch enabled {
	case "true", "false":
		// valid
	default:
		enabled = "false"
	}
	if err := b.redisClient.SetUserGoogleSearch(userID, enabled); err != nil {
		log.Printf("setUserGoogleSearch err: %v", err)
	}
}
