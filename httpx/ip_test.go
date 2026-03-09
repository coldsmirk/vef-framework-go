package httpx

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetIP tests GetIP functionality.
func TestGetIP(t *testing.T) {
	t.Run("XForwardedForHeader", func(t *testing.T) {
		app := fiber.New()
		forwardedIP := "192.168.1.100"

		app.Get("/test", func(c fiber.Ctx) error {
			ip := GetIP(c)
			assert.Equal(t, forwardedIP, ip, "Should use X-Forwarded-For header")

			return c.SendString(ip)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", forwardedIP)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, 200, resp.StatusCode, "Should equal expected value")
	})

	t.Run("FallbackToDirectIP", func(t *testing.T) {
		app := fiber.New()

		app.Get("/test", func(c fiber.Ctx) error {
			ip := GetIP(c)
			assert.NotEmpty(t, ip, "Should return direct IP when X-Forwarded-For is not present")

			return c.SendString(ip)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, 200, resp.StatusCode, "Should equal expected value")
	})

	t.Run("XForwardedForOverridesDirectIP", func(t *testing.T) {
		app := fiber.New()
		forwardedIP := "10.0.0.1"

		app.Get("/test", func(c fiber.Ctx) error {
			ip := GetIP(c)
			assert.Equal(t, forwardedIP, ip, "Should use X-Forwarded-For over direct IP")

			return c.SendString(ip)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", forwardedIP)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, 200, resp.StatusCode, "Should equal expected value")
	})

	t.Run("EmptyXForwardedForHeader", func(t *testing.T) {
		app := fiber.New()

		app.Get("/test", func(c fiber.Ctx) error {
			ip := GetIP(c)
			assert.NotEmpty(t, ip, "Should fall back to direct IP when header is empty")

			return c.SendString(ip)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "")
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, 200, resp.StatusCode, "Should equal expected value")
	})

	t.Run("MultipleIPsInXForwardedFor", func(t *testing.T) {
		app := fiber.New()
		forwardedIPs := "203.0.113.195, 70.41.3.18, 150.172.238.178"

		app.Get("/test", func(c fiber.Ctx) error {
			ip := GetIP(c)
			assert.Equal(t, forwardedIPs, ip, "Should return the full X-Forwarded-For value")

			return c.SendString(ip)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", forwardedIPs)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")
		require.Equal(t, 200, resp.StatusCode, "Should equal expected value")
	})
}
