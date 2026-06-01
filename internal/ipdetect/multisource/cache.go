package multisource

import (
	"sync"
	"time"
)

// cache 内存 IP 缓存，支持 TTL
type cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
	ttl   time.Duration
}

type cacheItem struct {
	value   string
	expires time.Time
}

// newCache 创建缓存实例
func newCache(ttl time.Duration) *cache {
	c := &cache{
		items: make(map[string]cacheItem),
		ttl:   ttl,
	}

	// 后台清理过期项
	go c.cleanupLoop()

	return c
}

// Get 获取缓存的 IP
func (c *cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return "", false
	}
	if time.Now().After(item.expires) {
		return "", false
	}
	return item.value, true
}

// Set 设置缓存 IP
func (c *cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:   value,
		expires: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存
func (c *cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear 清空缓存
func (c *cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}

// cleanupLoop 定期清理过期项
func (c *cache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expires) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
