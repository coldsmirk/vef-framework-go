package middleware

import (
	"slices"

	"github.com/coldsmirk/vef-framework-go/api"
)

// Chain manages the middleware chain for API requests.
type Chain struct {
	middlewares []api.Middleware
}

// Handlers converts the chain's middlewares into a slice of fiber.Handler.
// The returned handlers maintain the execution order (sorted by Order ascending),
// ensuring that middlewares with lower Order() values execute first.
func (c *Chain) Handlers() []any {
	handlers := make([]any, len(c.middlewares))
	for i, mid := range c.middlewares {
		handlers[i] = mid.Process
	}

	return handlers
}

// NewChain creates a new middleware chain with the given middlewares.
// Middlewares are sorted by their Order() value (ascending).
func NewChain(middlewares ...api.Middleware) *Chain {
	sorted := slices.Clone(middlewares)
	slices.SortFunc(sorted, func(a, b api.Middleware) int {
		return a.Order() - b.Order()
	})

	return &Chain{
		middlewares: sorted,
	}
}
