package store

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/cron"
)

// Registry indexes the registered JobHandlers by job name. It is immutable
// after construction; the engine claims only jobs this node can execute, so
// heterogeneous deployments (rolling updates, split job fleets) stay safe.
type Registry struct {
	handlers map[string]cron.JobHandler
	names    []string
}

// NewRegistry builds the registry from the DI-collected handlers. Duplicate
// job names fail construction — and with it application start-up.
func NewRegistry(handlers []cron.JobHandler) (*Registry, error) {
	registry := &Registry{handlers: make(map[string]cron.JobHandler, len(handlers))}

	for _, handler := range handlers {
		name := handler.Name()
		if name == "" {
			return nil, ErrJobHandlerNameEmpty
		}

		if utf8.RuneCountInString(name) > maxJobNameLength {
			return nil, fmt.Errorf("%w: %q", ErrJobHandlerNameTooLong, name)
		}

		if _, exists := registry.handlers[name]; exists {
			return nil, fmt.Errorf("%w: %q", ErrJobHandlerDuplicate, name)
		}

		registry.handlers[name] = handler
		registry.names = append(registry.names, name)
	}

	slices.Sort(registry.names)

	return registry, nil
}

// Lookup returns the handler serving the job name.
func (r *Registry) Lookup(name string) (cron.JobHandler, bool) {
	handler, ok := r.handlers[name]

	return handler, ok
}

// Names returns the registered job names, sorted. Callers must not mutate
// the returned slice.
func (r *Registry) Names() []string {
	return r.names
}

// IsEmpty reports whether no handler is registered.
func (r *Registry) IsEmpty() bool {
	return len(r.handlers) == 0
}

// All returns every registered handler in name order.
func (r *Registry) All() []cron.JobHandler {
	handlers := make([]cron.JobHandler, 0, len(r.names))
	for _, name := range r.names {
		handlers = append(handlers, r.handlers[name])
	}

	return handlers
}
