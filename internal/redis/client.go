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
