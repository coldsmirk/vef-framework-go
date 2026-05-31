package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache[T any](maxSize int64, defaultTTL time.Duration, evictionPolicy EvictionPolicy, gcInterval time.Duration) Cache[T] {
	return NewMemory[T](
		WithMemMaxSize(maxSize),
		WithMemDefaultTTL(defaultTTL),
		WithMemEvictionPolicy(evictionPolicy),
		WithMemGCInterval(gcInterval),
	)
}

// TestNewMemoryOptions tests new memory options functionality.
func TestNewMemoryOptions(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		cache := NewMemory[string]()
		defer cache.Close()

		mc, ok := cache.(*memoryCache[string])
		require.True(t, ok, "Type assertion to *memoryCache[string] should succeed")

		assert.Zero(t, mc.maxSize, "Should be zero value")
		assert.Equal(t, EvictionPolicyNone, mc.evictionPolicy, "TestNewMemoryOptions should match expected value")
		assert.Zero(t, mc.defaultTTL, "Should be zero value")
		assert.Greater(t, mc.gcInterval, time.Duration(0), "Should be greater")
	})

	t.Run("WithOptions", func(t *testing.T) {
		cache := NewMemory[string](
			WithMemMaxSize(128),
			WithMemDefaultTTL(2*time.Minute),
			WithMemEvictionPolicy(EvictionPolicyLFU),
			WithMemGCInterval(500*time.Millisecond),
		)
		defer cache.Close()

		mc, ok := cache.(*memoryCache[string])
		require.True(t, ok, "Type assertion to *memoryCache[string] should succeed")

		assert.Equal(t, int64(128), mc.maxSize, "TestNewMemoryOptions should match expected value")
		assert.Equal(t, EvictionPolicyLFU, mc.evictionPolicy, "TestNewMemoryOptions should match expected value")
		assert.Equal(t, 2*time.Minute, mc.defaultTTL, "TestNewMemoryOptions should match expected value")
		assert.Equal(t, 500*time.Millisecond, mc.gcInterval, "TestNewMemoryOptions should match expected value")
	})
}

// TestMemoryCacheBasicOperations tests memory cache basic operations functionality.
func TestMemoryCacheBasicOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("SetAndGet", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Set(ctx, "key1", "value1")
		require.NoError(t, err, "TestMemoryCacheBasicOperations should complete without error")

		value, found := cache.Get(ctx, "key1")
		assert.True(t, found, "Should be found")
		assert.Equal(t, "value1", value, "TestMemoryCacheBasicOperations should match expected value")
	})

	t.Run("GetNonExistentKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		value, found := cache.Get(ctx, "nonexistent")
		assert.False(t, found, "Should not be found")
		assert.Equal(t, "", value, "TestMemoryCacheBasicOperations should match expected value")
	})

	t.Run("ContainsExistingKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheBasicOperations condition should be true")
	})

	t.Run("ContainsNonExistentKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		assert.False(t, cache.Contains(ctx, "nonexistent"), "TestMemoryCacheBasicOperations condition should be false")
	})

	t.Run("DeleteExistingKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		err := cache.Delete(ctx, "key1")
		require.NoError(t, err, "TestMemoryCacheBasicOperations should complete without error")
		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheBasicOperations condition should be false")
	})

	t.Run("DeleteNonExistentKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Delete(ctx, "nonexistent")
		require.NoError(t, err, "TestMemoryCacheBasicOperations should complete without error")
	})

	t.Run("UpdateExistingKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key1", "value2")

		value, found := cache.Get(ctx, "key1")
		assert.True(t, found, "Should be found")
		assert.Equal(t, "value2", value, "TestMemoryCacheBasicOperations should match expected value")
	})

	t.Run("GetOrLoadUsesLoaderOnce", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		var loaderCalls atomic.Int32

		loader := func(context.Context) (string, error) {
			loaderCalls.Add(1)

			return "loaded", nil
		}

		value, err := cache.GetOrLoad(ctx, "key1", loader)
		require.NoError(t, err, "TestMemoryCacheBasicOperations should complete without error")
		assert.Equal(t, "loaded", value, "TestMemoryCacheBasicOperations should match expected value")
		assert.Equal(t, int32(1), loaderCalls.Load(), "TestMemoryCacheBasicOperations should match expected value")

		// Second call should hit cache without invoking loader again.
		value, err = cache.GetOrLoad(ctx, "key1", loader)
		require.NoError(t, err, "TestMemoryCacheBasicOperations should complete without error")
		assert.Equal(t, "loaded", value, "TestMemoryCacheBasicOperations should match expected value")
		assert.Equal(t, int32(1), loaderCalls.Load(), "TestMemoryCacheBasicOperations should match expected value")
	})

	t.Run("GetOrLoadRequiresLoader", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_, err := cache.GetOrLoad(ctx, "key1", nil)
		assert.ErrorIs(t, err, ErrLoaderRequired, "Error should be ErrLoaderRequired")
	})
}

// TestMemoryCacheExpiration tests memory cache expiration functionality.
func TestMemoryCacheExpiration(t *testing.T) {
	ctx := context.Background()

	t.Run("DefaultTtlExpiration", func(t *testing.T) {
		cache := newTestCache[string](0, 100*time.Millisecond, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Set(ctx, "key1", "value1")
		require.NoError(t, err, "TestMemoryCacheExpiration should complete without error")

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be true")

		time.Sleep(150 * time.Millisecond)

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be false")
		_, found := cache.Get(ctx, "key1")
		assert.False(t, found, "Should not be found")
	})

	t.Run("CustomTtlExpiration", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Set(ctx, "key1", "value1", 100*time.Millisecond)
		require.NoError(t, err, "TestMemoryCacheExpiration should complete without error")

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be true")

		time.Sleep(150 * time.Millisecond)

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be false")
	})

	t.Run("CustomTtlOverridesDefault", func(t *testing.T) {
		cache := newTestCache[string](0, 1*time.Second, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1", 100*time.Millisecond)

		time.Sleep(150 * time.Millisecond)

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be false")
	})

	t.Run("ZeroTtlMeansNoExpiration", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")

		time.Sleep(100 * time.Millisecond)

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be true")
	})

	t.Run("NegativeTtlIgnored", func(t *testing.T) {
		cache := newTestCache[string](0, 100*time.Millisecond, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1", -1*time.Second)

		time.Sleep(150 * time.Millisecond)

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheExpiration condition should be false")
	})
}

// TestMemoryCacheSize tests memory cache size functionality.
func TestMemoryCacheSize(t *testing.T) {
	ctx := context.Background()

	t.Run("SizeIncreasesOnInsert", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		size, err := cache.Size(ctx)
		require.NoError(t, err, "TestMemoryCacheSize should complete without error")
		assert.Equal(t, int64(0), size, "TestMemoryCacheSize should match expected value")

		_ = cache.Set(ctx, "key1", "value1")
		size, err = cache.Size(ctx)
		require.NoError(t, err, "TestMemoryCacheSize should complete without error")
		assert.Equal(t, int64(1), size, "TestMemoryCacheSize should match expected value")

		_ = cache.Set(ctx, "key2", "value2")
		size, err = cache.Size(ctx)
		require.NoError(t, err, "TestMemoryCacheSize should complete without error")
		assert.Equal(t, int64(2), size, "TestMemoryCacheSize should match expected value")
	})

	t.Run("SizeDecreasesOnDelete", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		cache.Delete(ctx, "key1")
		size, err := cache.Size(ctx)
		require.NoError(t, err, "TestMemoryCacheSize should complete without error")
		assert.Equal(t, int64(1), size, "TestMemoryCacheSize should match expected value")
	})

	t.Run("SizeUnchangedOnUpdate", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		size1, _ := cache.Size(ctx)

		_ = cache.Set(ctx, "key1", "value2")
		size2, _ := cache.Size(ctx)

		assert.Equal(t, size1, size2, "TestMemoryCacheSize should match expected value")
	})

	t.Run("SizeExcludesExpiredEntries", func(t *testing.T) {
		cache := newTestCache[string](0, 50*time.Millisecond, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		time.Sleep(100 * time.Millisecond)

		cache.Get(ctx, "key1")
		cache.Get(ctx, "key2")

		size, err := cache.Size(ctx)
		require.NoError(t, err, "TestMemoryCacheSize should complete without error")
		assert.Equal(t, int64(0), size, "TestMemoryCacheSize should match expected value")
	})
}

// TestMemoryCacheEvictionPolicies tests memory cache eviction policies functionality.
func TestMemoryCacheEvictionPolicies(t *testing.T) {
	ctx := context.Background()

	t.Run("LRUEviction", func(t *testing.T) {
		cache := newTestCache[string](3, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")
		_ = cache.Set(ctx, "key3", "value3")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(3), size, "TestMemoryCacheEvictionPolicies should match expected value")

		_ = cache.Set(ctx, "key4", "value4")

		size, _ = cache.Size(ctx)
		assert.Equal(t, int64(3), size, "TestMemoryCacheEvictionPolicies should match expected value")
		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheEvictionPolicies condition should be false")
		assert.True(t, cache.Contains(ctx, "key4"), "TestMemoryCacheEvictionPolicies condition should be true")
	})

	t.Run("LRUWithAccessUpdates", func(t *testing.T) {
		cache := newTestCache[string](3, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")
		_ = cache.Set(ctx, "key3", "value3")

		cache.Get(ctx, "key1")

		_ = cache.Set(ctx, "key4", "value4")

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.False(t, cache.Contains(ctx, "key2"), "TestMemoryCacheEvictionPolicies condition should be false")
		assert.True(t, cache.Contains(ctx, "key3"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.True(t, cache.Contains(ctx, "key4"), "TestMemoryCacheEvictionPolicies condition should be true")
	})

	t.Run("LFUEviction", func(t *testing.T) {
		cache := newTestCache[string](3, 0, EvictionPolicyLFU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")
		_ = cache.Set(ctx, "key3", "value3")

		cache.Get(ctx, "key1")
		cache.Get(ctx, "key1")
		cache.Get(ctx, "key2")

		_ = cache.Set(ctx, "key4", "value4")

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.True(t, cache.Contains(ctx, "key2"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.False(t, cache.Contains(ctx, "key3"), "TestMemoryCacheEvictionPolicies condition should be false")
		assert.True(t, cache.Contains(ctx, "key4"), "TestMemoryCacheEvictionPolicies condition should be true")
	})

	t.Run("FIFOEviction", func(t *testing.T) {
		cache := newTestCache[string](3, 0, EvictionPolicyFIFO, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")
		_ = cache.Set(ctx, "key3", "value3")

		cache.Get(ctx, "key1")
		cache.Get(ctx, "key1")

		_ = cache.Set(ctx, "key4", "value4")

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheEvictionPolicies condition should be false")
		assert.True(t, cache.Contains(ctx, "key2"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.True(t, cache.Contains(ctx, "key3"), "TestMemoryCacheEvictionPolicies condition should be true")
		assert.True(t, cache.Contains(ctx, "key4"), "TestMemoryCacheEvictionPolicies condition should be true")
	})

	t.Run("NoOpPolicyFallsBackToLRU", func(t *testing.T) {
		cache := newTestCache[string](2, 0, EvictionPolicyNone, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		err := cache.Set(ctx, "key3", "value3")
		assert.NoError(t, err, "TestMemoryCacheEvictionPolicies should complete without error")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(2), size, "TestMemoryCacheEvictionPolicies should match expected value")

		assert.True(t, cache.Contains(ctx, "key3"), "TestMemoryCacheEvictionPolicies condition should be true")

		count := 0
		if cache.Contains(ctx, "key1") {
			count++
		}

		if cache.Contains(ctx, "key2") {
			count++
		}

		assert.Equal(t, 1, count, "Exactly one of the old keys should remain")
	})

	t.Run("UpdateDoesNotTriggerEviction", func(t *testing.T) {
		cache := newTestCache[string](2, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		err := cache.Set(ctx, "key1", "value1_updated")
		require.NoError(t, err, "TestMemoryCacheEvictionPolicies should complete without error")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(2), size, "TestMemoryCacheEvictionPolicies should match expected value")
	})
}

// TestMemoryCacheKeys tests memory cache keys functionality.
func TestMemoryCacheKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("ListAllKeys", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "user:1", "alice")
		_ = cache.Set(ctx, "user:2", "bob")
		_ = cache.Set(ctx, "post:1", "hello")
		_ = cache.Set(ctx, "post:2", "world")

		keys, err := cache.Keys(ctx)
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Len(t, keys, 4, "Length should be 4")
	})

	t.Run("ListKeysWithPrefix", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "user:1", "alice")
		_ = cache.Set(ctx, "user:2", "bob")
		_ = cache.Set(ctx, "post:1", "hello")
		_ = cache.Set(ctx, "post:2", "world")

		userKeys, err := cache.Keys(ctx, "user:")
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Len(t, userKeys, 2, "Length should be 2")

		postKeys, err := cache.Keys(ctx, "post:")
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Len(t, postKeys, 2, "Length should be 2")
	})

	t.Run("EmptyCacheReturnsEmptyList", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		keys, err := cache.Keys(ctx)
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Empty(t, keys, "TestMemoryCacheKeys should return empty value")
	})

	t.Run("PrefixWithNoMatches", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "user:1", "alice")
		_ = cache.Set(ctx, "user:2", "bob")

		keys, err := cache.Keys(ctx, "post:")
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Empty(t, keys, "TestMemoryCacheKeys should return empty value")
	})

	t.Run("ExcludesExpiredKeys", func(t *testing.T) {
		cache := newTestCache[string](0, 50*time.Millisecond, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		time.Sleep(100 * time.Millisecond)

		keys, err := cache.Keys(ctx)
		require.NoError(t, err, "TestMemoryCacheKeys should complete without error")
		assert.Empty(t, keys, "TestMemoryCacheKeys should return empty value")
	})
}

// TestMemoryCacheForEach tests memory cache for each functionality.
func TestMemoryCacheForEach(t *testing.T) {
	ctx := context.Background()

	t.Run("IterateAllEntries", func(t *testing.T) {
		cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "a", 1)
		_ = cache.Set(ctx, "b", 2)
		_ = cache.Set(ctx, "c", 3)

		sum := 0
		err := cache.ForEach(ctx, func(_ string, value int) bool {
			sum += value

			return true
		})

		require.NoError(t, err, "TestMemoryCacheForEach should complete without error")
		assert.Equal(t, 6, sum, "TestMemoryCacheForEach should match expected value")
	})

	t.Run("IterateWithPrefixFilter", func(t *testing.T) {
		cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "user:1", 10)
		_ = cache.Set(ctx, "user:2", 20)
		_ = cache.Set(ctx, "post:1", 30)

		sum := 0
		err := cache.ForEach(ctx, func(_ string, value int) bool {
			sum += value

			return true
		}, "user:")

		require.NoError(t, err, "TestMemoryCacheForEach should complete without error")
		assert.Equal(t, 30, sum, "TestMemoryCacheForEach should match expected value")
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "a", 1)
		_ = cache.Set(ctx, "b", 2)
		_ = cache.Set(ctx, "c", 3)

		count := 0
		err := cache.ForEach(ctx, func(string, int) bool {
			count++

			return count < 2
		})

		require.NoError(t, err, "TestMemoryCacheForEach should complete without error")
		assert.Equal(t, 2, count, "TestMemoryCacheForEach should match expected value")
	})

	t.Run("EmptyCache", func(t *testing.T) {
		cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		called := false
		err := cache.ForEach(ctx, func(string, int) bool {
			called = true

			return true
		})

		require.NoError(t, err, "TestMemoryCacheForEach should complete without error")
		assert.False(t, called, "Should not be called")
	})

	t.Run("SkipsExpiredEntries", func(t *testing.T) {
		cache := newTestCache[int](0, 50*time.Millisecond, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "a", 1)
		_ = cache.Set(ctx, "b", 2)

		time.Sleep(100 * time.Millisecond)

		count := 0
		cache.ForEach(ctx, func(string, int) bool {
			count++

			return true
		})

		assert.Equal(t, 0, count, "TestMemoryCacheForEach should match expected value")
	})
}

// TestMemoryCacheClear tests memory cache clear functionality.
func TestMemoryCacheClear(t *testing.T) {
	ctx := context.Background()

	t.Run("ClearRemovesAllEntries", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")
		_ = cache.Set(ctx, "key3", "value3")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(3), size, "TestMemoryCacheClear should match expected value")

		err := cache.Clear(ctx)
		require.NoError(t, err, "TestMemoryCacheClear should complete without error")

		size, _ = cache.Size(ctx)
		assert.Equal(t, int64(0), size, "TestMemoryCacheClear should match expected value")
	})

	t.Run("ClearOnEmptyCache", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Clear(ctx)
		require.NoError(t, err, "TestMemoryCacheClear should complete without error")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(0), size, "TestMemoryCacheClear should match expected value")
	})

	t.Run("CanAddEntriesAfterClear", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		cache.Clear(ctx)

		_ = cache.Set(ctx, "key2", "value2")
		assert.True(t, cache.Contains(ctx, "key2"), "TestMemoryCacheClear condition should be true")
	})
}

// TestMemoryCacheClose tests memory cache close functionality.
func TestMemoryCacheClose(t *testing.T) {
	ctx := context.Background()

	t.Run("CloseStopsGCGoroutine", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 100*time.Millisecond)

		_ = cache.Set(ctx, "key1", "value1")

		err := cache.Close()
		require.NoError(t, err, "TestMemoryCacheClose should complete without error")

		time.Sleep(200 * time.Millisecond)
	})

	t.Run("OperationsAfterCloseFailGracefully", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)

		_ = cache.Set(ctx, "key1", "value1")
		cache.Close()

		_, found := cache.Get(ctx, "key1")
		assert.False(t, found, "Should not be found")

		err := cache.Set(ctx, "key2", "value2")
		assert.ErrorIs(t, err, ErrCacheClosed, "Error should be ErrCacheClosed")

		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheClose condition should be false")

		err = cache.Delete(ctx, "key1")
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")

		err = cache.Clear(ctx)
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")

		size, err := cache.Size(ctx)
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")
		assert.Equal(t, int64(0), size, "TestMemoryCacheClose should match expected value")

		keys, err := cache.Keys(ctx)
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")
		assert.Nil(t, keys, "TestMemoryCacheClose should return nil")

		err = cache.ForEach(ctx, func(_, _ string) bool {
			return true
		})
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")
	})

	t.Run("DoubleCloseIsSafe", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)

		err := cache.Close()
		require.NoError(t, err, "TestMemoryCacheClose should complete without error")

		err = cache.Close()
		assert.NoError(t, err, "TestMemoryCacheClose should complete without error")
	})
}

// TestMemoryCacheGC tests memory cache GC functionality.
func TestMemoryCacheGC(t *testing.T) {
	ctx := context.Background()

	t.Run("GCRemovesExpiredEntries", func(t *testing.T) {
		cache := newTestCache[string](0, 50*time.Millisecond, EvictionPolicyLRU, 100*time.Millisecond)
		defer cache.Close()

		for i := range 10 {
			_ = cache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
		}

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(10), size, "TestMemoryCacheGC should match expected value")

		time.Sleep(200 * time.Millisecond)

		size, _ = cache.Size(ctx)
		assert.Equal(t, int64(0), size, "TestMemoryCacheGC should match expected value")
	})

	t.Run("GCDoesNotRemoveNonExpiredEntries", func(t *testing.T) {
		cache := newTestCache[string](0, 1*time.Second, EvictionPolicyLRU, 100*time.Millisecond)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")

		time.Sleep(150 * time.Millisecond)

		assert.True(t, cache.Contains(ctx, "key1"), "TestMemoryCacheGC condition should be true")
	})
}

// TestMemoryCacheConcurrency tests memory cache concurrency functionality.
func TestMemoryCacheConcurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("ConcurrentWrites", func(t *testing.T) {
		cache := newTestCache[int](100, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		var wg sync.WaitGroup
		for i := range 50 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				_ = cache.Set(ctx, key, i)
			})
		}

		wg.Wait()

		size, _ := cache.Size(ctx)
		assert.LessOrEqual(t, size, int64(100), "Should match expected")
	})

	t.Run("ConcurrentReads", func(*testing.T) {
		cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		for i := range 50 {
			_ = cache.Set(ctx, fmt.Sprintf("key%d", i), i)
		}

		var wg sync.WaitGroup
		for i := range 50 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				cache.Get(ctx, key)
			})
		}

		wg.Wait()
	})

	t.Run("ConcurrentMixedOperations", func(t *testing.T) {
		cache := newTestCache[int](100, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		var wg sync.WaitGroup

		for i := range 50 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				_ = cache.Set(ctx, key, i)
			})
		}

		for i := range 50 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				cache.Get(ctx, key)
			})
		}

		for i := range 25 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				cache.Delete(ctx, key)
			})
		}

		wg.Wait()

		size, _ := cache.Size(ctx)
		assert.GreaterOrEqual(t, size, int64(0), "Should be greater or equal")
		assert.LessOrEqual(t, size, int64(100), "Should match expected")
	})

	t.Run("ConcurrentEvictions", func(t *testing.T) {
		cache := newTestCache[int](10, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		var wg sync.WaitGroup
		for i := range 100 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				_ = cache.Set(ctx, key, i)
			})
		}

		wg.Wait()

		size, _ := cache.Size(ctx)
		assert.LessOrEqual(t, size, int64(10), "Should match expected")
	})
}

// TestMemoryCacheEdgeCases tests memory cache edge cases functionality.
func TestMemoryCacheEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("ZeroMaxSizeMeansUnlimited", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		for i := range 1000 {
			err := cache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			require.NoError(t, err, "TestMemoryCacheEdgeCases should complete without error")
		}

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(1000), size, "TestMemoryCacheEdgeCases should match expected value")
	})

	t.Run("UnlimitedSizeForcesNoOpEvictionPolicy", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		mc, ok := cache.(*memoryCache[string])
		require.True(t, ok, "Type assertion to *memoryCache[string] should succeed")

		assert.Equal(t, EvictionPolicyNone, mc.evictionPolicy, "TestMemoryCacheEdgeCases should match expected value")

		_, isNoOp := mc.evictionHandler.(*NoOpEvictionHandler)
		assert.True(t, isNoOp, "Unlimited maxSize should use NoOpEvictionHandler")

		for i := range 1000 {
			err := cache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			require.NoError(t, err, "TestMemoryCacheEdgeCases should complete without error")
		}

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(1000), size, "TestMemoryCacheEdgeCases should match expected value")
	})

	t.Run("NegativeMaxSizeForcesNoOpEvictionPolicy", func(t *testing.T) {
		cache := newTestCache[string](-1, 0, EvictionPolicyLFU, 5*time.Minute)
		defer cache.Close()

		mc, ok := cache.(*memoryCache[string])
		require.True(t, ok, "Type assertion to *memoryCache[string] should succeed")

		assert.Equal(t, EvictionPolicyNone, mc.evictionPolicy, "TestMemoryCacheEdgeCases should match expected value")

		_, isNoOp := mc.evictionHandler.(*NoOpEvictionHandler)
		assert.True(t, isNoOp, "Negative maxSize should use NoOpEvictionHandler")
	})

	t.Run("MaxSizeOfOne", func(t *testing.T) {
		cache := newTestCache[string](1, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		_ = cache.Set(ctx, "key1", "value1")
		_ = cache.Set(ctx, "key2", "value2")

		size, _ := cache.Size(ctx)
		assert.Equal(t, int64(1), size, "TestMemoryCacheEdgeCases should match expected value")
		assert.False(t, cache.Contains(ctx, "key1"), "TestMemoryCacheEdgeCases condition should be false")
		assert.True(t, cache.Contains(ctx, "key2"), "TestMemoryCacheEdgeCases condition should be true")
	})

	t.Run("EmptyKey", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Set(ctx, "", "value")
		require.NoError(t, err, "TestMemoryCacheEdgeCases should complete without error")

		value, found := cache.Get(ctx, "")
		assert.True(t, found, "Should be found")
		assert.Equal(t, "value", value, "TestMemoryCacheEdgeCases should match expected value")
	})

	t.Run("EmptyValue", func(t *testing.T) {
		cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
		defer cache.Close()

		err := cache.Set(ctx, "key", "")
		require.NoError(t, err, "TestMemoryCacheEdgeCases should complete without error")

		value, found := cache.Get(ctx, "key")
		assert.True(t, found, "Should be found")
		assert.Equal(t, "", value, "TestMemoryCacheEdgeCases should match expected value")
	})

	t.Run("DifferentValueTypes", func(t *testing.T) {
		t.Run("IntCache", func(t *testing.T) {
			cache := newTestCache[int](0, 0, EvictionPolicyLRU, 5*time.Minute)
			defer cache.Close()

			_ = cache.Set(ctx, "key", 42)
			value, found := cache.Get(ctx, "key")
			assert.True(t, found, "Should be found")
			assert.Equal(t, 42, value, "TestMemoryCacheEdgeCases should match expected value")
		})

		t.Run("StructCache", func(t *testing.T) {
			type User struct {
				ID   int
				Name string
			}

			cache := newTestCache[User](0, 0, EvictionPolicyLRU, 5*time.Minute)
			defer cache.Close()

			user := User{ID: 1, Name: "Alice"}
			_ = cache.Set(ctx, "user:1", user)

			value, found := cache.Get(ctx, "user:1")
			assert.True(t, found, "Should be found")
			assert.Equal(t, user, value, "TestMemoryCacheEdgeCases should match expected value")
		})

		t.Run("PointerCache", func(t *testing.T) {
			cache := newTestCache[*string](0, 0, EvictionPolicyLRU, 5*time.Minute)
			defer cache.Close()

			str := "test"
			_ = cache.Set(ctx, "key", &str)

			value, found := cache.Get(ctx, "key")
			assert.True(t, found, "Should be found")
			assert.Equal(t, &str, value, "TestMemoryCacheEdgeCases should match expected value")
		})
	})
}

func BenchmarkMemoryCacheSet(b *testing.B) {
	cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()

	var i int
	for b.Loop() {
		key := fmt.Sprintf("key%d", i%1000)
		_ = cache.Set(ctx, key, "value")
		i++
	}
}

func BenchmarkMemoryCacheGet(b *testing.B) {
	cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
	defer cache.Close()

	ctx := context.Background()

	// Pre-populate
	for i := range 1000 {
		_ = cache.Set(ctx, fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()

	var i int
	for b.Loop() {
		key := fmt.Sprintf("key%d", i%1000)
		cache.Get(ctx, key)

		i++
	}
}

func BenchmarkMemoryCacheSetWithEviction(b *testing.B) {
	cache := newTestCache[string](100, 0, EvictionPolicyLRU, 5*time.Minute)
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()

	var i int
	for b.Loop() {
		key := fmt.Sprintf("key%d", i)
		_ = cache.Set(ctx, key, "value")
		i++
	}
}

func BenchmarkMemoryCacheConcurrent(b *testing.B) {
	cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
	defer cache.Close()

	ctx := context.Background()

	for i := range 1000 {
		_ = cache.Set(ctx, fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%1000)
			if i%2 == 0 {
				cache.Get(ctx, key)
			} else {
				_ = cache.Set(ctx, key, "value")
			}

			i++
		}
	})
}

func BenchmarkMemoryCacheSize(b *testing.B) {
	cache := newTestCache[string](0, 0, EvictionPolicyLRU, 5*time.Minute)
	defer cache.Close()

	ctx := context.Background()

	for i := range 1000 {
		_ = cache.Set(ctx, fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = cache.Size(ctx)
	}
}
