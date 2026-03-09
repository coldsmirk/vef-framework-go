package cache

import "time"

// MemoryOption configures the behavior of NewMemory caches.
type MemoryOption func(*memoryConfig)

// A value <= 0 disables size limits.
func WithMemMaxSize(size int64) MemoryOption {
	return func(cfg *memoryConfig) {
		cfg.maxSize = size
	}
}

func WithMemDefaultTTL(ttl time.Duration) MemoryOption {
	return func(cfg *memoryConfig) {
		cfg.defaultTTL = ttl
	}
}

// WithMemEvictionPolicy selects the eviction strategy used when max size is enforced.
func WithMemEvictionPolicy(policy EvictionPolicy) MemoryOption {
	return func(cfg *memoryConfig) {
		cfg.evictionPolicy = policy
	}
}

func WithMemGCInterval(interval time.Duration) MemoryOption {
	return func(cfg *memoryConfig) {
		cfg.gcInterval = interval
	}
}

// RedisOption configures Redis-backed cache instances.
type RedisOption func(*redisConfig)

func WithRdsDefaultTTL(ttl time.Duration) RedisOption {
	return func(cfg *redisConfig) {
		cfg.defaultTTL = ttl
	}
}
