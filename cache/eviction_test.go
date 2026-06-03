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
	handler := new(noOpEvictionHandler)
	require.NotNil(t, handler, "TestNoOpEvictionHandler should return a non-nil value")

	t.Run("AllOperationsNoOp", func(*testing.T) {
		handler.OnAccess("key1")
		handler.OnInsert("key1")
		handler.OnEvict("key1")
		handler.Reset()
	})

	t.Run("AlwaysReturnEmptyCandidate", func(*testing.T) {
		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestNoOpEvictionHandler should match expected value")
	})

	t.Run("HandleMultipleOperations", func(*testing.T) {
		for i := range 100 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
			handler.OnAccess(fmt.Sprintf("key%d", i))
		}

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestNoOpEvictionHandler should match expected value")
	})
}

// TestLRUHandler tests LRU handler functionality.
func TestLRUHandler(t *testing.T) {
	t.Run("BasicInsertionAndEviction", func(*testing.T) {
		handler := newLruHandler()
		require.NotNil(t, handler, "TestLRUHandler should return a non-nil value")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("AccessUpdatesRecency", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("EvictionRemovesEntry", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnEvict("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("MultipleAccessesMaintainOrder", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key2")
		handler.OnAccess("key1")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("ResetClearsAllEntries", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(*testing.T) {
		handler := newLruHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("SingleEntry", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("EvictNonExistentKey", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("AccessNonExistentKeyCreatesEntry", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandler should match expected value")
	})

	t.Run("ConcurrentOperations", func(*testing.T) {
		handler := newLruHandler()

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

	t.Run("StressTestWithManyEntries", func(*testing.T) {
		handler := newLruHandler()

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
	t.Run("BasicInsertionAndEviction", func(*testing.T) {
		handler := newFifoHandler()
		require.NotNil(t, handler, "TestFIFOHandler should return a non-nil value")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("AccessDoesNotAffectOrder", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("EvictionRemovesEntry", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnEvict("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("ResetClearsAllEntries", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(*testing.T) {
		handler := newFifoHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("SingleEntry", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("DuplicateInsertIgnored", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("EvictNonExistentKey", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandler should match expected value")
	})

	t.Run("ConcurrentOperations", func(*testing.T) {
		handler := newFifoHandler()

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
	t.Run("BasicInsertionAndEviction", func(*testing.T) {
		handler := newLfuHandler()
		require.NotNil(t, handler, "TestLFUHandler should return a non-nil value")

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("AccessIncreasesFrequency", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key3", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("EvictionRemovesEntry", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("TieBreakingByInsertionOrder", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("FrequencyOrderingMaintained", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key2")
		handler.OnAccess("key3")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("ResetClearsAllEntries", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.Reset()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("EmptyHandlerReturnsEmptyCandidate", func(*testing.T) {
		handler := newLfuHandler()

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("SingleEntry", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("EvictNonExistentKey", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("AccessNonExistentKeyCreatesEntry", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key1")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("ConcurrentOperations", func(*testing.T) {
		handler := newLfuHandler()

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

	t.Run("StressTestWithManyEntries", func(*testing.T) {
		handler := newLfuHandler()

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

	t.Run("FrequencyBucketsWorkCorrectly", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key2")
		handler.OnInsert("key3")
		handler.OnAccess("key3")
		handler.OnAccess("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLFUHandler should match expected value")

		handler.OnEvict("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLFUHandler should match expected value")

		handler.OnEvict("key2")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key3", candidate, "TestLFUHandler should match expected value")
	})

	t.Run("DrainingMinBucketViaAccessKeepsCandidate", func(*testing.T) {
		// Regression: accessing every key in the minimum-frequency bucket used
		// to leave minFreq stale, making SelectEvictionCandidate return "".
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		// Bump both keys off frequency 1, fully draining the min bucket.
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		assert.Equal(t, int64(2), handler.minFreq, "minFreq should track the surviving bucket after the min bucket drains")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "draining the min bucket must still yield a valid victim (FIFO within freq 2)")
	})

	t.Run("SingleEntryAccessKeepsCandidate", func(*testing.T) {
		// Regression: the only key drains its freq-1 bucket on access; the
		// handler must still report it as the eviction candidate.
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key1")

		assert.Equal(t, int64(2), handler.minFreq, "minFreq should advance to the surviving frequency")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "single accessed key must remain selectable for eviction")
	})
}

// TestNewEvictionHandler tests that newEvictionHandler maps each policy to its handler.
func TestNewEvictionHandler(t *testing.T) {
	testCases := []struct {
		policy       EvictionPolicy
		expectedType string
	}{
		{EvictionPolicyNone, "*cache.noOpEvictionHandler"},
		{EvictionPolicyLRU, "*cache.lruHandler"},
		{EvictionPolicyLFU, "*cache.lfuHandler"},
		{EvictionPolicyFIFO, "*cache.fifoHandler"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("policy_%d", tc.policy), func(*testing.T) {
			handler := newEvictionHandler(tc.policy)
			require.NotNil(t, handler, "newEvictionHandler should return a non-nil handler")

			typeName := fmt.Sprintf("%T", handler)
			assert.Equal(t, tc.expectedType, typeName, "newEvictionHandler should return the handler matching the policy")
		})
	}

	t.Run("InvalidPolicyDefaultsToNoOp", func(*testing.T) {
		handler := newEvictionHandler(EvictionPolicy(999))
		require.NotNil(t, handler, "newEvictionHandler should return a non-nil handler")

		typeName := fmt.Sprintf("%T", handler)
		assert.Equal(t, "*cache.noOpEvictionHandler", typeName, "unknown policy should fall back to the no-op handler")
	})
}

// TestLRUHandlerUpdateBehavior tests LRU handler update behavior functionality.
func TestLRUHandlerUpdateBehavior(t *testing.T) {
	t.Run("UpdateMoveKeyToFront", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandlerUpdateBehavior should match expected value")

		handler.OnAccess("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLRUHandlerUpdateBehavior should match expected value")
	})

	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.accessMap), "TestLRUHandlerUpdateBehavior should match expected value")
		assert.Equal(t, 1, handler.accessList.Len(), "TestLRUHandlerUpdateBehavior should match expected value")
	})

	t.Run("InterleavedInsertsAndAccesses", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key1")
		handler.OnInsert("key2")
		handler.OnAccess("key1")
		handler.OnInsert("key3")
		handler.OnAccess("key2")

		assert.Equal(t, 3, len(handler.accessMap), "TestLRUHandlerUpdateBehavior should match expected value")
		assert.Equal(t, 3, handler.accessList.Len(), "TestLRUHandlerUpdateBehavior should match expected value")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestLRUHandlerUpdateBehavior should match expected value")
	})
}

// TestLFUHandlerUpdateBehavior tests LFU handler update behavior functionality.
func TestLFUHandlerUpdateBehavior(t *testing.T) {
	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.keyToNode), "TestLFUHandlerUpdateBehavior should match expected value")
		assert.Equal(t, 1, len(handler.keyToBucket), "TestLFUHandlerUpdateBehavior should match expected value")
	})

	t.Run("FrequencyIncrementsCorrectly", func(*testing.T) {
		handler := newLfuHandler()

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
		assert.Equal(t, "key3", candidate, "TestLFUHandlerUpdateBehavior should match expected value")

		handler.OnEvict("key3")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestLFUHandlerUpdateBehavior should match expected value")
	})

	t.Run("FrequencyBucketMovement", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		assert.Equal(t, int64(1), handler.minFreq, "TestLFUHandlerUpdateBehavior should match expected value")

		handler.OnAccess("key1")

		assert.Equal(t, int64(1), handler.minFreq, "TestLFUHandlerUpdateBehavior should match expected value")

		handler.OnEvict("key2")
		handler.OnEvict("key3")

		assert.Equal(t, int64(2), handler.minFreq, "TestLFUHandlerUpdateBehavior should match expected value")

		assert.Equal(t, 1, len(handler.keyToNode), "TestLFUHandlerUpdateBehavior should match expected value")
	})
}

// TestFIFOHandlerUpdateBehavior tests FIFO handler update behavior functionality.
func TestFIFOHandlerUpdateBehavior(t *testing.T) {
	t.Run("RepeatedUpdatesDoNotCauseDuplicates", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")

		for range 10 {
			handler.OnAccess("key1")
		}

		assert.Equal(t, 1, len(handler.insertMap), "TestFIFOHandlerUpdateBehavior should match expected value")
		assert.Equal(t, 1, handler.insertList.Len(), "TestFIFOHandlerUpdateBehavior should match expected value")
	})

	t.Run("AccessDoesNotChangeOrder", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")
		handler.OnInsert("key3")

		handler.OnAccess("key3")
		handler.OnAccess("key1")
		handler.OnAccess("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestFIFOHandlerUpdateBehavior should match expected value")
	})
}

// TestEvictionHandlerInternalConsistency tests eviction handler internal consistency functionality.
func TestEvictionHandlerInternalConsistency(t *testing.T) {
	t.Run("LRUHandlerConsistency", func(*testing.T) {
		handler := newLruHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.accessMap), "TestEvictionHandlerInternalConsistency should match expected value")
		assert.Equal(t, 2, handler.accessList.Len(), "TestEvictionHandlerInternalConsistency should match expected value")
	})

	t.Run("LFUHandlerConsistency", func(*testing.T) {
		handler := newLfuHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.keyToNode), "TestEvictionHandlerInternalConsistency should match expected value")
		assert.Equal(t, 2, len(handler.keyToBucket), "TestEvictionHandlerInternalConsistency should match expected value")
	})

	t.Run("FIFOHandlerConsistency", func(*testing.T) {
		handler := newFifoHandler()

		for range 100 {
			handler.OnInsert("key1")
			handler.OnAccess("key1")
			handler.OnInsert("key2")
			handler.OnAccess("key2")
		}

		assert.Equal(t, 2, len(handler.insertMap), "TestEvictionHandlerInternalConsistency should match expected value")
		assert.Equal(t, 2, handler.insertList.Len(), "TestEvictionHandlerInternalConsistency should match expected value")
	})
}

// TestEvictionHandlerEdgeCases tests eviction handler edge cases functionality.
func TestEvictionHandlerEdgeCases(t *testing.T) {
	t.Run("LRUHandlerEvictAndReinsert", func(*testing.T) {
		handler := newLruHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		handler.OnEvict("key1")
		assert.Equal(t, 1, len(handler.accessMap), "TestEvictionHandlerEdgeCases should match expected value")

		handler.OnInsert("key1")
		assert.Equal(t, 2, len(handler.accessMap), "TestEvictionHandlerEdgeCases should match expected value")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestEvictionHandlerEdgeCases should match expected value")
	})

	t.Run("LFUHandlerEvictAndReinsert", func(*testing.T) {
		handler := newLfuHandler()

		handler.OnInsert("key1")
		handler.OnAccess("key1")
		handler.OnAccess("key1")

		handler.OnInsert("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestEvictionHandlerEdgeCases should match expected value")

		handler.OnEvict("key2")

		handler.OnInsert("key2")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestEvictionHandlerEdgeCases should match expected value")
	})

	t.Run("FIFOHandlerEvictAndReinsert", func(*testing.T) {
		handler := newFifoHandler()

		handler.OnInsert("key1")
		handler.OnInsert("key2")

		candidate := handler.SelectEvictionCandidate()
		assert.Equal(t, "key1", candidate, "TestEvictionHandlerEdgeCases should match expected value")

		handler.OnEvict("key1")

		handler.OnInsert("key1")

		candidate = handler.SelectEvictionCandidate()
		assert.Equal(t, "key2", candidate, "TestEvictionHandlerEdgeCases should match expected value")
	})
}

// TestEvictionHandlerLargeScale tests eviction handler large scale functionality.
func TestEvictionHandlerLargeScale(t *testing.T) {
	t.Run("LRUHandlerLargeScale", func(*testing.T) {
		handler := newLruHandler()

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

		assert.Equal(t, 5000, len(handler.accessMap), "TestEvictionHandlerLargeScale should match expected value")
		assert.Equal(t, 5000, handler.accessList.Len(), "TestEvictionHandlerLargeScale should match expected value")
	})

	t.Run("LFUHandlerLargeScale", func(*testing.T) {
		handler := newLfuHandler()

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

		assert.Equal(t, 5000, len(handler.keyToNode), "TestEvictionHandlerLargeScale should match expected value")
	})

	t.Run("FIFOHandlerLargeScale", func(*testing.T) {
		handler := newFifoHandler()

		for i := range 10000 {
			handler.OnInsert(fmt.Sprintf("key%d", i))
		}

		for i := range 5000 {
			candidate := handler.SelectEvictionCandidate()
			assert.Equal(t, fmt.Sprintf("key%d", i), candidate, "TestEvictionHandlerLargeScale should match expected value")
			handler.OnEvict(candidate)
		}

		assert.Equal(t, 5000, len(handler.insertMap), "TestEvictionHandlerLargeScale should match expected value")
		assert.Equal(t, 5000, handler.insertList.Len(), "TestEvictionHandlerLargeScale should match expected value")
	})
}

func BenchmarkLRUHandler(b *testing.B) {
	handler := newLruHandler()

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
	handler := newLfuHandler()

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
	handler := newFifoHandler()

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
	handler := newLfuHandler()

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
