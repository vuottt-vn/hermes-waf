package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/vinahost/waf/internal/cache"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	cache    cache.Cache
	rate     int           // requests per interval
	interval time.Duration // time interval
}

type bucket struct {
	tokens     int
	lastUpdate time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cache cache.Cache, rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		cache:    cache,
		rate:     rate,
		interval: interval,
	}
}

// Allow checks if a request is allowed for the given key
func (rl *RateLimiter) Allow(ctx context.Context, key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     rl.rate,
			lastUpdate: time.Now(),
		}
		rl.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastUpdate)
	tokensToAdd := int(elapsed / rl.interval) * rl.rate
	
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, rl.rate)
		b.lastUpdate = now
	}

	// Check if request is allowed
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// GetRemaining returns remaining tokens for a key
func (rl *RateLimiter) GetRemaining(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	b, exists := rl.buckets[key]
	if !exists {
		return rl.rate
	}

	// Calculate current tokens
	now := time.Now()
	elapsed := now.Sub(b.lastUpdate)
	tokensToAdd := int(elapsed / rl.interval) * rl.rate
	
	currentTokens := min(b.tokens+tokensToAdd, rl.rate)
	return currentTokens
}

// Reset resets the rate limiter for a key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, key)
}

// Cleanup removes old buckets (call periodically)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, b := range rl.buckets {
		// Remove buckets older than 2x interval
		if now.Sub(b.lastUpdate) > 2*rl.interval {
			delete(rl.buckets, key)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
