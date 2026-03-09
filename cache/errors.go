package cache

import "errors"

var (
	// ErrStoreRequiresName indicates the cache store requires a name.
	ErrStoreRequiresName = errors.New("cache store requires a name")
	// ErrMemoryLimitExceeded is returned when the cache cannot accept additional entries due to size limits or unavailable eviction candidates.
	ErrMemoryLimitExceeded = errors.New("memory cache size limit exceeded")
	// ErrCacheClosed is returned when cache operations are attempted after Close has been called.
	ErrCacheClosed = errors.New("cache closed")
	// ErrLoaderRequired is returned when GetOrLoad is called without providing a loader.
	ErrLoaderRequired = errors.New("cache loader is required")
	// ErrTypeAssertionFailed is returned when singleflight type assertion fails.
	ErrTypeAssertionFailed = errors.New("singleflight: type assertion failed")
)
