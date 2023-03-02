package cache2

import (
	"sync"
)

var (
	cache = make(map[interface{}]*CacheTable)
	mx    sync.RWMutex
)

func Cache(table string) *CacheTable {
	mx.RLock()
	t, ok := cache[table]
	mx.RUnlock()
	if !ok {
		mx.Lock()
		t, ok = cache[table]
		if !ok {
			t = &CacheTable{
				RWMutex:         sync.RWMutex{},
				name:            table,
				item:            make(map[interface{}]*CacheItem),
				cleanupTimer:    nil,
				cleanupInterval: 0, //这里都时默认 可以通过配置文件等方法来修改创建表的配置
				logger:          nil,
				loadData:        nil,
				addedItem:       nil,
				aboutDeleteItem: nil,
			}
			cache[table] = t
		}
		mx.Unlock()
	}
	return t
}
