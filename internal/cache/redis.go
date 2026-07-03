package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisCache implements cache using Redis
type RedisCache struct {
	client *redis.Client
	logger *zap.Logger
	prefix string
	ttl    time.Duration
}

// Config represents Redis cache configuration
type Config struct {
	Addr     string
	Password string
	DB       int
	Prefix   string
	TTL      time.Duration
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg Config, logger *zap.Logger) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "vinahost-waf:"
	}

	logger.Info("Redis cache connected",
		zap.String("addr", cfg.Addr),
		zap.Duration("ttl", ttl),
		zap.String("prefix", prefix),
	)

	return &RedisCache{
		client: client,
		logger: logger,
		prefix: prefix,
		ttl:    ttl,
	}, nil
}

// Set stores a value in cache
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	fullKey := c.prefix + key
	if err := c.client.Set(ctx, fullKey, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	c.logger.Debug("Cache set",
		zap.String("key", fullKey),
		zap.Duration("ttl", c.ttl),
	)

	return nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.prefix + key
	data, err := c.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss: key not found")
		}
		return fmt.Errorf("failed to get cache: %w", err)
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	c.logger.Debug("Cache hit",
		zap.String("key", fullKey),
	)

	return nil
}

// Delete removes a value from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := c.prefix + key
	if err := c.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	c.logger.Debug("Cache deleted",
		zap.String("key", fullKey),
	)

	return nil
}

// Exists checks if a key exists in cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := c.prefix + key
	exists, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check cache: %w", err)
	}

	return exists > 0, nil
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}
