package utils

import (
	"sync"
	"time"
)

var (
	DiskUsageCache     = newCache(30*time.Second, 24*time.Hour)
	RealPathCache      = newCache(48*time.Hour, 72*time.Hour)
	SearchResultsCache = newCache(15*time.Second, time.Hour)
)

func newCache(expires time.Duration, cleanup time.Duration) *KeyCache {
	newCache := KeyCache{
		data:         make(map[string]cachedValue),
		expiresAfter: expires, // default
	}
	go newCache.cleanupExpiredJob(cleanup)
	return &newCache
}

type KeyCache struct {
	data         map[string]cachedValue
	mu           sync.RWMutex
	expiresAfter time.Duration
}

type cachedValue struct {
	value     interface{}
	expiresAt time.Time
}

func (c *KeyCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cachedValue{
		value:     value,
		expiresAt: time.Now().Add(c.expiresAfter),
	}
}

func (c *KeyCache) SetWithExp(key string, value interface{}, exp time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cachedValue{
		value:     value,
		expiresAt: time.Now().Add(exp),
	}
}

func (c *KeyCache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cached, ok := c.data[key]
	if !ok || time.Now().After(cached.expiresAt) {
		return nil
	}
	return cached.value
}

func (c *KeyCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, cached := range c.data {
		if now.After(cached.expiresAt) {
			delete(c.data, key)
		}
	}
}

// should automatically run for all cache types as part of init.
func (c *KeyCache) cleanupExpiredJob(frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	for range ticker.C {
		c.cleanupExpired()
	}
}
