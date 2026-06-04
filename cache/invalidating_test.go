package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
)

func TestInvalidating(t *testing.T) {
	logger := ilogx.Named("test:invalidating")
	ctx := context.Background()

	t.Run("CachesAfterFirstLoad", func(t *testing.T) {
		var calls atomic.Int64

		inv := NewInvalidating(func(_ context.Context, key string) (string, error) {
			calls.Add(1)

			return "v:" + key, nil
		}, logger)

		got, err := inv.Get(ctx, "a")
		require.NoError(t, err, "first Get should load without error")
		require.Equal(t, "v:a", got, "first Get should return the loaded value")

		got2, err := inv.Get(ctx, "a")
		require.NoError(t, err, "second Get should serve from cache")
		require.Equal(t, "v:a", got2, "cached Get should return the same value")
		require.Equal(t, int64(1), calls.Load(), "loader must run once for a cached key")
	})

	t.Run("InvalidateSpecificKeyReloadsOnlyThatKey", func(t *testing.T) {
		var calls atomic.Int64

		inv := NewInvalidating(func(_ context.Context, _ string) (int64, error) {
			return calls.Add(1), nil
		}, logger)

		first, err := inv.Get(ctx, "k")
		require.NoError(t, err, "initial Get should succeed")
		require.Equal(t, int64(1), first, "initial Get should load generation 1")

		other, err := inv.Get(ctx, "other")
		require.NoError(t, err, "Get for a second key should succeed")
		require.Equal(t, int64(2), other, "second key should load generation 2")

		require.NoError(t, inv.Invalidate(ctx, "k"), "invalidating a specific key should not error")

		reloaded, err := inv.Get(ctx, "k")
		require.NoError(t, err, "Get after invalidation should reload")
		require.Equal(t, int64(3), reloaded, "the invalidated key must reload")

		survived, err := inv.Get(ctx, "other")
		require.NoError(t, err, "untouched key should still be cached")
		require.Equal(t, int64(2), survived, "a non-invalidated key must keep its cached value")
	})

	t.Run("InvalidateAllClearsEveryKey", func(t *testing.T) {
		var calls atomic.Int64

		inv := NewInvalidating(func(_ context.Context, _ string) (int64, error) {
			return calls.Add(1), nil
		}, logger)

		_, err := inv.Get(ctx, "a")
		require.NoError(t, err, "Get a should succeed")
		_, err = inv.Get(ctx, "b")
		require.NoError(t, err, "Get b should succeed")

		require.NoError(t, inv.Invalidate(ctx), "clearing the whole cache should not error")

		a, err := inv.Get(ctx, "a")
		require.NoError(t, err, "Get a after clear should reload")
		b, err := inv.Get(ctx, "b")
		require.NoError(t, err, "Get b after clear should reload")
		require.Equal(t, int64(3), a, "a must reload after a full clear")
		require.Equal(t, int64(4), b, "b must reload after a full clear")
	})

	t.Run("LoaderErrorPropagates", func(t *testing.T) {
		sentinel := errors.New("boom")
		inv := NewInvalidating(func(_ context.Context, _ string) (string, error) {
			return "", sentinel
		}, logger)

		_, err := inv.Get(ctx, "x")
		require.ErrorIs(t, err, sentinel, "loader error must propagate to the caller")
	})

	t.Run("SingleflightMergesConcurrentLoads", func(t *testing.T) {
		var calls atomic.Int64

		inv := NewInvalidating(func(_ context.Context, key string) (string, error) {
			calls.Add(1)

			return "v:" + key, nil
		}, logger)

		const n = 20

		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				_, _ = inv.Get(ctx, "hot")
			})
		}

		wg.Wait()

		require.Equal(t, int64(1), calls.Load(), "concurrent Gets for one key must trigger a single load")
	})
}
