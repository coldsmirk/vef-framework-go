package fiberx

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetIP verifies that GetIP resolves the client IP via Fiber and never trusts
// a raw, client-supplied X-Forwarded-For unless a trusted proxy is configured.
func TestGetIP(t *testing.T) {
	t.Run("IgnoresUntrustedXForwardedFor", func(t *testing.T) {
		app := fiber.New()
		spoofed := "10.0.0.1"

		var got string
		app.Get("/test", func(c fiber.Ctx) error {
			got = GetIP(c)

			return c.SendString(got)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", spoofed)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		require.Equal(t, 200, resp.StatusCode, "response status should be OK")
		assert.NotEqual(t, spoofed, got, "a client-supplied X-Forwarded-For must be ignored without a trusted proxy")
	})

	t.Run("DirectIPWhenNoHeader", func(t *testing.T) {
		app := fiber.New()

		var got string
		app.Get("/test", func(c fiber.Ctx) error {
			got = GetIP(c)

			return c.SendString(got)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		require.Equal(t, 200, resp.StatusCode, "response status should be OK")
		assert.NotEmpty(t, got, "should return the direct connection IP")
	})

	t.Run("HonorsXForwardedForFromTrustedProxy", func(t *testing.T) {
		app := fiber.New(fiber.Config{
			TrustProxy: true,
			// Trust the direct connection peer used by fiber's app.Test so the
			// single forwarded hop is treated as the real client, and read it
			// from X-Forwarded-For (mirrors createFiberApp).
			TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
			ProxyHeader:      fiber.HeaderXForwardedFor,
		})
		clientIP := "203.0.113.195"

		var got string
		app.Get("/test", func(c fiber.Ctx) error {
			got = GetIP(c)

			return c.SendString(got)
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", clientIP)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		require.Equal(t, 200, resp.StatusCode, "response status should be OK")
		assert.Equal(t, clientIP, got, "X-Forwarded-For from a trusted proxy should be honored")
	})

	t.Run("SurvivesLaterTraffic", func(t *testing.T) {
		app := fiber.New(fiber.Config{
			TrustProxy:       true,
			TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
			ProxyHeader:      fiber.HeaderXForwardedFor,
		})

		// Equal-length addresses, so the second request's header lands exactly
		// on the first one's bytes when the pooled request buffer is reused.
		const (
			openingIP = "203.0.113.195"
			laterIP   = "198.51.100.42"
		)

		var captured []string
		app.Get("/test", func(c fiber.Ctx) error {
			captured = append(captured, GetIP(c))

			return c.SendString("ok")
		})

		for _, ip := range []string{openingIP, laterIP} {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", ip)
			resp, err := app.Test(req)
			require.NoError(t, err, "request should complete without error")
			require.Equal(t, 200, resp.StatusCode, "response status should be OK")
		}

		require.Len(t, captured, 2, "both requests should have captured an address")
		assert.Equal(t, openingIP, captured[0],
			"behind a trusted proxy the resolved address is a view into the pooled request buffer, so GetIP must copy it: the first address must still read as its own after a later request reused the buffer")
		assert.Equal(t, laterIP, captured[1], "the second address must read as its own")
	})
}
