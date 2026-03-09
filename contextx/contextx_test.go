package contextx

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// --- Test helpers ---

func RunInFiber(t *testing.T, fn func(t *testing.T, ctx fiber.Ctx)) {
	t.Helper()

	app := fiber.New()
	app.Get("/test", func(ctx fiber.Ctx) error {
		fn(t, ctx)

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "Should execute fiber test request without error")
	assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
}

type MockLogger struct{ log.Logger }

type MockDB struct{ orm.DB }

type MockDataPermApplier struct{ security.DataPermissionApplier }

// --- Tests ---

// TestRequestID tests RequestID and SetRequestID for standard and fiber contexts.
func TestRequestID(t *testing.T) {
	t.Run("ReturnsEmptyStringFromEmptyContext", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", RequestID(ctx), "Should return empty string when no request ID is stored")
	})

	t.Run("ReturnsStoredValue", func(t *testing.T) {
		ctx := SetRequestID(context.Background(), "req-123")
		assert.Equal(t, "req-123", RequestID(ctx), "Should return the stored request ID")
	})

	t.Run("ReturnsEmptyStringForWrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), KeyRequestID, 12345)
		assert.Equal(t, "", RequestID(ctx), "Should return empty string when stored value is not a string")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			SetRequestID(ctx, "fiber-req-456")
			assert.Equal(t, "fiber-req-456", RequestID(ctx), "Should return stored request ID from fiber context")
		})
	})
}

// TestPrincipal tests Principal and SetPrincipal for standard and fiber contexts.
func TestPrincipal(t *testing.T) {
	t.Run("ReturnsNilFromEmptyContext", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, Principal(ctx), "Should return nil when no principal is stored")
	})

	t.Run("ReturnsStoredValue", func(t *testing.T) {
		expected := &security.Principal{ID: "user-1", Name: "Alice"}
		ctx := SetPrincipal(context.Background(), expected)
		assert.Same(t, expected, Principal(ctx), "Should return the same principal instance")
	})

	t.Run("ReturnsNilForWrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), KeyPrincipal, "not-a-principal")
		assert.Nil(t, Principal(ctx), "Should return nil when stored value is not a *Principal")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			expected := &security.Principal{ID: "fiber-user"}
			SetPrincipal(ctx, expected)
			assert.Same(t, expected, Principal(ctx), "Should return the same principal from fiber context")
		})
	})
}

// TestLogger tests Logger and SetLogger including fallback logic with typed nil handling.
func TestLogger(t *testing.T) {
	t.Run("ReturnsLoggerFromContext", func(t *testing.T) {
		expected := &MockLogger{}
		ctx := SetLogger(context.Background(), expected)
		assert.Same(t, expected, Logger(ctx), "Should return the stored logger")
	})

	t.Run("ReturnsNilWithNoFallback", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, Logger(ctx), "Should return nil when no logger and no fallback")
	})

	t.Run("ReturnsFirstValidFallback", func(t *testing.T) {
		ctx := context.Background()
		fb := &MockLogger{}
		assert.Same(t, fb, Logger(ctx, fb), "Should return the fallback logger")
	})

	t.Run("SkipsNilFallbacks", func(t *testing.T) {
		ctx := context.Background()
		valid := &MockLogger{}
		assert.Same(t, valid, Logger(ctx, nil, valid), "Should skip nil fallback and return the first valid one")
	})

	t.Run("SkipsTypedNilFallback", func(t *testing.T) {
		ctx := context.Background()

		var typedNil *MockLogger

		valid := &MockLogger{}
		assert.Same(t, valid, Logger(ctx, typedNil, valid), "Should skip typed nil fallback and return the first valid one")
	})

	t.Run("ReturnsNilWhenAllFallbacksNil", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, Logger(ctx, nil, nil), "Should return nil when all fallbacks are nil")
	})

	t.Run("ReturnsNilWhenAllFallbacksTypedNil", func(t *testing.T) {
		ctx := context.Background()

		var typedNil1, typedNil2 *MockLogger
		assert.Nil(t, Logger(ctx, typedNil1, typedNil2), "Should return nil when all fallbacks are typed nil")
	})

	t.Run("ContextValueTakesPrecedenceOverFallback", func(t *testing.T) {
		stored := &MockLogger{}
		fb := &MockLogger{}
		ctx := SetLogger(context.Background(), stored)
		assert.Same(t, stored, Logger(ctx, fb), "Should return context logger, not fallback")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			expected := &MockLogger{}
			SetLogger(ctx, expected)
			assert.Same(t, expected, Logger(ctx), "Should return stored logger from fiber context")
		})
	})
}

// TestDB tests DB and SetDB including fallback logic with typed nil handling.
func TestDB(t *testing.T) {
	t.Run("ReturnsDBFromContext", func(t *testing.T) {
		expected := &MockDB{}
		ctx := SetDB(context.Background(), expected)
		assert.Same(t, expected, DB(ctx), "Should return the stored DB")
	})

	t.Run("ReturnsNilWithNoFallback", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, DB(ctx), "Should return nil when no DB and no fallback")
	})

	t.Run("ReturnsFirstValidFallback", func(t *testing.T) {
		ctx := context.Background()
		fb := &MockDB{}
		assert.Same(t, fb, DB(ctx, fb), "Should return the fallback DB")
	})

	t.Run("SkipsNilFallbacks", func(t *testing.T) {
		ctx := context.Background()
		valid := &MockDB{}
		assert.Same(t, valid, DB(ctx, nil, valid), "Should skip nil fallback and return the first valid one")
	})

	t.Run("SkipsTypedNilFallback", func(t *testing.T) {
		ctx := context.Background()

		var typedNil *MockDB

		valid := &MockDB{}
		assert.Same(t, valid, DB(ctx, typedNil, valid), "Should skip typed nil fallback and return the first valid one")
	})

	t.Run("ReturnsNilWhenAllFallbacksNil", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, DB(ctx, nil, nil), "Should return nil when all fallbacks are nil")
	})

	t.Run("ReturnsNilWhenAllFallbacksTypedNil", func(t *testing.T) {
		ctx := context.Background()

		var typedNil1, typedNil2 *MockDB
		assert.Nil(t, DB(ctx, typedNil1, typedNil2), "Should return nil when all fallbacks are typed nil")
	})

	t.Run("ContextValueTakesPrecedenceOverFallback", func(t *testing.T) {
		stored := &MockDB{}
		fb := &MockDB{}
		ctx := SetDB(context.Background(), stored)
		assert.Same(t, stored, DB(ctx, fb), "Should return context DB, not fallback")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			expected := &MockDB{}
			SetDB(ctx, expected)
			assert.Same(t, expected, DB(ctx), "Should return stored DB from fiber context")
		})
	})
}

// TestDataPermApplier tests DataPermApplier and SetDataPermApplier for standard and fiber contexts.
func TestDataPermApplier(t *testing.T) {
	t.Run("ReturnsNilFromEmptyContext", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, DataPermApplier(ctx), "Should return nil when no applier is stored")
	})

	t.Run("ReturnsStoredValue", func(t *testing.T) {
		expected := &MockDataPermApplier{}
		ctx := SetDataPermApplier(context.Background(), expected)
		assert.Same(t, expected, DataPermApplier(ctx), "Should return the stored data permission applier")
	})

	t.Run("ReturnsNilForWrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), KeyDataPermApplier, "not-an-applier")
		assert.Nil(t, DataPermApplier(ctx), "Should return nil when stored value is not a DataPermissionApplier")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			expected := &MockDataPermApplier{}
			SetDataPermApplier(ctx, expected)
			assert.Same(t, expected, DataPermApplier(ctx), "Should return stored applier from fiber context")
		})
	})
}

// TestRequestIP tests RequestIP and SetRequestIP for standard and fiber contexts.
func TestRequestIP(t *testing.T) {
	t.Run("ReturnsEmptyStringFromEmptyContext", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", RequestIP(ctx), "Should return empty string when no IP is stored")
	})

	t.Run("ReturnsStoredValue", func(t *testing.T) {
		ctx := SetRequestIP(context.Background(), "192.168.1.1")
		assert.Equal(t, "192.168.1.1", RequestIP(ctx), "Should return the stored IP")
	})

	t.Run("ReturnsEmptyStringForWrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), KeyRequestIP, 12345)
		assert.Equal(t, "", RequestIP(ctx), "Should return empty string when stored value is not a string")
	})

	t.Run("WorksWithFiberContext", func(t *testing.T) {
		RunInFiber(t, func(t *testing.T, ctx fiber.Ctx) {
			SetRequestIP(ctx, "10.0.0.1")
			assert.Equal(t, "10.0.0.1", RequestIP(ctx), "Should return stored IP from fiber context")
		})
	})
}
