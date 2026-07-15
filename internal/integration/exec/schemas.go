package exec

import (
	"encoding/json"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/internal/integration/lru"
)

// schemaCacheCapacity bounds the resolved-schema cache.
const schemaCacheCapacity = 256

// schemaCache caches resolved contract schemas keyed by content hash,
// mirroring the program cache: editing a schema implicitly invalidates it.
type schemaCache struct {
	mu    sync.Mutex
	cache *lru.Cache[*jsonschema.Resolved]
}

func newSchemaCache() *schemaCache {
	return &schemaCache{cache: lru.New[*jsonschema.Resolved](schemaCacheCapacity)}
}

// Get returns the resolved schema for raw, compiling and caching it on first
// sight.
func (c *schemaCache) Get(raw json.RawMessage) (*jsonschema.Resolved, error) {
	key := hashx.SHA256Bytes(raw)

	c.mu.Lock()
	defer c.mu.Unlock()

	if resolved, ok := c.cache.Get(key); ok {
		return resolved, nil
	}

	resolved, err := definition.CompileSchema(raw)
	if err != nil {
		return nil, err
	}

	c.cache.Put(key, resolved)

	return resolved, nil
}
