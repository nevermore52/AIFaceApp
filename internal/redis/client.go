package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"telegram-ai-face-bot/internal/config"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
	ctx context.Context
}

type UserContext struct {
	PhotoURL string   `json:"photo_url"`
	Messages []string `json:"messages"`
}

func (c *Client) SetUserChatStyle(userID int64, style string) error {
	key := fmt.Sprintf("user:%d:chat_style", userID)
	return c.rdb.Set(c.ctx, key, style, 0).Err()
}

func (c *Client) GetUserChatStyle(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:chat_style", userID)
	style, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return style, nil
}
func (c *Client) SetUserMusicStyle(userID int64, style string) error {
	key := fmt.Sprintf("user:%d:music_style", userID)
	return c.rdb.Set(c.ctx, key, style, 0).Err()
}

func (c *Client) GetUserMusicStyle(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:music_style", userID)
	style, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return style, nil
}
func (c *Client) SetUserModel(userID int64, model string) error {
	key := fmt.Sprintf("user:%d:model", userID)
	return c.rdb.Set(c.ctx, key, model, 0).Err()
}
func (c *Client) GetUserModel(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:model", userID)
	model, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return model, nil
}

func (c *Client) SetUserAspectRatio(userID int64, ratio string) error {
	key := fmt.Sprintf("user:%d:aspect_ratio", userID)
	return c.rdb.Set(c.ctx, key, ratio, 0).Err()
}

func (c *Client) GetUserAspectRatio(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:aspect_ratio", userID)
	ratio, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return ratio, nil
}

func (c *Client) SetUserVideoDuration(userID int64, duration string) error {
	key := fmt.Sprintf("user:%d:video_duration", userID)
	return c.rdb.Set(c.ctx, key, duration, 0).Err()
}

func (c *Client) GetUserVideoDuration(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:video_duration", userID)
	duration, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return duration, nil
}

func (c *Client) SetUserVideoResolution(userID int64, resolution string) error {
	key := fmt.Sprintf("user:%d:video_resolution", userID)
	return c.rdb.Set(c.ctx, key, resolution, 0).Err()
}

func (c *Client) GetUserVideoResolution(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:video_resolution", userID)
	resolution, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return resolution, nil
}

func (c *Client) SetUserVideoSound(userID int64, sound string) error {
	key := fmt.Sprintf("user:%d:video_sound", userID)
	return c.rdb.Set(c.ctx, key, sound, 0).Err()
}

func (c *Client) GetUserVideoSound(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:video_sound", userID)
	sound, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return sound, nil
}

func (c *Client) SetUserPhotoResolution(userID int64, resolution string) error {
	key := fmt.Sprintf("user:%d:photo_resolution", userID)
	return c.rdb.Set(c.ctx, key, resolution, 0).Err()
}

func (c *Client) GetUserPhotoResolution(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:photo_resolution", userID)
	resolution, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return resolution, nil
}

func (c *Client) SetUserGoogleSearch(userID int64, enabled string) error {
	key := fmt.Sprintf("user:%d:google_search", userID)
	return c.rdb.Set(c.ctx, key, enabled, 0).Err()
}

func (c *Client) GetUserGoogleSearch(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:google_search", userID)
	val, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *Client) SetUserLanguage(userID int64, lang string) error {
	key := fmt.Sprintf("user:%d:language", userID)
	return c.rdb.Set(c.ctx, key, lang, 0).Err()
}

func (c *Client) GetUserLanguage(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:language", userID)
	lang, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return lang, nil
}

// MotionControlPending stores character photo URL + prompt while waiting for reference video
type MotionControlPending struct {
	PhotoURL string `json:"photo_url"`
	Prompt   string `json:"prompt"`
}

func (c *Client) SetMotionControlPending(userID int64, photoURL, prompt string) error {
	key := fmt.Sprintf("user:%d:motion_control_pending", userID)
	data := MotionControlPending{PhotoURL: photoURL, Prompt: prompt}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, string(bytes), 10*time.Minute).Err()
}

func (c *Client) GetMotionControlPending(userID int64) (*MotionControlPending, error) {
	key := fmt.Sprintf("user:%d:motion_control_pending", userID)
	val, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data MotionControlPending
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) DeleteMotionControlPending(userID int64) error {
	key := fmt.Sprintf("user:%d:motion_control_pending", userID)
	return c.rdb.Del(c.ctx, key).Err()
}

// MotionControlPendingVideo stores reference video URL + duration for album race-condition handling.
type MotionControlPendingVideo struct {
	VideoURL string `json:"video_url"`
	Duration int    `json:"duration"`
}

// SetMotionControlPendingVideo temporarily stores a reference video URL + duration
// for when the video arrives in the same album as the photo (race condition).
func (c *Client) SetMotionControlPendingVideo(userID int64, videoURL string, duration int) error {
	key := fmt.Sprintf("user:%d:motion_control_pending_video", userID)
	data := MotionControlPendingVideo{VideoURL: videoURL, Duration: duration}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, string(bytes), 30*time.Second).Err()
}

// GetAndDeleteMotionControlPendingVideo retrieves and atomically deletes the pending video info.
func (c *Client) GetAndDeleteMotionControlPendingVideo(userID int64) (*MotionControlPendingVideo, error) {
	key := fmt.Sprintf("user:%d:motion_control_pending_video", userID)
	val, err := c.rdb.GetDel(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data MotionControlPendingVideo
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func NewClient(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.URL,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		rdb: rdb,
		ctx: ctx,
	}, nil
}
func (c *Client) SavePhotoURL(userID int64, photoURL string) error {
	key := fmt.Sprintf("user:%d:context", userID)

	ctx := &UserContext{
		PhotoURL: photoURL,
		Messages: []string{},
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	return c.rdb.Set(c.ctx, key, data, time.Hour).Err()
}
func (c *Client) GetPhotoURL(userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:context", userID)

	data, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("context not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get context: %w", err)
	}

	var ctx UserContext
	if err := json.Unmarshal([]byte(data), &ctx); err != nil {
		return "", fmt.Errorf("failed to unmarshal context: %w", err)
	}

	return ctx.PhotoURL, nil
}
func (c *Client) AddMessage(userID int64, message string) error {
	key := fmt.Sprintf("user:%d:context", userID)

	data, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("context not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get context: %w", err)
	}

	var ctx UserContext
	if err := json.Unmarshal([]byte(data), &ctx); err != nil {
		return fmt.Errorf("failed to unmarshal context: %w", err)
	}

	ctx.Messages = append(ctx.Messages, message)
	if len(ctx.Messages) > 5 {
		ctx.Messages = ctx.Messages[len(ctx.Messages)-5:]
	}
	newData, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	return c.rdb.Set(c.ctx, key, newData, time.Hour).Err()
}
func (c *Client) GetContext(userID int64) (*UserContext, error) {
	key := fmt.Sprintf("user:%d:context", userID)

	data, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("context not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	var ctx UserContext
	if err := json.Unmarshal([]byte(data), &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	return &ctx, nil
}
func (c *Client) DeleteContext(userID int64) error {
	key := fmt.Sprintf("user:%d:context", userID)
	return c.rdb.Del(c.ctx, key).Err()
}
func (c *Client) TryAcquireCooldown(userID int64, ttl time.Duration) (ok bool, remaining time.Duration, err error) {
	key := fmt.Sprintf("user:%d:cooldown", userID)

	acquired, err := c.rdb.SetNX(c.ctx, key, "1", ttl).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to set cooldown: %w", err)
	}
	if acquired {
		return true, 0, nil
	}

	ttlLeft, err := c.rdb.TTL(c.ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to get cooldown ttl: %w", err)
	}
	if ttlLeft < 0 {
		ttlLeft = 0
	}
	return false, ttlLeft, nil
}
func (c *Client) TryAcquireCustomCooldown(userID int64, name string, ttl time.Duration) (ok bool, remaining time.Duration, err error) {
	key := fmt.Sprintf("user:%d:cooldown:%s", userID, name)

	acquired, err := c.rdb.SetNX(c.ctx, key, "1", ttl).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to set cooldown %s: %w", name, err)
	}
	if acquired {
		return true, 0, nil
	}

	ttlLeft, err := c.rdb.TTL(c.ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to get cooldown ttl %s: %w", name, err)
	}
	if ttlLeft < 0 {
		ttlLeft = 0
	}
	return false, ttlLeft, nil
}
func (c *Client) Close() error {
	return c.rdb.Close()
}

// PhotoDiscount stores discount settings for photo category
type PhotoDiscount struct {
	Percent int   `json:"percent"`  // Discount percentage (e.g., 50 for 50% off)
	EndTime int64 `json:"end_time"` // Unix timestamp when discount ends
}

// SetPhotoDiscount sets a temporary discount for photo category
func (c *Client) SetPhotoDiscount(percent int, endTime time.Time) error {
	if time.Until(endTime) <= 0 {
		return fmt.Errorf("end time must be in the future")
	}
	discount := PhotoDiscount{
		Percent: percent,
		EndTime: endTime.Unix(),
	}
	data, err := json.Marshal(discount)
	if err != nil {
		return fmt.Errorf("failed to marshal photo discount: %w", err)
	}
	fmt.Printf("[DISCOUNT] Set: percent=%d endTime=%d (now=%d, duration=%v)\n", percent, discount.EndTime, time.Now().Unix(), time.Until(endTime))
	// Store without TTL to avoid premature deletion on Redis restart/eviction; expiration is enforced in GetPhotoDiscount.
	return c.rdb.Set(c.ctx, "global:photo_discount", data, 0).Err()
}

// GetPhotoDiscount returns current photo discount settings if active
func (c *Client) GetPhotoDiscount() (*PhotoDiscount, error) {
	data, err := c.rdb.Get(c.ctx, "global:photo_discount").Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var discount PhotoDiscount
	if err := json.Unmarshal([]byte(data), &discount); err != nil {
		return nil, err
	}
	// Check if discount is still active
	now := time.Now().Unix()
	if now >= discount.EndTime {
		// Clean up expired key to prevent stale data
		_ = c.rdb.Del(c.ctx, "global:photo_discount").Err()
		fmt.Printf("[DISCOUNT] Expired: now=%d endTime=%d diff=%d sec\n", now, discount.EndTime, now-discount.EndTime)
		return nil, nil
	}
	fmt.Printf("[DISCOUNT] Active: percent=%d now=%d endTime=%d remaining=%d sec\n", discount.Percent, now, discount.EndTime, discount.EndTime-now)
	return &discount, nil
}

// RemovePhotoDiscount removes the photo discount
func (c *Client) RemovePhotoDiscount() error {
	return c.rdb.Del(c.ctx, "global:photo_discount").Err()
}
