package exec

import "container/list"

// lruCache is a minimal string-keyed LRU. It is not safe for concurrent use;
// owners guard it with their own mutex.
type lruCache[V any] struct {
	capacity int
	entries  map[string]*list.Element
	order    *list.List
}

// lruEntry is one keyed value in the recency list.
type lruEntry[V any] struct {
	key   string
	value V
}

func newLRUCache[V any](capacity int) *lruCache[V] {
	return &lruCache[V]{
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns the cached value and marks it most recently used.
func (c *lruCache[V]) Get(key string) (V, bool) {
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
func (c *lruCache[V]) Put(key string, value V) {
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
