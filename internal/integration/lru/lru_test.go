package lru

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRU(t *testing.T) {
	t.Run("MissOnEmpty", func(t *testing.T) {
		cache := New[int](2)

		_, ok := cache.Get("a")
		assert.False(t, ok, "Empty cache should miss")
	})

	t.Run("PutThenGet", func(t *testing.T) {
		cache := New[int](2)
		cache.Put("a", 1)

		value, ok := cache.Get("a")
		assert.True(t, ok, "Stored key should hit")
		assert.Equal(t, 1, value, "Stored value should round-trip")
	})

	t.Run("EvictsLeastRecentlyUsed", func(t *testing.T) {
		cache := New[int](2)
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
		cache := New[int](2)
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

func TestSynced(t *testing.T) {
	t.Run("BuildsOnceAndCaches", func(t *testing.T) {
		cache := NewSynced[int](2)
		builds := 0

		for range 2 {
			value, err := cache.GetOrBuild("a", func() (int, error) {
				builds++

				return 7, nil
			})
			require.NoError(t, err, "Build should succeed")
			assert.Equal(t, 7, value, "Cached value should round-trip")
		}

		assert.Equal(t, 1, builds, "Second lookup should hit the cache")
	})

	t.Run("BuildErrorIsNotCached", func(t *testing.T) {
		cache := NewSynced[int](2)

		_, err := cache.GetOrBuild("a", func() (int, error) {
			return 0, assert.AnError
		})
		require.ErrorIs(t, err, assert.AnError, "Build error should surface")

		value, err := cache.GetOrBuild("a", func() (int, error) {
			return 9, nil
		})
		require.NoError(t, err, "Retry should rebuild after a failed build")
		assert.Equal(t, 9, value, "A failed build must not be cached")
	})
}
