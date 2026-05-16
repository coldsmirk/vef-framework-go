package mold

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/event"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// eventTypeDataDictChanged is the topic used to invalidate cached
// dictionary values.
const eventTypeDataDictChanged = "vef.translate.data_dict.changed"

// DataDictLoaderFunc allows using a plain function as a DataDictLoader.
type DataDictLoaderFunc func(ctx context.Context, key string) (map[string]string, error)

// Load executes the wrapped function.
func (f DataDictLoaderFunc) Load(ctx context.Context, key string) (map[string]string, error) {
	return f(ctx, key)
}

// DataDictChangedEvent is emitted whenever dictionary entries need to be invalidated.
type DataDictChangedEvent struct {
	// Keys lists the affected dictionary keys. When empty, all cached dictionaries should be cleared.
	Keys []string `json:"keys"`
}

// EventType implements event.Event.
func (*DataDictChangedEvent) EventType() string { return eventTypeDataDictChanged }

// PublishDataDictChangedEvent publishes a dictionary invalidation event.
// When no keys are provided, subscribers are expected to clear their entire cache.
func PublishDataDictChangedEvent(ctx context.Context, bus event.Bus, keys ...string) error {
	return bus.Publish(ctx, &DataDictChangedEvent{Keys: keys})
}

// CachedDataDictResolver adds caching and event-based invalidation around a DataDictLoader implementation.
// Underlying cache implementations already coordinate concurrent loads to prevent stampede.
type CachedDataDictResolver struct {
	loader    DataDictLoader
	dictCache cache.Cache[map[string]string]
	logger    logx.Logger
}

// NewCachedDataDictResolver constructs a caching resolver for dictionary lookups.
func NewCachedDataDictResolver(
	loader DataDictLoader,
	bus event.Bus,
) DataDictResolver {
	if loader == nil {
		panic("NewCachedDataDictResolver requires a non-nil DataDictLoader, but got nil")
	}

	if bus == nil {
		panic("NewCachedDataDictResolver requires a non-nil event.Bus, but got nil")
	}

	resolver := &CachedDataDictResolver{
		loader:    loader,
		dictCache: cache.NewMemory[map[string]string](),
		logger:    ilogx.Named("translate:cached_data_dict_resolver"),
	}

	if _, err := event.SubscribeTyped[*DataDictChangedEvent](bus, resolver.handleInvalidation); err != nil {
		panic(fmt.Errorf("subscribe data_dict.changed: %w", err))
	}

	return resolver
}

// Resolve finds the dictionary display name for the provided key/code combination.
// Returns the translated name and an error if resolution fails.
// Returns empty string without error if the key or code is empty, or if the entry is not found.
func (r *CachedDataDictResolver) Resolve(ctx context.Context, key, code string) (string, error) {
	if key == "" || code == "" {
		return "", nil
	}

	entries, err := r.getEntries(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to load dictionary %q: %w", key, err)
	}

	name, ok := entries[code]
	if !ok {
		return "", nil
	}

	return name, nil
}

func (r *CachedDataDictResolver) getEntries(ctx context.Context, key string) (map[string]string, error) {
	entries, err := r.dictCache.GetOrLoad(ctx, key, func(ctx context.Context) (map[string]string, error) {
		// Load from underlying loader
		entries, err := r.loader.Load(ctx, key)
		if err != nil {
			return nil, err
		}

		if entries == nil {
			entries = make(map[string]string)
		}

		return entries, nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (r *CachedDataDictResolver) handleInvalidation(ctx context.Context, evt *DataDictChangedEvent, _ event.Envelope) error {
	if len(evt.Keys) == 0 {
		if err := r.dictCache.Clear(ctx); err != nil {
			r.logger.Errorf("Failed to clear dictionary cache: %v", err)
			return err
		}
		r.logger.Info("Cleared all dictionary cache entries")
		return nil
	}

	for _, dictKey := range evt.Keys {
		if err := r.dictCache.Delete(ctx, dictKey); err != nil {
			r.logger.Errorf("Failed to delete cache for dictionary %q: %v", dictKey, err)
			return err
		}
		r.logger.Infof("Cleared cache for dictionary %q", dictKey)
	}
	return nil
}
