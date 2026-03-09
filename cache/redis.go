package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisConfig struct {
	defaultTTL time.Duration
}

func defaultRedisConfig() *redisConfig {
	return &redisConfig{}
}

// redisCache provides a Redis-backed implementation of Cache[T].
type redisCache[T any] struct {
	client     *redis.Client
	keyBuilder KeyBuilder
	basePrefix string
	defaultTTL time.Duration
	serializer Serializer[T]
	loadMixin  SingleflightMixin[T]
	closed     atomic.Bool
}

// newRedisCache constructs a Redis-backed cache instance.
// KeyBuilder should encapsulate the namespace/prefix for this cache instance.
func newRedisCache[T any](client *redis.Client, keyBuilder KeyBuilder, cfg *redisConfig) Cache[T] {
	if client == nil {
		panic("redis cache requires a non-nil redis client")
	}

	if keyBuilder == nil {
		keyBuilder = defaultKeyBuilder
	}

	if cfg == nil {
		cfg = defaultRedisConfig()
	}

	return &redisCache[T]{
		client:     client,
		keyBuilder: keyBuilder,
		basePrefix: keyBuilder.Build(),
		defaultTTL: cfg.defaultTTL,
		serializer: newJSONSerializer[T](),
	}
}

func (c *redisCache[T]) getExpiration(ttl []time.Duration) time.Duration {
	if len(ttl) > 0 && ttl[0] > 0 {
		return ttl[0]
	}

	if c.defaultTTL > 0 {
		return c.defaultTTL
	}

	return 0
}

func (c *redisCache[T]) buildPattern(prefix string) string {
	if prefix == "" {
		return c.basePrefix + "*"
	}

	return c.keyBuilder.Build(prefix) + "*"
}

// stripPrefix removes the basePrefix from a Redis key to return the user's original key.
func (c *redisCache[T]) stripPrefix(cacheKey string) string {
	if c.basePrefix == "" {
		return cacheKey
	}

	// Remove "basePrefix:" from the key
	prefix := c.basePrefix + ":"
	if strings.HasPrefix(cacheKey, prefix) {
		return cacheKey[len(prefix):]
	}

	return cacheKey
}

func (c *redisCache[T]) getByCacheKey(ctx context.Context, cacheKey string) (value T, _ bool) {
	if c.closed.Load() {
		return value, false
	}

	data, err := c.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key not found - normal cache miss
			return value, false
		}

		// Redis error (network, timeout, etc.) - treat as cache miss
		// to avoid cascading failures in the application layer
		return value, false
	}

	value, err = c.serializer.Deserialize(data)
	if err != nil {
		// Deserialization failed - data is corrupted or incompatible
		// Treat as cache miss and let the application reload fresh data
		return value, false
	}

	return value, true
}

func (c *redisCache[T]) setByCacheKey(ctx context.Context, cacheKey string, value T, ttl ...time.Duration) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	payload, err := c.serializer.Serialize(value)
	if err != nil {
		return err
	}

	expiration := c.getExpiration(ttl)

	if err := c.client.Set(ctx, cacheKey, payload, expiration).Err(); err != nil {
		return fmt.Errorf("redis cache set failed for key %s: %w", cacheKey, err)
	}

	return nil
}

// Get retrieves a value by key.
func (c *redisCache[T]) Get(ctx context.Context, key string) (T, bool) {
	cacheKey := c.keyBuilder.Build(key)

	return c.getByCacheKey(ctx, cacheKey)
}

// GetOrLoad retrieves a value from cache or loads it using the provided loader.
func (c *redisCache[T]) GetOrLoad(ctx context.Context, key string, loader LoaderFunc[T], ttl ...time.Duration) (T, error) {
	cacheKey := c.keyBuilder.Build(key)

	return c.loadMixin.GetOrLoad(
		ctx,
		cacheKey,
		loader,
		ttl,
		c.getByCacheKey,
		c.setByCacheKey,
	)
}

// Set stores a value with the given key and optional Ttl.
func (c *redisCache[T]) Set(ctx context.Context, key string, value T, ttl ...time.Duration) error {
	cacheKey := c.keyBuilder.Build(key)

	return c.setByCacheKey(ctx, cacheKey, value, ttl...)
}

// Contains checks if a key exists in the cache.
func (c *redisCache[T]) Contains(ctx context.Context, key string) bool {
	if c.closed.Load() {
		return false
	}

	cacheKey := c.keyBuilder.Build(key)

	exists, err := c.client.Exists(ctx, cacheKey).Result()
	if err != nil {
		return false
	}

	return exists > 0
}

// Delete removes a key from the cache.
func (c *redisCache[T]) Delete(ctx context.Context, key string) error {
	if c.closed.Load() {
		return nil
	}

	cacheKey := c.keyBuilder.Build(key)
	if err := c.client.Del(ctx, cacheKey).Err(); err != nil {
		return fmt.Errorf("redis cache delete failed for key %s: %w", cacheKey, err)
	}

	return nil
}

// Clear removes all entries managed by this cache instance.
func (c *redisCache[T]) Clear(ctx context.Context) error {
	if c.closed.Load() {
		return nil
	}

	if c.basePrefix == "" {
		return c.client.FlushDB(ctx).Err()
	}

	pattern := c.basePrefix + "*"
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis cache clear scan failed: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis cache clear delete failed: %w", err)
	}

	return nil
}

// Keys returns all keys in the cache (with prefix stripped), optionally filtered by prefix.
func (c *redisCache[T]) Keys(ctx context.Context, prefix ...string) ([]string, error) {
	if c.closed.Load() {
		return nil, nil
	}

	filter := ""
	if len(prefix) > 0 {
		filter = prefix[0]
	}

	pattern := c.buildPattern(filter)
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		cacheKey := iter.Val()
		// Strip the prefix to return user's original key
		userKey := c.stripPrefix(cacheKey)
		keys = append(keys, userKey)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("redis cache keys scan failed: %w", err)
	}

	return keys, nil
}

// ForEach iterates over all key-value pairs in the cache (with prefix stripped), optionally filtered by prefix.
func (c *redisCache[T]) ForEach(ctx context.Context, callback func(key string, value T) bool, prefix ...string) error {
	if c.closed.Load() {
		return nil
	}

	filter := ""
	if len(prefix) > 0 {
		filter = prefix[0]
	}

	pattern := c.buildPattern(filter)
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		cacheKey := iter.Val()

		data, err := c.client.Get(ctx, cacheKey).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}

			return fmt.Errorf("redis cache foreach get failed for key %s: %w", cacheKey, err)
		}

		value, err := c.serializer.Deserialize(data)
		if err != nil {
			return fmt.Errorf("redis cache foreach deserialize failed for key %s: %w", cacheKey, err)
		}

		// Strip the prefix to return user's original key
		userKey := c.stripPrefix(cacheKey)
		if !callback(userKey, value) {
			break
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis cache foreach scan failed: %w", err)
	}

	return nil
}

// Size returns the number of entries in the cache.
func (c *redisCache[T]) Size(ctx context.Context) (int64, error) {
	if c.closed.Load() {
		return 0, nil
	}

	if c.basePrefix == "" {
		return c.client.DBSize(ctx).Result()
	}

	pattern := c.basePrefix + "*"
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()

	var count int64
	for iter.Next(ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("redis cache size scan failed: %w", err)
	}

	return count, nil
}

// Close marks the cache as closed. The underlying Redis client remains managed externally.
func (c *redisCache[T]) Close() error {
	c.closed.Store(true)

	return nil
}
