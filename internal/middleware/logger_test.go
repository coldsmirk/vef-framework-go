package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/middleware"
)

// Request IDs of equal length, so the second request's header lands exactly on
// the first one's bytes when the pooled request buffer is reused. Equal length
// is what makes the aliasing observable rather than a matter of luck.
const (
	openingRequestID = "req-AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAA"
	laterRequestID   = "req-BBBBBBBB-BBBB-BBBB-BBBB-BBBBBBBB"
)

// newRequestIDApp builds the request-id + logger middleware pair over a handler
// that parks the context's request id where it outlives the request, mirroring
// what an asynchronously published audit event or event envelope does with it.
func newRequestIDApp(captured *[]string) *fiber.App {
	app := fiber.New()

	for _, m := range []interface{ Apply(fiber.Router) }{
		middleware.NewRequestIDMiddleware(),
		middleware.NewLoggerMiddleware(),
	} {
		m.Apply(app)
	}

	app.Get("/", func(ctx fiber.Ctx) error {
		*captured = append(*captured, contextx.RequestID(ctx))

		return ctx.SendString("ok")
	})

	return app
}

// get drives one request carrying the given client-supplied request id.
func get(t *testing.T, app *fiber.App, requestID string) {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderXRequestID, requestID)

	resp, err := app.Test(req)
	require.NoError(t, err, "The request should complete")
	require.Equal(t, http.StatusOK, resp.StatusCode, "The request should succeed")
}

// TestRequestIDSurvivesLaterTraffic pins that the request id parked in the
// context is copied out of the request buffer. Fiber's requestid middleware
// returns a client-supplied X-Request-ID verbatim, and the framework builds
// Fiber with Immutable off, so without the copy in NewLoggerMiddleware the
// value is a view into the pooled buffer — and every consumer that reads it
// after the request returned (the audit event and the event envelope's
// CorrelationID are both built behind an asynchronous publish) reports whatever
// request reused that buffer.
func TestRequestIDSurvivesLaterTraffic(t *testing.T) {
	var captured []string

	app := newRequestIDApp(&captured)

	get(t, app, openingRequestID)
	get(t, app, laterRequestID)

	require.Len(t, captured, 2, "Both requests should have parked their request id")
	assert.Equal(t, openingRequestID, captured[0],
		"The first request id must still read as its own after a later request reused the buffer")
	assert.Equal(t, laterRequestID, captured[1], "The second request id must read as its own")
}

// TestGeneratedRequestIDIsPreserved covers the branch where no client supplies
// the header: the middleware generates the id itself, so there is nothing to
// alias, and the copy must not disturb it.
func TestGeneratedRequestIDIsPreserved(t *testing.T) {
	var captured []string

	app := newRequestIDApp(&captured)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "The request should complete")
	require.Equal(t, http.StatusOK, resp.StatusCode, "The request should succeed")

	require.Len(t, captured, 1, "The request should have parked a request id")
	assert.NotEmpty(t, captured[0], "A request without X-Request-ID must still carry a generated id")
}
