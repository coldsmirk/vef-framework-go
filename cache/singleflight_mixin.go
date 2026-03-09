package cache

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// LoaderFunc defines a function that loads a value for a given key when cache miss happens.
type LoaderFunc[T any] func(ctx context.Context) (T, error)

// GetFunc reads a value from cache for the provided key.
type GetFunc[T any] func(context.Context, string) (T, bool)

// SetFunc writes a value into cache for the provided key and optional Ttl.
type SetFunc[T any] func(context.Context, string, T, ...time.Duration) error

// SingleflightMixin provides reusable singleflight-backed GetOrLoad logic
// that cache implementations can embed to prevent cache stampede.
//
// Usage:
//
//	type MyCache[T any] struct {
//	    // ... other fields ...
//	    loadMixin SingleflightMixin[T]
//	}
//
//	func (c *MyCache[T]) GetOrLoad(ctx context.Context, key string, loader LoaderFunc[T], ttl ...time.Duration) (T, error) {
//	    return c.loadMixin.GetOrLoad(ctx, key, loader, ttl, c.Get, c.Set)
//	}
type SingleflightMixin[T any] struct {
	group singleflight.Group
}

// GetOrLoad retrieves a value from cache or loads it using the provided loader,
// coordinating concurrent requests for the same cacheKey to prevent cache stampede.
func (m *SingleflightMixin[T]) GetOrLoad(
	ctx context.Context,
	cacheKey string,
	loader LoaderFunc[T],
	ttl []time.Duration,
	getFn GetFunc[T],
	setFn SetFunc[T],
) (value T, _ error) {
	if loader == nil {
		return value, ErrLoaderRequired
	}

	// First check: Try to get from cache
	if value, found := getFn(ctx, cacheKey); found {
		return value, nil
	}

	// Use singleflight to coordinate concurrent requests
	result, err, _ := m.group.Do(cacheKey, func() (any, error) {
		// Double-check: Another goroutine might have loaded it while we waited
		if value, found := getFn(ctx, cacheKey); found {
			return value, nil
		}

		// Load the value
		value, loadErr := loader(ctx)
		if loadErr != nil {
			return value, loadErr
		}

		// Store in cache
		if setErr := setFn(ctx, cacheKey, value, ttl...); setErr != nil {
			return value, setErr
		}

		return value, nil
	})
	if err != nil {
		return value, err
	}

	value, ok := result.(T)
	if !ok {
		return value, ErrTypeAssertionFailed
	}

	return value, nil
}
