package telegram

import (
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type broadcastAlbumBuffer struct {
	chatID  int64
	photos  []tgbotapi.FileID
	caption string
	timer   *time.Timer
	mu      sync.Mutex
}

var (
	broadcastAlbumBuffers = make(map[string]*broadcastAlbumBuffer)
	broadcastAlbumMu      sync.Mutex
)

func (b *Bot) handleAdminBroadcastAlbum(msg *tgbotapi.Message) {
	mediaID := msg.MediaGroupID
	if mediaID == "" {
		return
	}

	broadcastAlbumMu.Lock()
	buf := broadcastAlbumBuffers[mediaID]
	if buf == nil {
		buf = &broadcastAlbumBuffer{
			chatID:  msg.Chat.ID,
			photos:  []tgbotapi.FileID{},
			caption: "",
		}
		broadcastAlbumBuffers[mediaID] = buf
	}
	broadcastAlbumMu.Unlock()

	buf.mu.Lock()
	defer buf.mu.Unlock()

	if msg.Photo != nil && len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		buf.photos = append(buf.photos, tgbotapi.FileID(photo.FileID))
	}

	if msg.Caption != "" {
		buf.caption = msg.Caption
	}

	if buf.timer != nil {
		buf.timer.Stop()
	}

	buf.timer = time.AfterFunc(1*time.Second, func() {
		b.processBroadcastAlbum(mediaID)
	})
}

func (b *Bot) processBroadcastAlbum(mediaID string) {
	broadcastAlbumMu.Lock()
	buf := broadcastAlbumBuffers[mediaID]
	delete(broadcastAlbumBuffers, mediaID)
	broadcastAlbumMu.Unlock()

	if buf == nil {
		return
	}

	buf.mu.Lock()
	photos := buf.photos
	caption := buf.caption
	chatID := buf.chatID
	buf.mu.Unlock()

	if len(photos) == 0 {
		b.sendErrorMessage(chatID, "Альбом пуст, рассылка отменена")
		return
	}

	if len(photos) > 10 {
		photos = photos[:10]
	}

	b.sendText(chatID, fmt.Sprintf("📣 Запускаю рассылку альбома (%d фото)...", len(photos)))

	b.goLimited(func() {
		const batchSize = 200
		total := 0
		sent := 0
		failed := 0

		for offset := 0; ; offset += batchSize {
			users, err := b.userService.GetAllUsers(batchSize, offset)
			if err != nil {
				b.sendErrorMessage(chatID, fmt.Sprintf("Ошибка при получении пользователей: %v", err))
				return
			}
			if len(users) == 0 {
				break
			}

			for _, user := range users {
				if user == nil || user.TelegramID == 0 {
					continue
				}
				total++

				var mediaGroup []interface{}
				for i, photoID := range photos {
					media := tgbotapi.NewInputMediaPhoto(photoID)
					if i == 0 && caption != "" {
						media.Caption = convertBroadcastMarkupToHTML(caption)
						media.ParseMode = tgbotapi.ModeHTML
					}
					mediaGroup = append(mediaGroup, media)
				}

				config := tgbotapi.NewMediaGroup(user.TelegramID, mediaGroup)
				if _, err := b.api.Send(config); err != nil {
					failed++
					log.Printf("broadcast album send to %d failed: %v", user.TelegramID, err)
				} else {
					sent++
				}
				time.Sleep(50 * time.Millisecond)
			}
		}

		b.sendText(chatID, fmt.Sprintf("✅ Рассылка альбома завершена\nПолучателей: %d\nУспешно: %d\nОшибки: %d", total, sent, failed))
	})
}
