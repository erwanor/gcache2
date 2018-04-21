package gcache

import (
	"container/list"
	"time"
)

// Discards the least frequently used store first.
type LFUCache struct {
	baseCache
	store    map[interface{}]*lfuItem
	freqList *list.List // list for freqEntry
}

type freqEntry struct {
	freq  uint
	items map[*lfuItem]struct{}
}

type lfuItem struct {
	clock       Clock
	key         interface{}
	value       interface{}
	freqElement *list.Element
	expiration  *time.Time
}

func newLFUCache(cb *CacheBuilder) *LFUCache {
	c := &LFUCache{}
	buildCache(&c.baseCache, cb)

	c.init()
	c.loadGroup.cache = c
	return c
}

func (c *LFUCache) init() {
	c.freqList = list.New()
	c.store = make(map[interface{}]*lfuItem, c.capacity+1)
	c.freqList.PushFront(&freqEntry{
		freq:  0,
		items: make(map[*lfuItem]struct{}),
	})
}

func (c *LFUCache) set(key, value interface{}) (interface{}, error) {
	var err error
	if c.serializeFunc != nil {
		value, err = c.serializeFunc(key, value)
		if err != nil {
			return nil, err
		}
	}

	if c.addedFunc != nil {
		defer c.addedFunc(key, value)
	}

	entry, exists := c.store[key]
	if !exists {
		if len(c.store) >= c.capacity {
			c.evict(1)
		}

		c.size++

		entry = &lfuItem{
			key:         key,
			value:       value,
			freqElement: nil,
			clock:       c.clock,
		}

		lfuEntry := c.freqList.Front()
		fe := lfuEntry.Value.(*freqEntry)
		fe.items[entry] = struct{}{}
		entry.freqElement = lfuEntry
		c.store[key] = entry
	}

	entry.value = value

	if c.expiration != nil {
		t := c.clock.Now().Add(*c.expiration)
		entry.expiration = &t
	}

	return entry, nil
}

// Set a new key-value pair
func (c *LFUCache) Set(key, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.set(key, value)
	return err
}

// Set a new key-value pair with an expiration time
func (c *LFUCache) SetWithExpire(key, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, err := c.set(key, value)
	if err != nil {
		return err
	}

	t := c.clock.Now().Add(expiration)
	item.(*lfuItem).expiration = &t
	return nil
}

func (c *LFUCache) get(key interface{}, onLoad bool) (interface{}, error) {
	item, exists := c.store[key]

	if !exists {
		if !onLoad {
			c.stats.IncrMissCount()
		}
		return nil, KeyNotFoundError
	}

	if item.isExpired(nil) {
		c.removeItem(item)
		return nil, KeyNotFoundError
	}

	c.increment(item)

	v := item.value
	if !onLoad {
		c.stats.IncrHitCount()
	}

	if c.deserializeFunc != nil {
		return c.deserializeFunc(key, v)
	}

	return v, nil
}

func (c *LFUCache) getWithLoader(key interface{}, isWait bool) (interface{}, error) {
	if c.loaderExpireFunc == nil {
		return nil, KeyNotFoundError
	}
	value, _, err := c.load(key, func(v interface{}, expiration *time.Duration, e error) (interface{}, error) {
		if e != nil {
			return nil, e
		}

		err := c.Set(key, v)
		if err != nil {
			return nil, err
		}

		return v, nil
	}, isWait)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Get a value from cache pool using key if it exists.
// If it dose not exists key and has LoaderFunc,
// generate a value using `LoaderFunc` method returns value.
func (c *LFUCache) Get(key interface{}) (interface{}, error) {
	c.mu.Lock()
	v, err := c.get(key, false)
	c.mu.Unlock()

	if err == KeyNotFoundError {
		return c.getWithLoader(key, true)
	}
	return v, err
}

// Get a value from cache pool using key if it exists.
// If it dose not exists key, returns KeyNotFoundError.
// And send a request which refresh value for specified key if cache object has LoaderFunc.
func (c *LFUCache) GetIFPresent(key interface{}) (interface{}, error) {
	c.mu.Lock()
	v, err := c.get(key, false)
	c.mu.Unlock()

	if err == KeyNotFoundError {
		return c.getWithLoader(key, false)
	}
	return v, err
}

// Returns all key-value pairs in the cache.
func (c *LFUCache) GetALL() map[interface{}]interface{} {
	c.mu.Lock()
	allKeys := c.keys()
	c.mu.Unlock()

	m := make(map[interface{}]interface{})
	for _, k := range allKeys {
		v, err := c.GetIFPresent(k)
		if err == nil {
			m[k] = v
		}
	}
	return m
}

func (c *LFUCache) remove(key interface{}) error {
	if item, ok := c.store[key]; ok {
		c.removeItem(item)
		return nil
	}
	return KeyNotFoundError
}

// Removes the provided key from the cache.
func (c *LFUCache) Remove(key interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.remove(key)
}

// Completely clear the cache
func (c *LFUCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.purgeVisitorFunc != nil {
		for key, item := range c.store {
			c.purgeVisitorFunc(key, item.value)
		}
	}

	c.init()
}

func (c *LFUCache) keys() []interface{} {
	keys := make([]interface{}, len(c.store))
	var i = 0
	for k := range c.store {
		keys[i] = k
		i++
	}
	return keys
}

// Returns a slice of the keys in the cache.
func (c *LFUCache) Keys() []interface{} {
	c.mu.Lock()
	allKeys := c.keys()
	c.mu.Unlock()

	keys := []interface{}{}
	for _, k := range allKeys {
		_, err := c.GetIFPresent(k)
		if err == nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// Returns the number of store in the cache.
func (c *LFUCache) Len() int {
	return len(c.store)
}

func (c *LFUCache) increment(item *lfuItem) {
	currentFreqElement := item.freqElement
	currentFreqEntry := currentFreqElement.Value.(*freqEntry)
	nextFreq := currentFreqEntry.freq + 1
	delete(currentFreqEntry.items, item)

	nextFreqElement := currentFreqElement.Next()
	if nextFreqElement == nil {
		nextFreqElement = c.freqList.InsertAfter(&freqEntry{
			freq:  nextFreq,
			items: make(map[*lfuItem]struct{}),
		}, currentFreqElement)
	}
	nextFreqElement.Value.(*freqEntry).items[item] = struct{}{}
	item.freqElement = nextFreqElement
}

// evict removes the item with smallest frequency from the cache.
func (c *LFUCache) evict(count int) {
	entry := c.freqList.Front()
	for i := 0; i < count; {
		if entry == nil {
			return
		} else {
			for item := range entry.Value.(*freqEntry).items {
				if i >= count {
					return
				}
				c.removeItem(item)
				i++
			}
			entry = entry.Next()
		}
	}
}

// removeElement is used to remove a given list element from the cache
func (c *LFUCache) removeItem(item *lfuItem) {
	delete(c.store, item.key)
	delete(item.freqElement.Value.(*freqEntry).items, item)
	if c.evictedFunc != nil {
		c.evictedFunc(item.key, item.value)
	}
}

// returns boolean value whether this item is expired or not.
func (it *lfuItem) isExpired(now *time.Time) bool {
	if it.expiration == nil {
		return false
	}
	if now == nil {
		t := it.clock.Now()
		now = &t
	}
	return it.expiration.Before(*now)
}

func (c *LFUCache) Debug() map[string][]int {
	d := make(map[string][]int)
	d["lfu"] = []int{len(c.store), c.freqList.Len()}
	return d
}

func (c *LFUCache) unsafeGet(key interface{}, onLoad bool) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(key, onLoad)
}
