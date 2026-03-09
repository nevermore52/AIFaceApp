package telegram

import (
	"fmt"
	"log"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

type broadcastAlbumBuffer struct {
	chatID  int64
	photos  []string
	caption string
	timer   *time.Timer
	mu      sync.Mutex
}

var (
	broadcastAlbumBuffers = make(map[string]*broadcastAlbumBuffer)
	broadcastAlbumMu      sync.Mutex
)

func (b *Bot) handleAdminBroadcastAlbum(msg *tgmodels.Message) {
	mediaID := msg.MediaGroupID
	if mediaID == "" {
		return
	}

	broadcastAlbumMu.Lock()
	buf := broadcastAlbumBuffers[mediaID]
	if buf == nil {
		buf = &broadcastAlbumBuffer{
			chatID:  msg.Chat.ID,
			photos:  []string{},
			caption: "",
		}
		broadcastAlbumBuffers[mediaID] = buf
	}
	broadcastAlbumMu.Unlock()

	buf.mu.Lock()
	defer buf.mu.Unlock()

	if msg.Photo != nil && len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		buf.photos = append(buf.photos, photo.FileID)
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
		const (
			batchSize  = 500
			numWorkers = 20
		)

		// Collect all users first
		var allUsers []int64
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
				if user != nil && user.TelegramID != 0 {
					allUsers = append(allUsers, user.TelegramID)
				}
			}
		}

		total := len(allUsers)
		if total == 0 {
			b.sendText(chatID, "❌ Нет пользователей для рассылки")
			return
		}

		b.sendText(chatID, fmt.Sprintf("📊 Найдено %d пользователей, начинаю рассылку...", total))

		// Create channels for worker pool
		jobs := make(chan int64, total)
		results := make(chan bool, total)

		// Start workers
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for userID := range jobs {
					var mediaGroup []tgmodels.InputMedia
					for i, photoID := range photos {
						media := &tgmodels.InputMediaPhoto{
							Media: photoID,
						}
						if i == 0 && caption != "" {
							media.Caption = convertBroadcastMarkupToHTML(caption)
							media.ParseMode = tgmodels.ParseModeHTML
						}
						mediaGroup = append(mediaGroup, media)
					}

					_, err := b.api.SendMediaGroup(b.ctx, &tgbot.SendMediaGroupParams{
						ChatID: userID,
						Media:  mediaGroup,
					})
					if err != nil {
						log.Printf("broadcast album send to %d failed: %v", userID, err)
						results <- false
					} else {
						results <- true
					}
					time.Sleep(35 * time.Millisecond)
				}
			}()
		}

		// Send jobs to workers
		go func() {
			for _, userID := range allUsers {
				jobs <- userID
			}
			close(jobs)
		}()

		// Wait for all workers to finish
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		sent := 0
		failed := 0
		for success := range results {
			if success {
				sent++
			} else {
				failed++
			}
		}

		b.sendText(chatID, fmt.Sprintf("✅ Рассылка альбома завершена\nПолучателей: %d\nУспешно: %d\nОшибки: %d", total, sent, failed))
	})
}
