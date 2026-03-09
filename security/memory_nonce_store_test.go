package security

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMemoryNonceStore tests constructor and interface compliance.
func TestNewMemoryNonceStore(t *testing.T) {
	t.Run("CreatesValidStore", func(t *testing.T) {
		store := NewMemoryNonceStore()

		assert.NotNil(t, store, "Store should not be nil")
		_, ok := store.(*MemoryNonceStore)
		assert.True(t, ok, "Should return *MemoryNonceStore")
	})

	t.Run("ImplementsNonceStoreInterface", func(*testing.T) {
		_ = NewMemoryNonceStore()
	})
}

// TestMemoryNonceStoreStoreIfAbsent tests atomic nonce reservation scenarios.
func TestMemoryNonceStoreStoreIfAbsent(t *testing.T) {
	ctx := context.Background()

	t.Run("StoreNewNonce", func(t *testing.T) {
		store := NewMemoryNonceStore()

		stored, err := store.StoreIfAbsent(ctx, "test-app", "test-nonce", 5*time.Minute)

		require.NoError(t, err, "Should atomically store nonce without error")
		assert.True(t, stored, "First StoreIfAbsent should store nonce")
	})

	t.Run("StoreDuplicateNonce", func(t *testing.T) {
		store := NewMemoryNonceStore()

		stored, err := store.StoreIfAbsent(ctx, "test-app", "test-nonce", 5*time.Minute)
		require.NoError(t, err, "Should atomically store nonce without error")
		assert.True(t, stored, "First StoreIfAbsent should store nonce")

		stored, err = store.StoreIfAbsent(ctx, "test-app", "test-nonce", 5*time.Minute)
		require.NoError(t, err, "Should atomically check existing nonce without error")
		assert.False(t, stored, "Second StoreIfAbsent should not store duplicate nonce")
	})

	t.Run("SameNonceDifferentApps", func(t *testing.T) {
		store := NewMemoryNonceStore()

		stored, err := store.StoreIfAbsent(ctx, "app-1", "shared-nonce", 5*time.Minute)
		require.NoError(t, err, "Should store nonce for first app")
		assert.True(t, stored, "First app should store nonce")

		stored, err = store.StoreIfAbsent(ctx, "app-2", "shared-nonce", 5*time.Minute)
		require.NoError(t, err, "Should store same nonce for different app")
		assert.True(t, stored, "Different app should store same nonce independently")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		store := NewMemoryNonceStore()

		stored, err := store.StoreIfAbsent(ctx, "", "nonce", 5*time.Minute)
		require.NoError(t, err, "Should support empty appID")
		assert.True(t, stored, "Should store with empty appID")
	})

	t.Run("EmptyNonce", func(t *testing.T) {
		store := NewMemoryNonceStore()

		stored, err := store.StoreIfAbsent(ctx, "test-app", "", 5*time.Minute)
		require.NoError(t, err, "Should support empty nonce")
		assert.True(t, stored, "Should store with empty nonce")
	})
}

// TestMemoryNonceStoreTTLExpiration tests TTL behavior.
func TestMemoryNonceStoreTTLExpiration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryNonceStore()

	t.Run("NonceCanBeReusedAfterTTL", func(t *testing.T) {
		stored, err := store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		require.NoError(t, err, "Should store expiring nonce without error")
		assert.True(t, stored, "First store should succeed")

		stored, err = store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		require.NoError(t, err, "Should check duplicate before expiry without error")
		assert.False(t, stored, "Duplicate should be rejected before TTL expiration")

		time.Sleep(100 * time.Millisecond)

		stored, err = store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		require.NoError(t, err, "Should allow reuse after TTL expiration")
		assert.True(t, stored, "Nonce should be reusable after expiration")
	})
}

// TestMemoryNonceStoreKeyFormat tests key handling with special characters.
func TestMemoryNonceStoreKeyFormat(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryNonceStore()

	t.Run("SpecialCharacters", func(t *testing.T) {
		testCases := []struct {
			appID string
			nonce string
		}{
			{"app:with:colons", "nonce:with:colons"},
			{"app-with-dashes", "nonce-with-dashes"},
			{"app_with_underscores", "nonce_with_underscores"},
			{"app.with.dots", "nonce.with.dots"},
			{"app/with/slashes", "nonce/with/slashes"},
			{"应用", "随机数"},
		}

		for _, tc := range testCases {
			stored, err := store.StoreIfAbsent(ctx, tc.appID, tc.nonce, 5*time.Minute)
			require.NoError(t, err, "Should store nonce with special characters")
			assert.True(t, stored, "First store should succeed for appID=%q nonce=%q", tc.appID, tc.nonce)
		}
	})
}

// TestMemoryNonceStoreConcurrency tests concurrent reservation behavior.
func TestMemoryNonceStoreConcurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("ConcurrentSameNonceOnlyOneSucceeds", func(t *testing.T) {
		store := NewMemoryNonceStore()

		const numGoroutines = 100

		var successCount atomic.Int32

		errCh := make(chan error, numGoroutines)

		var wg sync.WaitGroup
		for range numGoroutines {
			wg.Go(func() {
				stored, err := store.StoreIfAbsent(ctx, "test-app", "same-nonce", 5*time.Minute)
				if err != nil {
					errCh <- err

					return
				}

				if stored {
					successCount.Add(1)
				}
			})
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			require.NoError(t, err, "Concurrent StoreIfAbsent should not error")
		}

		assert.Equal(t, int32(1), successCount.Load(), "Exactly one goroutine should reserve the nonce")
	})

	t.Run("ConcurrentDifferentAppsAllSucceed", func(t *testing.T) {
		store := NewMemoryNonceStore()

		const numGoroutines = 50

		var successCount atomic.Int32

		errCh := make(chan error, numGoroutines)

		var wg sync.WaitGroup
		for i := range numGoroutines {
			wg.Go(func() {
				stored, err := store.StoreIfAbsent(ctx, "app-"+string(rune('a'+i%26))+string(rune('0'+i%10)), "same-nonce", 5*time.Minute)
				if err != nil {
					errCh <- err

					return
				}

				if stored {
					successCount.Add(1)
				}
			})
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			require.NoError(t, err, "Concurrent StoreIfAbsent should not error")
		}

		assert.Equal(t, int32(numGoroutines), successCount.Load(), "All goroutines should reserve nonce for different apps")
	})
}

// TestMemoryNonceStoreContextHandling tests behavior with canceled/expired contexts.
func TestMemoryNonceStoreContextHandling(t *testing.T) {
	t.Run("CancelledContext", func(t *testing.T) {
		store := NewMemoryNonceStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		stored, err := store.StoreIfAbsent(ctx, "test-app", "test-nonce", 5*time.Minute)
		require.NoError(t, err, "Should store nonce with canceled context without error")
		assert.True(t, stored, "Should still reserve nonce with canceled context")
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		store := NewMemoryNonceStore()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(1 * time.Millisecond)

		stored, err := store.StoreIfAbsent(ctx, "test-app", "test-nonce", 5*time.Minute)
		require.NoError(t, err, "Should store nonce with timeout context without error")
		assert.True(t, stored, "Should reserve nonce with timeout context")
	})
}
