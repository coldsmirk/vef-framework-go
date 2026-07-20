package lru

import (
	"container/list"
	"sync"
)

// Cache is a minimal string-keyed LRU cache shared by the integration
// module's content-hash caches. It is not safe for concurrent use; owners
// guard it with their own mutex.
type Cache[V any] struct {
	capacity int
	entries  map[string]*list.Element
	order    *list.List
}

// entry is one keyed value in the recency list.
type entry[V any] struct {
	key   string
	value V
}

// New creates a cache bounded to capacity entries.
func New[V any](capacity int) *Cache[V] {
	return &Cache[V]{
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns the cached value and marks it most recently used.
func (c *Cache[V]) Get(key string) (V, bool) {
	element, ok := c.entries[key]
	if !ok {
		var zero V

		return zero, false
	}

	c.order.MoveToFront(element)

	return element.Value.(*entry[V]).value, true
}

// Put stores value under key, evicting the least recently used entry when
// the cache is full.
func (c *Cache[V]) Put(key string, value V) {
	if element, ok := c.entries[key]; ok {
		element.Value.(*entry[V]).value = value
		c.order.MoveToFront(element)

		return
	}

	if len(c.entries) >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.entries, oldest.Value.(*entry[V]).key)
		}
	}

	c.entries[key] = c.order.PushFront(&entry[V]{key: key, value: value})
}

// Synced is a mutex-guarded Cache with a get-or-build fast path — the access
// pattern every integration content-hash cache (compiled programs, resolved
// schemas, code map indexes, per-system clients) shares.
type Synced[V any] struct {
	mu    sync.Mutex
	cache *Cache[V]
}

// NewSynced creates a synchronized cache bounded to capacity entries.
func NewSynced[V any](capacity int) *Synced[V] {
	return &Synced[V]{cache: New[V](capacity)}
}

// GetOrBuild returns the value cached under key, building and caching it on
// first sight. Build runs under the cache lock, so concurrent misses never
// build the same entry twice.
func (s *Synced[V]) GetOrBuild(key string, build func() (V, error)) (V, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if value, ok := s.cache.Get(key); ok {
		return value, nil
	}

	value, err := build()
	if err != nil {
		var zero V

		return zero, err
	}

	s.cache.Put(key, value)

	return value, nil
}
