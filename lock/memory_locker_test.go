package lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLocker(t *testing.T) {
	locker := NewMemoryLocker()

	steal := func(t *testing.T, name string) {
		t.Helper()

		store, ok := locker.(*MemoryLocker)
		require.True(t, ok, "the memory locker should expose its concrete type for the steal hook")

		store.mu.Lock()
		delete(store.holders, name)
		store.mu.Unlock()
	}

	runLockerContract(t, locker, steal)
}

func TestMemoryLockerRemovesExpiredDynamicNames(t *testing.T) {
	locker := NewMemoryLocker()
	store, ok := locker.(*MemoryLocker)
	require.True(t, ok, "the constructor should return the concrete memory locker")

	for _, name := range []string{"dynamic:a", "dynamic:b", "dynamic:c"} {
		_, err := locker.TryAcquire(context.Background(), name, WithTTL(20*time.Millisecond))
		require.NoError(t, err, "each dynamic name should acquire")
	}

	require.Eventually(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()

		return len(store.holders) == 0
	}, time.Second, 10*time.Millisecond, "expired dynamic names should be removed without being reacquired")

	store.mu.Lock()
	defer store.mu.Unlock()

	assert.Empty(t, store.holders, "the holder map should not retain expired names")
}

func TestMemoryLockerRefreshReschedulesExpiry(t *testing.T) {
	ctx := context.Background()
	locker := NewMemoryLocker()
	held, err := locker.TryAcquire(ctx, "refresh-expiry", WithTTL(50*time.Millisecond))
	require.NoError(t, err, "the lock should acquire")

	time.Sleep(30 * time.Millisecond)
	require.NoError(t, held.Refresh(ctx), "refresh should extend the expiry timer")
	time.Sleep(30 * time.Millisecond)

	_, err = locker.TryAcquire(ctx, "refresh-expiry")
	require.ErrorIs(t, err, ErrNotAcquired, "the original timer must not delete a refreshed holder")
	require.NoError(t, held.Release(ctx), "cleanup release should succeed")
}
