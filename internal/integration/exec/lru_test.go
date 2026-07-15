package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache(t *testing.T) {
	t.Run("MissOnEmpty", func(t *testing.T) {
		cache := newLRUCache[int](2)

		_, ok := cache.Get("a")
		assert.False(t, ok, "Empty cache should miss")
	})

	t.Run("PutThenGet", func(t *testing.T) {
		cache := newLRUCache[int](2)
		cache.Put("a", 1)

		value, ok := cache.Get("a")
		assert.True(t, ok, "Stored key should hit")
		assert.Equal(t, 1, value, "Stored value should round-trip")
	})

	t.Run("EvictsLeastRecentlyUsed", func(t *testing.T) {
		cache := newLRUCache[int](2)
		cache.Put("a", 1)
		cache.Put("b", 2)
		cache.Get("a") // refresh a
		cache.Put("c", 3)

		_, ok := cache.Get("b")
		assert.False(t, ok, "Least recently used entry should be evicted")

		_, ok = cache.Get("a")
		assert.True(t, ok, "Recently used entry should survive")

		_, ok = cache.Get("c")
		assert.True(t, ok, "New entry should be present")
	})

	t.Run("PutExistingUpdatesValue", func(t *testing.T) {
		cache := newLRUCache[int](2)
		cache.Put("a", 1)
		cache.Put("a", 2)

		value, ok := cache.Get("a")
		assert.True(t, ok, "Updated key should hit")
		assert.Equal(t, 2, value, "Put should overwrite the value")

		cache.Put("b", 3)
		cache.Put("c", 4)

		_, ok = cache.Get("c")
		assert.True(t, ok, "Capacity should not be consumed by the double Put")
	})
}
