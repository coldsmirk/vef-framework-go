package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestCompressionMiddleware(t *testing.T) {
	t.Run("SkipsProtectedAPIResponses", func(t *testing.T) {
		app := fiber.New()
		NewCompressionMiddleware(&config.APIConfig{
			BodyEncoding: config.APIBodyEncodingConfig{Enabled: true},
		}).Apply(app)
		app.Get("/api", func(ctx fiber.Ctx) error {
			return ctx.Type("json").SendString(strings.Repeat("a", 4096))
		})

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api", nil)
		req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
		response, err := app.Test(req)
		require.NoError(t, err, "fiber test request should not fail")

		assert.Empty(t, response.Header.Get(fiber.HeaderContentEncoding),
			"protected API ciphertext should not receive HTTP compression")
	})

	t.Run("CompressesUnprotectedAPIResponses", func(t *testing.T) {
		app := fiber.New()
		NewCompressionMiddleware(new(config.APIConfig)).Apply(app)
		app.Get("/api", func(ctx fiber.Ctx) error {
			return ctx.Type("json").SendString(strings.Repeat("a", 4096))
		})

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api", nil)
		req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
		response, err := app.Test(req)
		require.NoError(t, err, "fiber test request should not fail")

		assert.Equal(t, "gzip", response.Header.Get(fiber.HeaderContentEncoding),
			"legacy API responses should retain native HTTP compression")
	})
}
