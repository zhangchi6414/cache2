package cache2

import (
	"sync"
	"time"
)

type CacheItem struct {
	sync.RWMutex
	key  interface{}
	data interface{}
	// 存活时间
	lifeSpan time.Duration
	createOn time.Time
	//最后一次访问时间
	accessOn time.Time
	//访问次数
	accessCount int64
	//从缓存中删除项目的回调方法
	aboutToExpire []func(interface{})
}

// 创建新的item
func NewCacheItem(key, data interface{}, lifeSpan time.Duration) *CacheItem {
	t := time.Now()
	return &CacheItem{
		RWMutex:       sync.RWMutex{},
		key:           key,
		data:          data,
		lifeSpan:      lifeSpan,
		createOn:      t,
		accessOn:      t,
		accessCount:   0,
		aboutToExpire: nil,
	}
}

// 更新最后一次访问
func (item *CacheItem) KeepAlive() {
	item.RLock()
	defer item.RUnlock()
	item.accessOn = time.Now()
	item.accessCount++
}

// 从缓存中删除项目的回调函数
func (item *CacheItem) SetAboutToExpireCallback(f func(interface{})) {
	if len(item.aboutToExpire) > 0 {
		item.RemoveAboutToExpireCallback()
	}
	item.RLock()
	defer item.RUnlock()
	item.aboutToExpire = append(item.aboutToExpire, f)
}

// 新增回调信息
func (item *CacheItem) AddAboutToExpireCallback(f func(interface{})) {
	item.RLock()
	defer item.RUnlock()
	item.aboutToExpire = append(item.aboutToExpire, f)
}

// 清空回调信息
func (item *CacheItem) RemoveAboutToExpireCallback() {
	item.RLock()
	defer item.RUnlock()
	item.aboutToExpire = nil
}

// 返回项目的过期时间
func (item *CacheItem) LifeSpan() time.Duration {
	return item.lifeSpan
}
func (item *CacheItem) AccessOn() time.Time {
	item.RLock()
	defer item.RUnlock()
	return item.accessOn
}

func (item *CacheItem) CreateOn() time.Time {
	return item.createOn
}

func (item *CacheItem) AccessCount() int64 {
	item.RLock()
	defer item.RUnlock()
	return item.accessCount
}

func (item *CacheItem) Key() interface{} {
	return item.key
}

func (item *CacheItem) Data() interface{} {
	return item.data
}
