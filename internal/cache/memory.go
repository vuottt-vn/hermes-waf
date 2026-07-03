package cache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MemoryCache implements in-memory cache (fallback when Redis unavailable)
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
	logger *zap.Logger
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(ttl time.Duration, logger *zap.Logger) *MemoryCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	cache := &MemoryCache{
		items:  make(map[string]*cacheItem),
		ttl:    ttl,
		logger: logger,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	logger.Info("Memory cache initialized",
		zap.Duration("ttl", ttl),
	)

	return cache
}

// Set stores a value in cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}

	c.logger.Debug("Memory cache set",
		zap.String("key", key),
		zap.Duration("ttl", c.ttl),
	)

	return nil
}

// Get retrieves a value from cache
func (c *MemoryCache) Get(ctx context.Context, key string, dest interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil
	}

	if time.Now().After(item.expiresAt) {
		return nil
	}

	// Copy value to dest
	if destPtr, ok := dest.(*interface{}); ok {
		*destPtr = item.value
	}

	c.logger.Debug("Memory cache hit",
		zap.String("key", key),
	)

	return nil
}

// Delete removes a value from cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)

	c.logger.Debug("Memory cache deleted",
		zap.String("key", key),
	)

	return nil
}

// Exists checks if a key exists in cache
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false, nil
	}

	return time.Now().Before(item.expiresAt), nil
}

// Close closes the cache
func (c *MemoryCache) Close() error {
	return nil
}

// cleanup removes expired items
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
