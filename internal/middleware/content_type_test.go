package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runContentTypeRequest sends one request through the middleware to a
// catch-all route that reports whether it was reached.
func runContentTypeRequest(t *testing.T, method, path, contentType string) int {
	t.Helper()

	app := fiber.New()
	NewContentTypeMiddleware().Apply(app)
	app.All("/*", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), method, path, nil)
	if contentType != "" {
		req.Header.Set(fiber.HeaderContentType, contentType)
	}

	resp, err := app.Test(req)
	require.NoError(t, err, "Fiber test request should not fail")

	return resp.StatusCode
}

func TestContentTypeMiddleware(t *testing.T) {
	t.Run("APIPostRequiresJSON", func(t *testing.T) {
		status := runContentTypeRequest(t, "POST", "/api", "text/xml")
		assert.Equal(t, http.StatusUnsupportedMediaType, status, "Non-JSON POST to the API surface must be rejected")
	})

	t.Run("APIPostAcceptsJSON", func(t *testing.T) {
		status := runContentTypeRequest(t, "POST", "/api", fiber.MIMEApplicationJSON)
		assert.Equal(t, http.StatusOK, status, "JSON POST to the API surface must pass")
	})

	t.Run("APIGetUnrestricted", func(t *testing.T) {
		status := runContentTypeRequest(t, "GET", "/api/users", "")
		assert.Equal(t, http.StatusOK, status, "Reads carry no body contract")
	})

	t.Run("NonAPISurfaceUnrestricted", func(t *testing.T) {
		status := runContentTypeRequest(t, "POST", "/integration/inbound/his/patient.get", "text/xml")
		assert.Equal(t, http.StatusOK, status, "Vendor callbacks outside /api may POST any format")
	})

	t.Run("PrefixMustBeASegment", func(t *testing.T) {
		status := runContentTypeRequest(t, "POST", "/apidocs", "text/xml")
		assert.Equal(t, http.StatusOK, status, "/apidocs is not part of the API surface")
	})
}
