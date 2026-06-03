package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trustedProxyApp mirrors createFiberApp's proxy wiring so Scheme() honors
// X-Forwarded-Proto from the (test) direct peer, which fiber.Test connects as.
func trustedProxyApp() *fiber.App {
	return fiber.New(fiber.Config{
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
	})
}

func TestHeadersMiddleware(t *testing.T) {
	apply := func(app *fiber.App) {
		NewHeadersMiddleware().Apply(app)
		app.Get("/resource", func(c fiber.Ctx) error {
			return c.SendString("ok")
		})
	}

	t.Run("AlwaysSetsNosniff", func(t *testing.T) {
		app := fiber.New()
		apply(app)

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/resource", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions), "X-Content-Type-Options must always be nosniff")
	})

	t.Run("SetsDefaultCacheControlWhenAbsent", func(t *testing.T) {
		app := fiber.New()
		apply(app)

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/resource", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Equal(t, "no-store, no-cache, must-revalidate, max-age=0", resp.Header.Get(fiber.HeaderCacheControl), "missing Cache-Control should default to no-store")
	})

	t.Run("PreservesHandlerCacheControl", func(t *testing.T) {
		app := fiber.New()
		NewHeadersMiddleware().Apply(app)
		app.Get("/cached", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")

			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/cached", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Equal(t, "public, max-age=60", resp.Header.Get(fiber.HeaderCacheControl), "handler-provided Cache-Control must be preserved")
	})

	t.Run("OmitsHSTSOverPlainHTTP", func(t *testing.T) {
		app := fiber.New()
		apply(app)

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/resource", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Empty(t, resp.Header.Get(fiber.HeaderStrictTransportSecurity), "HSTS must not be sent over plain HTTP")
	})

	t.Run("SetsHSTSWhenSchemeIsHTTPS", func(t *testing.T) {
		// Scheme() returns https from a trusted proxy's X-Forwarded-Proto; the
		// previous Protocol()-based check returned the wire version and never matched.
		app := trustedProxyApp()
		apply(app)

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/resource", nil)
		req.Header.Set(fiber.HeaderXForwardedProto, "https")
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Equal(t, "max-age=31536000; includeSubDomains", resp.Header.Get(fiber.HeaderStrictTransportSecurity), "HSTS must be sent when the request scheme is https")
	})

	t.Run("IgnoresUntrustedForwardedProto", func(t *testing.T) {
		// Without trusted-proxy configuration a spoofed X-Forwarded-Proto must not
		// trigger HSTS, since Scheme() only consults the header behind a trusted proxy.
		app := fiber.New()
		apply(app)

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/resource", nil)
		req.Header.Set(fiber.HeaderXForwardedProto, "https")
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")
		assert.Empty(t, resp.Header.Get(fiber.HeaderStrictTransportSecurity), "a spoofed X-Forwarded-Proto must not trigger HSTS without a trusted proxy")
	})
}
