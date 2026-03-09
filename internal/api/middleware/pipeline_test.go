package middleware

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
)

// MockMiddleware is a test implementation of api.Middleware.
type MockMiddleware struct {
	name  string
	order int
}

func (m *MockMiddleware) Name() string              { return m.name }
func (m *MockMiddleware) Order() int                { return m.order }
func (*MockMiddleware) Process(ctx fiber.Ctx) error { return ctx.Next() }

var _ api.Middleware = (*MockMiddleware)(nil)

// TestNewChain tests new chain scenarios.
func TestNewChain(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		chain := NewChain()
		assert.NotNil(t, chain, "Chain should not be nil")
		assert.Empty(t, chain.Handlers(), "Handlers should be empty")
	})

	t.Run("Single", func(t *testing.T) {
		chain := NewChain(&MockMiddleware{name: "test", order: 1})
		handlers := chain.Handlers()

		assert.Len(t, handlers, 1, "Should have exactly 1 handler")
		assert.NotNil(t, handlers[0], "Handler should not be nil")
	})

	t.Run("SortsByOrder", func(t *testing.T) {
		high := &MockMiddleware{name: "high", order: 100}
		low := &MockMiddleware{name: "low", order: -100}
		mid := &MockMiddleware{name: "mid", order: 0}

		chain := NewChain(high, low, mid)

		// Verify internal ordering: low(-100) < mid(0) < high(100)
		assert.Equal(t, "low", chain.middlewares[0].Name(), "First middleware should be low")
		assert.Equal(t, "mid", chain.middlewares[1].Name(), "Second middleware should be mid")
		assert.Equal(t, "high", chain.middlewares[2].Name(), "Third middleware should be high")
	})

	t.Run("DoesNotMutateInput", func(t *testing.T) {
		mids := []api.Middleware{
			&MockMiddleware{name: "b", order: 2},
			&MockMiddleware{name: "a", order: 1},
		}

		NewChain(mids...)

		// Original slice should be unmodified
		assert.Equal(t, "b", mids[0].Name(), "Original first element should be unchanged")
		assert.Equal(t, "a", mids[1].Name(), "Original second element should be unchanged")
	})
}

// TestChainHandlers tests chain handlers scenarios.
func TestChainHandlers(t *testing.T) {
	t.Run("ReturnsCorrectCount", func(t *testing.T) {
		chain := NewChain(
			&MockMiddleware{name: "a", order: 1},
			&MockMiddleware{name: "b", order: 2},
		)

		assert.Len(t, chain.Handlers(), 2, "Should have exactly 2 handlers")
	})

	t.Run("HandlerTypesAreFiberHandler", func(t *testing.T) {
		chain := NewChain(&MockMiddleware{name: "test", order: 1})

		for i, h := range chain.Handlers() {
			_, ok := h.(func(fiber.Ctx) error)
			assert.True(t, ok, "Handler %d should be func(fiber.Ctx) error", i)
		}
	})

	t.Run("TypicalMiddlewareOrdering", func(t *testing.T) {
		auth := &MockMiddleware{name: "auth", order: -100}
		contextual := &MockMiddleware{name: "contextual", order: -90}
		dataPermission := &MockMiddleware{name: "data_permission", order: -80}
		rateLimit := &MockMiddleware{name: "ratelimit", order: -70}
		audit := &MockMiddleware{name: "audit", order: -60}

		// Pass in shuffled order
		chain := NewChain(audit, auth, rateLimit, contextual, dataPermission)

		assert.Equal(t, "auth", chain.middlewares[0].Name(), "First should be auth")
		assert.Equal(t, "contextual", chain.middlewares[1].Name(), "Second should be contextual")
		assert.Equal(t, "data_permission", chain.middlewares[2].Name(), "Third should be data_permission")
		assert.Equal(t, "ratelimit", chain.middlewares[3].Name(), "Fourth should be ratelimit")
		assert.Equal(t, "audit", chain.middlewares[4].Name(), "Fifth should be audit")
	})
}
