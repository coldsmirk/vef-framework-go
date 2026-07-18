package mold

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/event"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
)

// eventTypeCodeSetChanged is the topic used to invalidate cached
// code set values.
const eventTypeCodeSetChanged = "vef.translate.code_set.changed"

// CodeSetLoaderFunc allows using a plain function as a CodeSetLoader.
type CodeSetLoaderFunc func(ctx context.Context, codeSet string) (map[string]string, error)

// Load executes the wrapped function.
func (f CodeSetLoaderFunc) Load(ctx context.Context, codeSet string) (map[string]string, error) {
	return f(ctx, codeSet)
}

// CodeSetChangedEvent is emitted whenever code set entries need to be invalidated.
type CodeSetChangedEvent struct {
	// Keys lists the affected code set identifiers. When empty, all cached code sets should be cleared.
	Keys []string `json:"keys"`
}

// EventType implements event.Event.
func (*CodeSetChangedEvent) EventType() string { return eventTypeCodeSetChanged }

// PublishCodeSetChangedEvent publishes a code set invalidation event.
// When no keys are provided, subscribers are expected to clear their entire cache.
func PublishCodeSetChangedEvent(ctx context.Context, bus event.Bus, keys ...string) error {
	return bus.Publish(ctx, &CodeSetChangedEvent{Keys: keys})
}

// CachedCodeSetResolver adds caching and event-based invalidation around a CodeSetLoader implementation.
// Underlying cache implementations already coordinate concurrent loads to prevent stampede.
type CachedCodeSetResolver struct {
	cache *cache.Invalidating[map[string]string]
}

// NewCachedCodeSetResolver constructs a caching resolver for code set lookups.
func NewCachedCodeSetResolver(
	loader CodeSetLoader,
	bus event.Bus,
) CodeSetResolver {
	if loader == nil {
		panic("NewCachedCodeSetResolver requires a non-nil CodeSetLoader, but got nil")
	}

	if bus == nil {
		panic("NewCachedCodeSetResolver requires a non-nil event.Bus, but got nil")
	}

	resolver := &CachedCodeSetResolver{
		cache: cache.NewInvalidating(
			func(ctx context.Context, codeSet string) (map[string]string, error) {
				entries, err := loader.Load(ctx, codeSet)
				if err != nil {
					return nil, err
				}

				if entries == nil {
					entries = make(map[string]string)
				}

				return entries, nil
			},
			ilogx.Named("translate:cached_code_set_resolver"),
			// Code set identifiers originate from mold tags but flow through
			// host-provided loaders; the LRU bound caps growth if a caller
			// probes with unbounded keys (evicted entries reload on demand).
			cache.WithMemMaxSize(4096),
		),
	}

	if _, err := event.SubscribeTyped[*CodeSetChangedEvent](bus, resolver.handleInvalidation); err != nil {
		panic(fmt.Errorf("subscribe code_set.changed: %w", err))
	}

	return resolver
}

// Resolve finds the display label for the provided code set/code combination.
// Returns the translated name and an error if resolution fails.
// Returns empty string without error if the code set or code is empty, or if the entry is not found.
func (r *CachedCodeSetResolver) Resolve(ctx context.Context, codeSet, code string) (string, error) {
	if codeSet == "" || code == "" {
		return "", nil
	}

	entries, err := r.cache.Get(ctx, codeSet)
	if err != nil {
		return "", fmt.Errorf("failed to load code set %q: %w", codeSet, err)
	}

	name, ok := entries[code]
	if !ok {
		return "", nil
	}

	return name, nil
}

func (r *CachedCodeSetResolver) handleInvalidation(ctx context.Context, evt *CodeSetChangedEvent, _ event.Envelope) error {
	return r.cache.Invalidate(ctx, evt.Keys...)
}
