package auth

import (
	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/api"
)

// Registry implements api.AuthStrategyRegistry using a concurrent map.
type Registry struct {
	strategies collections.ConcurrentMap[string, api.AuthStrategy]
}

// NewRegistry creates a new authentication strategy registry.
func NewRegistry(strategies ...api.AuthStrategy) api.AuthStrategyRegistry {
	registry := &Registry{
		strategies: collections.NewConcurrentHashMap[string, api.AuthStrategy](),
	}

	for _, strategy := range strategies {
		registry.Register(strategy)
	}

	return registry
}

// Register adds a strategy to the registry.
func (r *Registry) Register(strategy api.AuthStrategy) {
	r.strategies.Put(strategy.Name(), strategy)
}

// Get retrieves a strategy by name.
func (r *Registry) Get(name string) (api.AuthStrategy, bool) {
	return r.strategies.Get(name)
}

// Names returns all registered strategy names.
func (r *Registry) Names() []string {
	return r.strategies.Keys()
}
