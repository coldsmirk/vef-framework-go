package cache

import (
	"context"
	"io"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache defines the interface for a generic key-value cache.
type Cache[T any] interface {
	io.Closer

	// Get retrieves a value by key. Returns the value and true if found, zero value and false if not found.
	Get(ctx context.Context, key string) (T, bool)
	// GetOrLoad retrieves a value by key or computes it using the provided loader when missing.
	// Implementations must ensure concurrent calls for the same key only trigger a single loader execution.
	GetOrLoad(ctx context.Context, key string, loader LoaderFunc[T], ttl ...time.Duration) (T, error)
	// Set stores a value with the given key. If ttl is provided and > 0, the entry will expire after the duration.
	Set(ctx context.Context, key string, value T, ttl ...time.Duration) error
	// Contains checks if a key exists in the cache.
	Contains(ctx context.Context, key string) bool
	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error
	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error
	// Keys returns all keys in the cache, optionally filtered by prefix.
	Keys(ctx context.Context, prefix ...string) ([]string, error)
	// ForEach iterates over all key-value pairs in the cache, optionally filtered by prefix.
	// The iteration stops if the callback returns false.
	ForEach(ctx context.Context, callback func(key string, value T) bool, prefix ...string) error
	// Size returns the number of entries in the cache.
	Size(ctx context.Context) (int64, error)
}

// Store defines the interface for the underlying cache storage implementation.
// This interface works with raw bytes and can be implemented by different backends
// like Badger, Redis, etc. It's designed to be injected as a singleton via fx.
type Store interface {
	// Name returns the name of the store.
	Name() string
	// Get retrieves raw bytes by key. Returns the data and true if found, nil and false if not found.
	Get(ctx context.Context, key string) ([]byte, bool)
	// Set stores raw bytes with the given key. If ttl is provided and > 0, the entry will expire after the duration.
	Set(ctx context.Context, key string, data []byte, ttl ...time.Duration) error
	// Contains checks if a key exists in the cache.
	Contains(ctx context.Context, key string) bool
	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error
	// Clear removes all entries from the cache.
	Clear(ctx context.Context, prefix string) error
	// Keys returns all keys in the cache, optionally filtered by prefix.
	Keys(ctx context.Context, prefix string) ([]string, error)
	// ForEach iterates over all key-value pairs in the cache, optionally filtered by prefix.
	// The iteration stops if the callback returns false.
	ForEach(ctx context.Context, prefix string, callback func(key string, data []byte) bool) error
	// Size returns the number of entries in the cache.
	Size(ctx context.Context, prefix string) (int64, error)
	// Close closes the cache and releases any resources.
	Close(ctx context.Context) error
}

const (
	cacheKeyPrefix = "vef" + ":" + "cache"
)

// NewMemory constructs an in-memory cache using functional options.
func NewMemory[T any](opts ...MemoryOption) Cache[T] {
	cfg := defaultMemoryConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return newMemoryCache[T](cfg)
}

// NewRedis constructs a Redis-backed cache with the given namespace.
// The namespace must be non-empty and is used to isolate keys.
func NewRedis[T any](client *redis.Client, namespace string, opts ...RedisOption) Cache[T] {
	if client == nil {
		panic("redis cache requires a non-nil redis client")
	}

	if namespace == "" {
		panic("cache.NewRedis requires a non-empty namespace")
	}

	cfg := defaultRedisConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	prefix := defaultKeyBuilder.Build(cacheKeyPrefix, namespace)

	return newRedisCache[T](client, NewPrefixKeyBuilder(prefix), cfg)
}
