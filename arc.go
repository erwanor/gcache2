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
}

func newARC(cb *CacheBuilder) *ARC {
	c := &ARC{}
	buildCache(&c.baseCache, cb)
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

	}


		}

	}

	}

}

}

}

func (c *ARC) get(key interface{}, onLoad bool) (interface{}, error) {
		}
	}

	}
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

	c.mu.Lock()
	defer c.mu.Unlock()

}


	}

}

}

func (c *ARC) Keys() []interface{} {
}

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

	}

	l.Remove(lru)
	delete(c.store, lru.Value.(*arcItem).key)
	c.size--
}



}

}
