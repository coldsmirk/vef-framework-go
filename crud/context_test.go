package crud

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryError tests query error functionality.
func TestQueryError(t *testing.T) {
	t.Run("ReturnsNilWhenNoErrorStored", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(ctx fiber.Ctx) error {
			assert.Nil(t, QueryError(ctx), "Should return nil when no error is stored")

			return nil
		})

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should execute test request without error")
		assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
	})

	t.Run("ReturnsStoredError", func(t *testing.T) {
		app := fiber.New()
		expectedErr := errors.New("test query error")
		app.Get("/test", func(ctx fiber.Ctx) error {
			SetQueryError(ctx, expectedErr)
			got := QueryError(ctx)
			assert.Equal(t, expectedErr, got, "Should return the stored error")

			return nil
		})

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should execute test request without error")
		assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
	})

	t.Run("ReturnsNilForNonErrorType", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(ctx fiber.Ctx) error {
			// Store a non-error value
			ctx.Locals(keyQueryError, "not-an-error")
			result := QueryError(ctx)
			assert.Nil(t, result, "Should return nil for non-error type")

			return nil
		})

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should execute test request without error")
		assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
	})
}
