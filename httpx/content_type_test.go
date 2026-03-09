package httpx

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsJSON tests IsJSON functionality.
func TestIsJSON(t *testing.T) {
	t.Run("ApplicationJson", func(t *testing.T) {
		app := fiber.New()

		app.Post("/json", func(c fiber.Ctx) error {
			result := IsJSON(c)
			assert.True(t, result, "Should return true for application/json")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/json", nil)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("ApplicationJsonWithCharset", func(t *testing.T) {
		app := fiber.New()

		app.Post("/json", func(c fiber.Ctx) error {
			result := IsJSON(c)
			assert.True(t, result, "Should return true for application/json with charset")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/json", nil)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("MissingContentType", func(t *testing.T) {
		app := fiber.New()

		app.Post("/json", func(c fiber.Ctx) error {
			result := IsJSON(c)
			assert.False(t, result, "Should return false when Content-Type header is missing")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/json", nil)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("NonJsonContentType", func(t *testing.T) {
		app := fiber.New()

		app.Post("/json", func(c fiber.Ctx) error {
			result := IsJSON(c)
			assert.False(t, result, "Should return false for non-JSON Content-Type")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/json", nil)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})
}

// TestIsMultipart tests is multipart functionality.
func TestIsMultipart(t *testing.T) {
	t.Run("MultipartFormData", func(t *testing.T) {
		app := fiber.New()

		app.Post("/multipart", func(c fiber.Ctx) error {
			result := IsMultipart(c)
			assert.True(t, result, "Should return true for multipart/form-data")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/multipart", nil)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEMultipartForm+"; boundary=MyBoundary")

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("NonMultipartContentType", func(t *testing.T) {
		app := fiber.New()

		app.Post("/multipart", func(c fiber.Ctx) error {
			result := IsMultipart(c)
			assert.False(t, result, "Should return false for non-multipart Content-Type")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/multipart", nil)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("MissingContentType", func(t *testing.T) {
		app := fiber.New()

		app.Post("/multipart", func(c fiber.Ctx) error {
			result := IsMultipart(c)
			assert.False(t, result, "Should return false when Content-Type header is missing")

			return c.SendStatus(fiber.StatusOK)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/multipart", nil)

		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Should equal expected value")
	})
}
