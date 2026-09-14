package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
)

func TestCORSMiddleware(t *testing.T) {
	app := fiber.New()
	NewCORSMiddleware(&config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"https://app.example.com"},
	}).Apply(app)
	app.Get("/api", func(ctx fiber.Ctx) error {
		ctx.Set(api.HeaderXBodyEncoding, string(config.APIBodyEncodingAESGCMBase64))

		return ctx.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api", nil)
	req.Header.Set(fiber.HeaderOrigin, "https://app.example.com")
	response, err := app.Test(req)
	require.NoError(t, err, "fiber test request should not fail")

	assert.Contains(t, response.Header.Get(fiber.HeaderAccessControlExposeHeaders), api.HeaderXBodyEncoding,
		"browser clients should be allowed to read the response body encoding marker")
}
