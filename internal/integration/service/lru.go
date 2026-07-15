package service

import "container/list"

// LRU is a minimal string-keyed LRU shared by the integration module's
// content-hash caches. It is not safe for concurrent use; owners guard it
// with their own mutex.
type LRU[V any] struct {
	capacity int
	entries  map[string]*list.Element
	order    *list.List
}

// lruEntry is one keyed value in the recency list.
type lruEntry[V any] struct {
	key   string
	value V
}

// NewLRU creates an LRU bounded to capacity entries.
func NewLRU[V any](capacity int) *LRU[V] {
	return &LRU[V]{
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns the cached value and marks it most recently used.
func (c *LRU[V]) Get(key string) (V, bool) {
	element, ok := c.entries[key]
	if !ok {
		var zero V

		return zero, false
	}

	c.order.MoveToFront(element)

	return element.Value.(*lruEntry[V]).value, true
}

// Put stores value under key, evicting the least recently used entry when
// the cache is full.
func (c *LRU[V]) Put(key string, value V) {
	if element, ok := c.entries[key]; ok {
		element.Value.(*lruEntry[V]).value = value
		c.order.MoveToFront(element)

		return
	}

	if len(c.entries) >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.entries, oldest.Value.(*lruEntry[V]).key)
		}
	}

	c.entries[key] = c.order.PushFront(&lruEntry[V]{key: key, value: value})
}
