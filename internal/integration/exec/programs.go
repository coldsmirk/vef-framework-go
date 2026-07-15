package exec

import (
	"sync"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/js"
)

// programCacheCapacity bounds the compiled-program cache; scripts beyond it
// evict least recently used and recompile on next use.
const programCacheCapacity = 256

// programCache caches compiled adapter programs keyed by script content
// hash, so editing a script invalidates its entry implicitly and unchanged
// scripts never recompile.
type programCache struct {
	mu  sync.Mutex
	lru *lruCache[*js.Program]
}

func newProgramCache() *programCache {
	return &programCache{lru: newLRUCache[*js.Program](programCacheCapacity)}
}

// Get returns the compiled program for script, compiling and caching it on
// first sight.
func (c *programCache) Get(script string) (*js.Program, error) {
	key := hashx.SHA256(script)

	c.mu.Lock()
	defer c.mu.Unlock()

	if program, ok := c.lru.Get(key); ok {
		return program, nil
	}

	program, err := service.CompileScript(script)
	if err != nil {
		return nil, err
	}

	c.lru.Put(key, program)

	return program, nil
}
