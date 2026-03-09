package cache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoOpEvictionHandler tests no op eviction handler functionality.
func TestNoOpEvictionHandler(t *testing.T) {
	handler := NewNoOpEvictionHandler()
	require.NotNil(t, handler, "Should not be nil")

	t.Run("AllOperationsNoOp", func(_ *testing.T) {
		handler.OnAccess("key1")
		handler.OnInsert("key1")
		handler.OnEvict("key1")
		handler.Reset()
	})

	t.Run("AlwaysReturnEmptyCandidate", func(_ *testing.T) {
		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("HandleMultipleOperations", func(_ *testing.T) {
		for i := range 100 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
			handler.OnAccess(fmt.Sprintf("key%d", i))
		}

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})
}

// TestLRUHandler tests LRU handler functionality.
func TestLRUHandler(t *testing.T) {
	t.Run("BasicInsertionAndEviction", func(_ *testing.T) {
		handler := NewLruHandler()
		require.NotNil(t, handler, "Should not be nil")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("AccessUpdatesRecency", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("EvictionRemovesEntry", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnEvict("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("MultipleAccessesMaintainOrder", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key2")
		handler.OnAccess("key1")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("ResetClearsAllEntries", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(_ *testing.T) {
		handler := NewLruHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("SingleEntry", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("EvictNonExistentKey", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("AccessNonExistentKeyCreatesEntry", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("ConcurrentOperations", func(_ *testing.T) {
		handler := NewLruHandler()

		var wg sync.WaitGroup

		for i := range 100 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i%26)
				handler.OnInsert(key)
				handler.OnAccess(key)
			})
		}

		wg.Wait()

		candidate := handler.SelectEvictionCandidate()
		assert.NotEqual(t, "", candidate, "Should not equal")
	})

	t.Run("StressTestWithManyEntries", func(_ *testing.T) {
		handler := NewLruHandler()

		for i := range 1000 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := range 500 {
			handler.OnAccess(fmt.Sprintf("key%d", i*2))
		}

		candidate := handler.SelectEvictionCandidate()
		assert.NotEqual(t, "", candidate, "Should not equal")
	})
}

// TestFIFOHandler tests FIFO handler functionality.
func TestFIFOHandler(t *testing.T) {
	t.Run("BasicInsertionAndEviction", func(_ *testing.T) {
		handler := NewFifoHandler()
		require.NotNil(t, handler, "Should not be nil")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("AccessDoesNotAffectOrder", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("EvictionRemovesEntry", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnEvict("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("ResetClearsAllEntries", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(_ *testing.T) {
		handler := NewFifoHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("SingleEntry", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("DuplicateInsertIgnored", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("EvictNonExistentKey", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("ConcurrentOperations", func(_ *testing.T) {
		handler := NewFifoHandler()

		var wg sync.WaitGroup

		// Concurrent inserts
		for i := range 100 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i)
				handler.OnInsert(key)
				handler.OnAccess(key)
			})
		}

		wg.Wait()

		candidate := handler.SelectEvictionCandidate()
		assert.NotEqual(t, "", candidate, "Should not equal")
	})
}

// TestLFUHandler tests LFU handler functionality.
func TestLFUHandler(t *testing.T) {
	t.Run("BasicInsertionAndEviction", func(_ *testing.T) {
		handler := NewLfuHandler()
		require.NotNil(t, handler, "Should not be nil")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("AccessIncreasesFrequency", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key3", candidate, "Should equal expected value")
	})

	t.Run("EvictionRemovesEntry", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("TieBreakingByInsertionOrder", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("FrequencyOrderingMaintained", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key2")
		handler.OnAccess("key3")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("ResetClearsAllEntries", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(_ *testing.T) {
		handler := NewLfuHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "Should equal expected value")
	})

	t.Run("SingleEntry", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("EvictNonExistentKey", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})

	t.Run("AccessNonExistentKeyCreatesEntry", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key1")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("ConcurrentOperations", func(_ *testing.T) {
		handler := NewLfuHandler()

		var wg sync.WaitGroup

		for i := range 100 {
			wg.Go(func() {
				key := fmt.Sprintf("key%d", i%26)
				handler.OnInsert(key)

				for range i % 10 {
					handler.OnAccess(key)
				}
			})
		}

		wg.Wait()

		candidate := handler.SelectEvictionCandidate()
		_ = candidate
	})

	t.Run("StressTestWithManyEntries", func(_ *testing.T) {
		handler := NewLfuHandler()

		n := 1000
		for i := range n {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := range n {
			for range i % 100 {
				handler.OnAccess(fmt.Sprintf("key%d", i))
			}
		}

		for range 100 {
			candidate := handler.SelectEvictionCandidate()
			require.NotEqual(t, "", candidate, "Should not equal")
			handler.OnEvict(candidate)
		}
	})

	t.Run("FrequencyBucketsWorkCorrectly", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key2")
		handler.OnInsert("key3")
		handler.OnAccess("key3")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")

		handler.OnEvict("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")

		handler.OnEvict("key2")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key3", candidate, "Should equal expected value")
	})
}

// TestEvictionHandlerFactory tests eviction handler factory functionality.
func TestEvictionHandlerFactory(t *testing.T) {
	factory := &EvictionHandlerFactory{}
	require.NotNil(t, factory, "Should not be nil")

	testCases := []struct {
		policy       EvictionPolicy
		expectedType string
	}{
		{EvictionPolicyNone, "*cache.NoOpEvictionHandler"},
		{EvictionPolicyLRU, "*cache.LruHandler"},
		{EvictionPolicyLFU, "*cache.LfuHandler"},
		{EvictionPolicyFIFO, "*cache.FifoHandler"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("policy_%d", tc.policy), func(_ *testing.T) {
			handler := factory.CreateHandler(tc.policy)
			require.NotNil(t, handler, "Should not be nil")

			typeName := fmt.Sprintf("%T", handler)
			assert.Equal(t, tc.expectedType, typeName, "Should equal expected value")
		})
	}

	t.Run("InvalidPolicyDefaultsToNoOp", func(_ *testing.T) {
		handler := factory.CreateHandler(EvictionPolicy(999))
		require.NotNil(t, handler, "Should not be nil")

		typeName := fmt.Sprintf("%T", handler)
		assert.Equal(t, "*cache.NoOpEvictionHandler", typeName, "Should equal expected value")
	})
}

// TestLRUHandlerUpdateBehavior tests LRU handler update behavior functionality.
func TestLRUHandlerUpdateBehavior(t *testing.T) {
	t.Run("UpdateMoveKeyToFront", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")

		handler.OnAccess("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.accessMap), "Should equal expected value")
		assert.Equal(t, 1, handler.accessList.Len(), "Should equal expected value")
	})

	t.Run("InterleavedInsertsAndAccesses", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key1")
		handler.OnInsert("key3")
		handler.OnAccess("key2")

		assert.Equal(t, 3, len(handler.accessMap), "Should equal expected value")
		assert.Equal(t, 3, handler.accessList.Len(), "Should equal expected value")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})
}

// TestLFUHandlerUpdateBehavior tests LFU handler update behavior functionality.
func TestLFUHandlerUpdateBehavior(t *testing.T) {
	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.keyToNode), "Should equal expected value")
		assert.Equal(t, 1, len(handler.keyToBucket), "Should equal expected value")
	})

	t.Run("FrequencyIncrementsCorrectly", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		for range 5 {
			handler.OnAccess("key1")
		}

		for range 3 {
			handler.OnAccess("key2")
		}

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key3", candidate, "Should equal expected value")

		handler.OnEvict("key3")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("FrequencyBucketMovement", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		assert.Equal(t, int64(1), handler.minFreq, "Should equal expected value")

		handler.OnAccess("key1")

		assert.Equal(t, int64(1), handler.minFreq, "Should equal expected value")

		handler.OnEvict("key2")
		handler.OnEvict("key3")

		assert.Equal(t, int64(2), handler.minFreq, "Should equal expected value")

		assert.Equal(t, 1, len(handler.keyToNode), "Should equal expected value")
	})
}

// TestFIFOHandlerUpdateBehavior tests FIFO handler update behavior functionality.
func TestFIFOHandlerUpdateBehavior(t *testing.T) {
	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.insertMap), "Should equal expected value")
		assert.Equal(t, 1, handler.insertList.Len(), "Should equal expected value")
	})

	t.Run("AccessDoesNotChangeOrder", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key3")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")
	})
}

// TestEvictionHandlerInternalConsistency tests eviction handler internal consistency functionality.
func TestEvictionHandlerInternalConsistency(t *testing.T) {
	t.Run("LRUHandlerConsistency", func(_ *testing.T) {
		handler := NewLruHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.accessMap), "Should equal expected value")
		assert.Equal(t, 2, handler.accessList.Len(), "Should equal expected value")
	})

	t.Run("LFUHandlerConsistency", func(_ *testing.T) {
		handler := NewLfuHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.keyToNode), "Should equal expected value")
		assert.Equal(t, 2, len(handler.keyToBucket), "Should equal expected value")
	})

	t.Run("FIFOHandlerConsistency", func(_ *testing.T) {
		handler := NewFifoHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.insertMap), "Should equal expected value")
		assert.Equal(t, 2, handler.insertList.Len(), "Should equal expected value")
	})
}

// TestEvictionHandlerEdgeCases tests eviction handler edge cases functionality.
func TestEvictionHandlerEdgeCases(t *testing.T) {
	t.Run("LRUHandlerEvictAndReinsert", func(_ *testing.T) {
		handler := NewLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key1")
		assert.Equal(t, 1, len(handler.accessMap), "Should equal expected value")

		handler.OnInsert("key1")
		assert.Equal(t, 2, len(handler.accessMap), "Should equal expected value")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("LFUHandlerEvictAndReinsert", func(_ *testing.T) {
		handler := NewLfuHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key1")

		handler.OnInsert("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")

		handler.OnEvict("key2")

		handler.OnInsert("key2")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})

	t.Run("FIFOHandlerEvictAndReinsert", func(_ *testing.T) {
		handler := NewFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "Should equal expected value")

		handler.OnEvict("key1")

		handler.OnInsert("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "Should equal expected value")
	})
}

// TestEvictionHandlerLargeScale tests eviction handler large scale functionality.
func TestEvictionHandlerLargeScale(t *testing.T) {
	t.Run("LRUHandlerLargeScale", func(_ *testing.T) {
		handler := NewLruHandler()

		for i := range 10000 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := 0; i < 10000; i += 10 {
			handler.OnAccess(fmt.Sprintf("key%d", i))
		}

		for range 5000 {
			candidate := handler.SelectEvictionCandidate()
			assert.NotEqual(t, "", candidate, "Should not equal")
			handler.OnEvict(candidate)
		}

		assert.Equal(t, 5000, len(handler.accessMap), "Should equal expected value")
		assert.Equal(t, 5000, handler.accessList.Len(), "Should equal expected value")
	})

	t.Run("LFUHandlerLargeScale", func(_ *testing.T) {
		handler := NewLfuHandler()

		for i := range 10000 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := range 10000 {
			for j := 0; j < i%10; j++ {
				handler.OnAccess(fmt.Sprintf("key%d", i))
			}
		}

		for range 5000 {
			candidate := handler.SelectEvictionCandidate()
			assert.NotEqual(t, "", candidate, "Should not equal")
			handler.OnEvict(candidate)
		}

		assert.Equal(t, 5000, len(handler.keyToNode), "Should equal expected value")
	})

	t.Run("FIFOHandlerLargeScale", func(_ *testing.T) {
		handler := NewFifoHandler()

		for i := range 10000 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := range 5000 {
			candidate := handler.SelectEvictionCandidate()
			assert.Equal(t, fmt.Sprintf("key%d", i), candidate, "Should equal expected value")
			handler.OnEvict(candidate)
		}

		assert.Equal(t, 5000, len(handler.insertMap), "Should equal expected value")
		assert.Equal(t, 5000, handler.insertList.Len(), "Should equal expected value")
	})
}

func BenchmarkLRUHandler(b *testing.B) {
	handler := NewLruHandler()

	for i := range 1000 {
		handler.OnInsert(fmt.Sprintf("key%d", i))
	}

	b.ResetTimer()
	b.Run("OnAccess", func(b *testing.B) {
		var i int
		for b.Loop() {
			handler.OnAccess(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})

	b.Run("SelectEvictionCandidate", func(b *testing.B) {
		for b.Loop() {
			handler.SelectEvictionCandidate()
		}
	})
}

func BenchmarkLFUHandler(b *testing.B) {
	handler := NewLfuHandler()

	for i := range 1000 {
		handler.OnInsert(fmt.Sprintf("key%d", i))
	}

	b.ResetTimer()
	b.Run("OnAccess", func(b *testing.B) {
		var i int
		for b.Loop() {
			handler.OnAccess(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})

	b.Run("SelectEvictionCandidate", func(b *testing.B) {
		for b.Loop() {
			handler.SelectEvictionCandidate()
		}
	})
}

func BenchmarkFIFOHandler(b *testing.B) {
	handler := NewFifoHandler()

	for i := range 1000 {
		handler.OnInsert(fmt.Sprintf("key%d", i))
	}

	b.ResetTimer()
	b.Run("OnAccess", func(b *testing.B) {
		var i int
		for b.Loop() {
			handler.OnAccess(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})

	b.Run("SelectEvictionCandidate", func(b *testing.B) {
		for b.Loop() {
			handler.SelectEvictionCandidate()
		}
	})
}

func BenchmarkLFUHandlerConcurrent(b *testing.B) {
	handler := NewLfuHandler()

	for i := range 1000 {
		handler.OnInsert(fmt.Sprintf("key%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			handler.OnAccess(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})
}
