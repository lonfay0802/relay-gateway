package common

import (
	"sync"
	"time"
)

// LocalCacheItem 本地缓存项
type LocalCacheItem struct {
	Value      interface{}
	ExpireTime time.Time
}

// LocalCache 本地内存缓存（带过期时间）
type LocalCache struct {
	cache sync.Map
	ttl   time.Duration
}

// NewLocalCache 创建新的本地缓存
func NewLocalCache(ttl time.Duration) *LocalCache {
	lc := &LocalCache{
		ttl: ttl,
	}
	// 启动后台清理协程
	go lc.cleanupExpired()
	return lc
}

// Set 设置缓存
func (lc *LocalCache) Set(key string, value interface{}) {
	item := LocalCacheItem{
		Value:      value,
		ExpireTime: time.Now().Add(lc.ttl),
	}
	lc.cache.Store(key, item)
}

// Get 获取缓存，返回 value 和是否找到
func (lc *LocalCache) Get(key string) (interface{}, bool) {
	val, ok := lc.cache.Load(key)
	if !ok {
		return nil, false
	}

	item := val.(LocalCacheItem)
	// 检查是否过期
	if time.Now().After(item.ExpireTime) {
		lc.cache.Delete(key)
		return nil, false
	}

	return item.Value, true
}

// Delete 删除缓存
func (lc *LocalCache) Delete(key string) {
	lc.cache.Delete(key)
}

// Clear 清空所有缓存
func (lc *LocalCache) Clear() {
	lc.cache.Range(func(key, value interface{}) bool {
		lc.cache.Delete(key)
		return true
	})
}

// cleanupExpired 定期清理过期缓存
func (lc *LocalCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute) // 每分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		lc.cache.Range(func(key, value interface{}) bool {
			item := value.(LocalCacheItem)
			if now.After(item.ExpireTime) {
				lc.cache.Delete(key)
			}
			return true
		})
	}
}
