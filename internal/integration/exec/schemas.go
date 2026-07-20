package exec

import (
	"encoding/json"

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
	cache *lru.Synced[*jsonschema.Resolved]
}

func newSchemaCache() *schemaCache {
	return &schemaCache{cache: lru.NewSynced[*jsonschema.Resolved](schemaCacheCapacity)}
}

// Get returns the resolved schema for raw, compiling and caching it on first
// sight.
func (c *schemaCache) Get(raw json.RawMessage) (*jsonschema.Resolved, error) {
	return c.cache.GetOrBuild(hashx.SHA256Bytes(raw), func() (*jsonschema.Resolved, error) {
		return definition.CompileSchema(raw)
	})
}
