package jscache

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/js"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "cache"

// lib exposes key-value state as the global "cache" object — the only channel
// scripts have for keeping state across executions, since runtimes are
// discarded after each run:
//
//	cache.set('counter', { n: 1 })          // store default TTL
//	cache.set('token', value, 60000)        // TTL in milliseconds
//	cache.get('counter')                    // → value | null
//	cache.has('counter')                    // → boolean
//	cache.delete('counter')
//
// Values round-trip through the backing store's own serialization (plain Go
// values in memory, JSON in Redis), so JSON-able values are safe everywhere.
type lib struct {
	store     cache.Cache[any]
	keyPrefix string
}

// New builds the cache library over store. The caller picks the backing store
// — cache.NewMemory for single-node state, cache.NewRedis for shared state —
// and thereby its lifecycle and serialization. See WithKeyPrefix for
// namespacing the scripts' keys.
func New(store cache.Cache[any], opts ...Option) js.Lib {
	var cfg libConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	return &lib{store: store, keyPrefix: cfg.keyPrefix}
}

func (*lib) Name() string {
	return Name
}

func (l *lib) Install(rt *js.Runtime) error {
	return rt.Set(Name, map[string]any{
		"get": func(key string) any {
			value, ok := l.store.Get(rt.Context(), l.key(key))
			if !ok {
				return nil
			}

			return value
		},
		"set": func(key string, value any, ttlMs int64) error {
			if ttlMs > 0 {
				return l.store.Set(rt.Context(), l.key(key), value, time.Duration(ttlMs)*time.Millisecond)
			}

			return l.store.Set(rt.Context(), l.key(key), value)
		},
		"has": func(key string) bool {
			return l.store.Contains(rt.Context(), l.key(key))
		},
		"delete": func(key string) error {
			return l.store.Delete(rt.Context(), l.key(key))
		},
	})
}

// key applies the configured namespace prefix.
func (l *lib) key(key string) string {
	return l.keyPrefix + key
}
