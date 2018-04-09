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
		c.mu.Lock()
		defer c.mu.Unlock()

		_, err := c.set(key, v)
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
	defer c.mu.Unlock()

	v, err := c.get(key, false)
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


		}
	}

	if e.parent == c.b2 {
		if c.b2.Len() >= c.b1.Len() {
			delta = 1
		} else {
			delta = c.b1.Len() / c.b2.Len()
		}

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



}

func (e *arcItem) setMRU(l *list.List) {
	if e.parent != nil {
		e.parent.Remove(e.element)
	}

	e.parent = l
	e.element = e.parent.PushFront(e)
}
