package cache2

import (
	"log"
	"sort"
	"sync"
	"time"
)

type CacheTable struct {
	sync.RWMutex
	name string
	//存的key信息
	item map[interface{}]*CacheItem

	//定时处理器
	cleanupTimer *time.Timer
	//计时器
	cleanupInterval time.Duration

	logger *log.Logger

	//尝试加载不存在的key的回调函数
	loadData func(key interface{}, args ...interface{}) *CacheItem
	//新增key的回调函数
	addedItem []func(item *CacheItem)
	//从缓存中删除项目之前的回调方法
	aboutDeleteItem []func(item *CacheItem)
}

// 表中总共的缓存数
func (table *CacheTable) Count() int {
	table.RLock()
	defer table.RUnlock()
	return len(table.item)
}

// 循环所有缓存
func (table *CacheTable) Foreach(trans func(key interface{}, item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()

	for k, v := range table.item {
		trans(k, v)
	}
}

// 加载不存在的key时的回调
func (table *CacheTable) SetDataLoader(f func(key interface{}, args ...interface{}) *CacheItem) {
	table.RLock()
	defer table.RUnlock()
	table.loadData = f
}

// 设置新增时的回调
func (table *CacheTable) SetAddedItemCallback(f func(item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()
	if len(table.addedItem) > 0 {
		table.RemoveAddedItemCallbacks()
	}
	table.addedItem = append(table.addedItem, f)
}

func (table *CacheTable) AddAddedItemCallback(f func(item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()
	table.addedItem = append(table.addedItem, f)
}

func (table *CacheTable) RemoveAddedItemCallbacks() {
	table.RLock()
	defer table.RUnlock()
	table.addedItem = nil
}

// 设置删除之前的回调方法
func (table *CacheTable) SetAboutDeletItemCallback(f func(item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()
	if len(table.aboutDeleteItem) > 0 {
		table.RemoveAboutDeletItemCallback()
	}
	table.aboutDeleteItem = append(table.aboutDeleteItem, f)
}

func (table *CacheTable) AddAboutDeletItemCallback(f func(item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()
	table.aboutDeleteItem = append(table.aboutDeleteItem, f)
}

func (table *CacheTable) RemoveAboutDeletItemCallback() {
	table.RLock()
	defer table.RUnlock()
	table.aboutDeleteItem = nil
}

// 生成日志
func (table *CacheTable) SetLogger(logger *log.Logger) {
	table.RLock()
	defer table.RUnlock()
	table.logger = logger
}

// ***到期循环检查定时器
func (table *CacheTable) expirationCheck() {
	table.Lock()
	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
	if table.cleanupInterval > 0 {
		table.log("在", table.cleanupInterval, "之后将为表", table.name, "执行清理")
	} else {
		table.log("已经为表", table.name, "执过行清理")
	}

	now := time.Now()
	smallestDuration := 0 * time.Second
	for k, item := range table.item {
		item.RLock()
		lifespan := item.lifeSpan
		accesson := item.accessOn
		item.RUnlock()
		if lifespan == 0 {
			continue
		}
		// 获取当前时间和最后一次访问的时间差
		if now.Sub(accesson) >= lifespan {
			table.deleteInternal(k)
		} else {
			// 获取下次过期时间数
			if smallestDuration == 0 || lifespan-now.Sub(accesson) < smallestDuration {
				smallestDuration = lifespan - now.Sub(accesson)
			}
		}
	}
	table.cleanupInterval = smallestDuration
	if smallestDuration > 0 {
		table.cleanupTimer = time.AfterFunc(smallestDuration, func() {
			go table.expirationCheck()
		})
	}
	table.Unlock()

}

// 添加计时器
func (table *CacheTable) addInternal(item *CacheItem) {
	table.log("在表", table.name, "中添加项目，项目key：", item.key, "过期时间为：", item.lifeSpan)
	table.item[item.key] = item

	// 缓存值避免一直被阻塞
	expDur := table.cleanupInterval
	addedItem := table.addedItem
	table.Unlock()

	if addedItem != nil {
		for _, callback := range addedItem {
			callback(item)
		}
	}

	if item.lifeSpan > 0 && (expDur == 0 || item.lifeSpan < expDur) {
		table.expirationCheck()
	}
}

func (table *CacheTable) deleteInternal(key interface{}) (error, *CacheItem) {
	r, ok := table.item[key]
	if !ok {
		return ErrKeyNotFound, nil
	}
	aboutToDeleteItem := table.aboutDeleteItem
	table.Unlock()
	if aboutToDeleteItem != nil {
		for _, callback := range aboutToDeleteItem {
			callback(r)
		}
	}

	r.RLock()
	defer r.RUnlock()
	if r.aboutToExpire != nil {
		for _, callback := range aboutToDeleteItem {
			callback(r)
		}
	}
	table.Lock()
	table.log("删除项目", key, "创建时间：", r.createOn, "最后连接时间：", r.accessOn, "来自表", table.name)
	delete(table.item, key)
	return nil, r
}

func (table *CacheTable) Add(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
	item := NewCacheItem(key, data, lifeSpan)
	table.Lock()
	table.addInternal(item)
	return item
}

func (table *CacheTable) Delete(key interface{}) (error, *CacheItem) {
	table.Lock()
	table.Unlock()
	return table.deleteInternal(key)
}

// 是否存在
func (table *CacheTable) Exists(key interface{}) bool {
	table.RLock()
	defer table.RUnlock()
	_, ok := table.item[key]
	return ok
}

func (table *CacheTable) NotFoundAdd(key, data interface{}, lifespan time.Duration) bool {
	table.Lock()
	if _, ok := table.item[key]; ok {
		table.Unlock()
		return false
	}
	item := NewCacheItem(key, data, lifespan)
	table.addInternal(item)
	return true
}

// 获取值
func (table *CacheTable) Value(key interface{}, args ...interface{}) (*CacheItem, error) {
	table.RLock()
	r, ok := table.item[key]
	loadData := table.loadData
	defer table.RUnlock()

	// 存在key返回
	if ok {
		r.KeepAlive()
		return r, nil
	}

	// 不存在key但是有回调方法，执行回调方法并缓存key
	if loadData != nil {
		item := loadData(key, args...)
		if item != nil {
			table.Add(item.key, item.lifeSpan, item.data)
			return item, nil
		}
		return nil, ErrKeyNotFoundOrLoadable
	}

	return nil, ErrKeyNotFound

}

// 更新所有表
func (table *CacheTable) Flush() {
	table.Lock()
	defer table.Unlock()

	// 删除表中所有的数据
	table.log("删除表中所有的数据：", table.name)
	table.item = make(map[interface{}]*CacheItem)
	table.cleanupInterval = 0
	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
}

// 把key映射到计数器
type CacheItemPair struct {
	key         interface{}
	AccessCount int64
}

// 切片用来根据访问次数排序
type CacheItemPairList []CacheItemPair

func (p CacheItemPairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p CacheItemPairList) Len() int           { return len(p) }
func (p CacheItemPairList) Less(i, j int) bool { return p[i].AccessCount > p[j].AccessCount }

// 返回项目中访问最多的前count项目
func (table *CacheTable) MostAccessed(count int64) []*CacheItem {
	table.RLock()
	defer table.RUnlock()

	p := make(CacheItemPairList, len(table.item))
	i := 0
	for k, v := range table.item {
		p[i] = CacheItemPair{k, v.accessCount}
		i++
	}

	sort.Sort(p)

	var r []*CacheItem
	c := int64(0)
	for _, v := range p {
		if c >= count {
			break
		}

		item, ok := table.item[v.key]
		if ok {
			r = append(r, item)
		}
		c++
	}
	return r
}

// Internal logging method for convenience.
func (table *CacheTable) log(v ...interface{}) {
	if table.logger == nil {
		return
	}

	table.logger.Println(v...)
}
