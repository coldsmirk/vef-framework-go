package exec

import (
	"encoding/json"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
)

// schemaCacheCapacity bounds the resolved-schema cache.
const schemaCacheCapacity = 256

// schemaCache caches resolved contract schemas keyed by content hash,
// mirroring the program cache: editing a schema implicitly invalidates it.
type schemaCache struct {
	mu  sync.Mutex
	lru *lruCache[*jsonschema.Resolved]
}

func newSchemaCache() *schemaCache {
	return &schemaCache{lru: newLRUCache[*jsonschema.Resolved](schemaCacheCapacity)}
}

// Get returns the resolved schema for raw, compiling and caching it on first
// sight.
func (c *schemaCache) Get(raw json.RawMessage) (*jsonschema.Resolved, error) {
	key := hashx.SHA256Bytes(raw)

	c.mu.Lock()
	defer c.mu.Unlock()

	if resolved, ok := c.lru.Get(key); ok {
		return resolved, nil
	}

	resolved, err := service.CompileSchema(raw)
	if err != nil {
		return nil, err
	}

	c.lru.Put(key, resolved)

	return resolved, nil
}
