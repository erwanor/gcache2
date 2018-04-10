package gcache

import (
	"container/list"
	"errors"
	"time"
)

type arcItem struct {
	key     interface{}
	value   interface{}
	parent  *list.List
	element *list.Element
	ghost   bool
}

// Constantly balances between LRU and LFU, to improve the combined result.
type ARC struct {
	baseCache
	store map[interface{}]*arcItem
	// t1 ("tier-1") is an internal cache tracking "recency"
	t1 *list.List
	// t2 ("tier-2") is an internal cache tracking "frequency"
	t2 *list.List
	// Ghost lists for t1 and t2 respectively: tracks recently evicted keys.
	b1 *list.List
	b2 *list.List

	// split is a tuning parameter used by the cache to optimize hit rate for
	//      recency or frequency, depending on the workload.
	//	It's sometime referred as the "learning parameter" of the cache.
	split int
}

func newARC(cb *CacheBuilder) *ARC {
	c := &ARC{}
	buildCache(&c.baseCache, cb)
	c.store = make(map[interface{}]*arcItem, c.capacity)
	c.t1 = list.New()
	c.t2 = list.New()
	c.b1 = list.New()
	c.b2 = list.New()
	c.loadGroup.cache = c
	return c
}

func (c *ARC) set(key, value interface{}) (interface{}, error) {
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
		c.size++

		entry = &arcItem{
			key:   key,
			value: value,
		}

		c.request(entry)
		c.store[key] = entry
		return entry, nil
	}

	if entry.ghost {
		c.size++
	}

	entry.value = value
	entry.ghost = false
	c.request(entry)
	return entry, nil
}

func (c *ARC) Set(key, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.set(key, value)
	return err
}

func (c *ARC) SetWithExpire(key, value interface{}, expiration time.Duration) error {
	return c.Set(key, value)
}

func (c *ARC) get(key interface{}, onLoad bool) (interface{}, error) {
	entry, exists := c.store[key]
	if !exists {
		// Ugly. This needs to go.
		if !onLoad {
			c.stats.IncrMissCount()
		}
		return nil, KeyNotFoundError
	}

	c.request(entry)

	if c.deserializeFunc != nil {
		return c.deserializeFunc(key, entry.value)
	}

	return entry.value, nil
}

func (c *ARC) getWithLoader(key interface{}, isWait bool) (interface{}, error) {
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

func (c *ARC) Get(key interface{}) (interface{}, error) {
	c.mu.Lock()
	v, err := c.get(key, false)
	c.mu.Unlock()
	if err == KeyNotFoundError {
		return c.getWithLoader(key, true)
	}
	return v, err
}

func (c *ARC) GetIFPresent(key interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	v, err := c.get(key, false)
	if err == KeyNotFoundError {
		return c.getWithLoader(key, false)
	}

	return v, err
}

func (c *ARC) GetALL() map[interface{}]interface{} {
	m := make(map[interface{}]interface{})
	return m
}

func (c *ARC) Keys() []interface{} {
	return make([]interface{}, 0)
}

func (c *ARC) Purge() {
	return
}

// Change bool to error
func (c *ARC) Remove(key interface{}) bool {
	return false
}

func (c *ARC) Len() int {
	return c.size
}

func (c *ARC) request(e *arcItem) error {
	var delta int
	if e.parent == c.t1 || e.parent == c.t2 {
		c.stats.IncrHitCount()
		e.setMRU(c.t2)
		return nil
	}

	// Note: I find this way of doing things really ugly. I am not happy with that.
	// For now, we will stick to the specs but there is a refactor to be done here.
	if e.parent == c.b1 {
		if c.b1.Len() >= c.b2.Len() {
			delta = 1
		} else {
			delta = c.b2.Len() / c.b1.Len()
		}

		c.split = minInt(c.split+delta, c.capacity)
		c.replace(e)
		e.setMRU(c.t2)
		return nil
	}

	if e.parent == c.b2 {
		if c.b2.Len() >= c.b1.Len() {
			delta = 1
		} else {
			delta = c.b1.Len() / c.b2.Len()
		}

		c.split = maxInt(c.split-delta, 0)
		c.replace(e)
		e.setMRU(c.t2)
		return nil
	}

	if e.parent != nil {
		return errors.New("ARC/internal/request: unknown internal cache type")
	}

	l1Len := c.t1.Len() + c.b1.Len()
	l2Len := c.t2.Len() + c.b2.Len()

	if l1Len == c.capacity {
		if c.t1.Len() < c.capacity {
			c.removeLRU(c.b1)
			c.replace(e)
		} else {
			c.removeLRU(c.t1)
		}
	} else {
		if l1Len+l2Len >= c.capacity {
			if l1Len+l2Len == 2*c.capacity {
				c.removeLRU(c.b2)
			}
			c.replace(e)
		}
	}

	e.setMRU(c.t1)

	return nil
}

func (c *ARC) removeLRU(l *list.List) {
	lru := l.Back()
	if c.evictedFunc != nil {
		defer c.evictedFunc(lru.Value.(*arcItem).key, lru.Value.(*arcItem).value)
	}

	l.Remove(lru)
	delete(c.store, lru.Value.(*arcItem).key)
	c.size--
}

func (c *ARC) replace(e *arcItem) {
	var lru *arcItem
	var target *list.List
	if c.t1.Len() > 0 && (c.t1.Len() > c.split) || (e.parent == c.b2 && c.t1.Len() == c.split) {
		lru = c.t1.Back().Value.(*arcItem)
		target = c.b1
	} else {
		lru = c.t2.Back().Value.(*arcItem)
		target = c.b2
	}

	if c.evictedFunc != nil {
		defer c.evictedFunc(lru.key, lru.value)
	}

	lru.value = nil
	lru.ghost = true
	lru.setMRU(target)
	c.size--
}

func (e *arcItem) setMRU(l *list.List) {
	if e.parent != nil {
		e.parent.Remove(e.element)
	}

	e.parent = l
	e.element = e.parent.PushFront(e)
}
