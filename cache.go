package gcache

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	TYPE_SIMPLE = "simple"
	TYPE_LRU    = "lru"
	TYPE_LFU    = "lfu"
	TYPE_ARC    = "arc"
)

type Cache interface {
	Set(interface{}, interface{}) error
	SetWithExpire(interface{}, interface{}, time.Duration) error
	Get(interface{}) (interface{}, error)
	GetIFPresent(interface{}) (interface{}, error)
	GetALL() map[interface{}]interface{}
	Remove(interface{}) error
	Purge()
	Keys() []interface{}
	Len() int
	Refresh() error

	Debug() map[string][]int
	unsafeGet(interface{}, bool) (interface{}, error)

	statsAccessor
}

type (
	LoaderFunc       func(interface{}) (interface{}, error)
	LoaderExpireFunc func(interface{}) (interface{}, *time.Duration, error)
	EvictedFunc      func(interface{}, interface{})
	PurgeVisitorFunc func(interface{}, interface{})
	AddedFunc        func(interface{}, interface{})
	DeserializeFunc  func(interface{}, interface{}) (interface{}, error)
	SerializeFunc    func(interface{}, interface{}) (interface{}, error)
)

type baseCache struct {
	capacity int
	size     int

	loaderExpireFunc LoaderExpireFunc
	evictedFunc      EvictedFunc
	purgeVisitorFunc PurgeVisitorFunc
	addedFunc        AddedFunc
	deserializeFunc  DeserializeFunc
	serializeFunc    SerializeFunc

	expiration *time.Duration
	clock      Clock

	*stats
	mu        sync.RWMutex
	loadGroup Group
}

type CacheBuilder struct {
	tp       string
	capacity int

	loaderExpireFunc LoaderExpireFunc
	evictedFunc      EvictedFunc
	purgeVisitorFunc PurgeVisitorFunc
	addedFunc        AddedFunc
	deserializeFunc  DeserializeFunc
	serializeFunc    SerializeFunc

	expiration *time.Duration
	clock      Clock
}

var KeyNotFoundError = errors.New("Key not found.")

func New(capacity int) *CacheBuilder {
	return &CacheBuilder{
		tp:       TYPE_SIMPLE,
		capacity: capacity,
		clock:    NewRealClock(),
	}
}

// Set a loader function.
// loaderFunc: create a new value with this function if cached value is expired.
func (cb *CacheBuilder) LoaderFunc(loaderFunc LoaderFunc) *CacheBuilder {
	cb.loaderExpireFunc = func(k interface{}) (interface{}, *time.Duration, error) {
		v, err := loaderFunc(k)
		return v, nil, err
	}
	return cb
}

func (cb *CacheBuilder) EvictType(tp string) *CacheBuilder {
	cb.tp = tp
	return cb
}

func (cb *CacheBuilder) Simple() *CacheBuilder {
	return cb.EvictType(TYPE_SIMPLE)
}

func (cb *CacheBuilder) LRU() *CacheBuilder {
	return cb.EvictType(TYPE_LRU)
}

func (cb *CacheBuilder) LFU() *CacheBuilder {
	return cb.EvictType(TYPE_LFU)
}

func (cb *CacheBuilder) ARC() *CacheBuilder {
	return cb.EvictType(TYPE_ARC)
}

// Set a loader function with expiration.
// loaderExpireFunc: create a new value with this function if cached value is expired.
// If nil returned instead of time.Duration from loaderExpireFunc than value will never expire.
func (cb *CacheBuilder) LoaderExpireFunc(loaderExpireFunc LoaderExpireFunc) *CacheBuilder {
	cb.loaderExpireFunc = loaderExpireFunc
	return cb
}

func (cb *CacheBuilder) EvictedFunc(evictedFunc EvictedFunc) *CacheBuilder {
	cb.evictedFunc = evictedFunc
	return cb
}

func (cb *CacheBuilder) PurgeVisitorFunc(purgeVisitorFunc PurgeVisitorFunc) *CacheBuilder {
	cb.purgeVisitorFunc = purgeVisitorFunc
	return cb
}

func (cb *CacheBuilder) AddedFunc(addedFunc AddedFunc) *CacheBuilder {
	cb.addedFunc = addedFunc
	return cb
}

func (cb *CacheBuilder) DeserializeFunc(deserializeFunc DeserializeFunc) *CacheBuilder {
	cb.deserializeFunc = deserializeFunc
	return cb
}

func (cb *CacheBuilder) SerializeFunc(serializeFunc SerializeFunc) *CacheBuilder {
	cb.serializeFunc = serializeFunc
	return cb
}

func (cb *CacheBuilder) Clock(clock Clock) *CacheBuilder {
	cb.clock = clock
	return cb
}

func (cb *CacheBuilder) Expiration(expiration time.Duration) *CacheBuilder {
	cb.expiration = &expiration
	return cb
}

func (cb *CacheBuilder) Build() (Cache, error) {
	if cb.capacity <= 0 && cb.tp != TYPE_SIMPLE {
		return nil, fmt.Errorf("gcache2: can't Build Cache, invalid Cache capacity (%v<=0)", cb.capacity)
	}

	return cb.build()
}

func (cb *CacheBuilder) build() (Cache, error) {
	switch cb.tp {
	case TYPE_SIMPLE:
		return newSimpleCache(cb), nil
	case TYPE_LRU:
		return newLRUCache(cb), nil
	case TYPE_LFU:
		return newLFUCache(cb), nil
	case TYPE_ARC:
		return newARC(cb), nil
	default:
		return nil, fmt.Errorf("gcache2: can't build Cache, unknow Cache type (%s", cb.tp)
	}
}

func buildCache(c *baseCache, cb *CacheBuilder) {
	c.clock = cb.clock
	c.capacity = cb.capacity
	c.loaderExpireFunc = cb.loaderExpireFunc
	c.expiration = cb.expiration
	c.addedFunc = cb.addedFunc
	c.deserializeFunc = cb.deserializeFunc
	c.serializeFunc = cb.serializeFunc
	c.evictedFunc = cb.evictedFunc
	c.purgeVisitorFunc = cb.purgeVisitorFunc
	c.stats = &stats{}
}

// load a new value using by specified key.
func (c *baseCache) load(key interface{}, cb func(interface{}, *time.Duration, error) (interface{}, error), isWait bool) (interface{}, bool, error) {
	v, called, err := c.loadGroup.Do(key, func() (v interface{}, e error) {
		defer func() {
			if r := recover(); r != nil {
				e = fmt.Errorf("Loader panics: %v", r)
			}
		}()
		return cb(c.loaderExpireFunc(key))
	}, isWait)
	if err != nil {
		return nil, called, err
	}
	return v, called, nil
}
