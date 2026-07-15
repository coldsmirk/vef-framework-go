package jscache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jscache"
)

// newCacheRuntime builds a bare runtime with the cache library enabled over
// the given store.
func newCacheRuntime(t *testing.T, store cache.Cache[any], opts ...jscache.Option) *js.Runtime {
	t.Helper()

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jscache.New(store, opts...)))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jscache.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	return rt
}

// TestCache tests the script-facing key-value API.
func TestCache(t *testing.T) {
	t.Run("SetGetRoundTrip", func(t *testing.T) {
		rt := newCacheRuntime(t, cache.NewMemory[any]())

		result, err := rt.RunString(t.Context(), `
			cache.set('user', { name: 'alice', tags: ['a', 'b'] });
			const user = cache.get('user');
			user.name + ':' + user.tags.join(',')
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "alice:a,b", result.String(), "Structured values should round-trip through the cache")
	})

	t.Run("MissingReturnsNull", func(t *testing.T) {
		rt := newCacheRuntime(t, cache.NewMemory[any]())

		result, err := rt.RunString(t.Context(), `cache.get('missing') === null`)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "A missing key should yield null")
	})

	t.Run("HasAndDelete", func(t *testing.T) {
		rt := newCacheRuntime(t, cache.NewMemory[any]())

		result, err := rt.RunString(t.Context(), `
			cache.set('k', 1);
			const before = cache.has('k');
			cache.delete('k');
			({ before, after: cache.has('k') })
		`)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.True(t, obj.Get("before").ToBoolean(), "Key should exist after set")
		assert.False(t, obj.Get("after").ToBoolean(), "Key should be gone after delete")
	})

	t.Run("TTLExpires", func(t *testing.T) {
		rt := newCacheRuntime(t, cache.NewMemory[any]())

		_, err := rt.RunString(t.Context(), `cache.set('t', 'v', 30)`)
		require.NoError(t, err, "Script should execute successfully")

		time.Sleep(100 * time.Millisecond)

		result, err := rt.RunString(t.Context(), `cache.get('t') === null`)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "An expired entry should yield null")
	})

	t.Run("KeyPrefix", func(t *testing.T) {
		store := cache.NewMemory[any]()
		rt := newCacheRuntime(t, store, jscache.WithKeyPrefix("js:"))

		_, err := rt.RunString(t.Context(), `cache.set('k', 'v')`)
		require.NoError(t, err, "Script should execute successfully")

		_, bare := store.Get(t.Context(), "k")
		assert.False(t, bare, "The bare key should not exist in the store")

		value, prefixed := store.Get(t.Context(), "js:k")
		require.True(t, prefixed, "The prefixed key should exist in the store")
		assert.Equal(t, "v", value, "The stored value should match")
	})
}
