package definition

import (
	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/internal/integration/lru"
	"github.com/coldsmirk/vef-framework-go/js"
)

// programCacheCapacity bounds each compiled-program cache; scripts beyond it
// evict least recently used and recompile on next use.
const programCacheCapacity = 256

// ProgramCache caches compiled scripts keyed by content hash, so editing a
// script invalidates its entry implicitly and unchanged scripts never
// recompile. Each script wrapper owns an instance: adapter execution,
// script-scheme signing and verification, and the two envelope directions.
type ProgramCache struct {
	compile func(string) (*js.Program, error)
	cache   *lru.Synced[*js.Program]
}

// NewProgramCache creates an empty compiled-program cache backed by compile.
func NewProgramCache(compile func(string) (*js.Program, error)) *ProgramCache {
	return &ProgramCache{compile: compile, cache: lru.NewSynced[*js.Program](programCacheCapacity)}
}

// Get returns the compiled program for script, compiling and caching it on
// first sight.
func (c *ProgramCache) Get(script string) (*js.Program, error) {
	return c.cache.GetOrBuild(hashx.SHA256(script), func() (*js.Program, error) {
		return c.compile(script)
	})
}
