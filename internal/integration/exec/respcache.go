package exec

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/hashx"
)

// responseCache holds validated invocation outputs for call sites that opt
// in via WithCache. Per node and in memory: a cache entry is an
// optimization, never a correctness dependency.
type responseCache struct {
	store cache.Cache[any]
}

func newResponseCache() *responseCache {
	return &responseCache{store: cache.NewMemory[any]()}
}

// Get returns the cached output for key.
func (c *responseCache) Get(ctx context.Context, key string) (any, bool) {
	return c.store.Get(ctx, key)
}

// Set stores output under key for ttl.
func (c *responseCache) Set(ctx context.Context, key string, output any, ttl time.Duration) {
	_ = c.store.Set(ctx, key, output, ttl)
}

// responseCacheKey derives the cache key from the target and the canonical
// input. Map key order is stable under json.Marshal, so equal inputs yield
// equal keys.
func responseCacheKey(system, contract string, input any) string {
	payload, _ := json.Marshal([]any{system, contract, input})

	return "itg:res:" + hashx.SHA256Bytes(payload)
}
