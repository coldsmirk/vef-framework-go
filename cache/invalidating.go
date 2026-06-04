package cache

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/logx"
)

// KeyedLoaderFunc loads the value for a single cache key. It is the per-key
// counterpart of LoaderFunc, used by Invalidating where the key is known up
// front.
type KeyedLoaderFunc[T any] func(ctx context.Context, key string) (T, error)

// Invalidating is a read-through cache whose entries are evicted out-of-band —
// typically when a domain event reports which keys changed. It owns an
// in-memory cache (which already coordinates concurrent loads to prevent
// stampede) and leaves the choice of invalidation trigger to the caller: wire
// a subscription that forwards the affected keys to Invalidate.
type Invalidating[T any] struct {
	cache  Cache[T]
	loader KeyedLoaderFunc[T]
	logger logx.Logger
}

// NewInvalidating builds an Invalidating cache that loads missing keys with
// loader and reports eviction activity through logger.
func NewInvalidating[T any](loader KeyedLoaderFunc[T], logger logx.Logger) *Invalidating[T] {
	return &Invalidating[T]{
		cache:  NewMemory[T](),
		loader: loader,
		logger: logger,
	}
}

// Get returns the value for key, loading and caching it on a miss.
func (i *Invalidating[T]) Get(ctx context.Context, key string) (T, error) {
	return i.cache.GetOrLoad(ctx, key, func(ctx context.Context) (T, error) {
		return i.loader(ctx, key)
	})
}

// Invalidate evicts the named keys, or the entire cache when keys is empty.
func (i *Invalidating[T]) Invalidate(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		if err := i.cache.Clear(ctx); err != nil {
			i.logger.Errorf("Failed to clear cache: %v", err)

			return err
		}

		i.logger.Info("Cleared all cache entries")

		return nil
	}

	for _, key := range keys {
		if err := i.cache.Delete(ctx, key); err != nil {
			i.logger.Errorf("Failed to delete cache entry %q: %v", key, err)

			return err
		}

		i.logger.Infof("Cleared cache entry %q", key)
	}

	return nil
}
