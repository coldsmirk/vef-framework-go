package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
)

func TestCreateFiberAppDefaultBodyLimit(t *testing.T) {
	app, err := createFiberApp(&config.AppConfig{})
	require.NoError(t, err, "Fiber app should be created with default config")

	assert.Equal(t, 32*1024*1024, app.Config().BodyLimit, "Default body limit should fit MinIO multipart chunks")
}

// orderRecordingMiddleware appends its name to a shared slice when it runs,
// so a test can assert which middlewares executed and in what order.
type orderRecordingMiddleware struct {
	name  string
	order int
	log   *[]string
}

func (m *orderRecordingMiddleware) Name() string { return m.name }
func (m *orderRecordingMiddleware) Order() int   { return m.order }
func (m *orderRecordingMiddleware) Apply(router fiber.Router) {
	router.Use(func(c fiber.Ctx) error {
		*m.log = append(*m.log, m.name)

		return c.Next()
	})
}

// nopEngine is a no-op api.Engine that registers a single catch-all route so
// configureFiberApp's before/after middlewares wrap a real handler.
type nopEngine struct{}

func (*nopEngine) Register(...api.Resource) error       { return nil }
func (*nopEngine) Lookup(api.Identifier) *api.Operation { return nil }
func (*nopEngine) Mount(router fiber.Router) error {
	router.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	return nil
}

func TestConfigureFiberAppMiddlewareOrdering(t *testing.T) {
	var ran []string

	app, err := createFiberApp(&config.AppConfig{})
	require.NoError(t, err, "Fiber app should be created")

	// before-group middlewares register ahead of the routes and so wrap the
	// engine's handlers. Order 0 is the regression guard: it was previously
	// filtered out of both groups and silently never ran. An after-group
	// middleware (order 10) is included to prove configureFiberApp still
	// applies it without error; Fiber's router.Use only affects routes mounted
	// afterwards, so it does not wrap the already-mounted catch-all and is not
	// asserted in the execution order.
	middlewares := []Middleware{
		&orderRecordingMiddleware{name: "after", order: 10, log: &ran},
		&orderRecordingMiddleware{name: "zero", order: 0, log: &ran},
		&orderRecordingMiddleware{name: "before", order: -10, log: &ran},
	}

	require.NoError(t,
		configureFiberApp(app, middlewares, new(nopEngine)),
		"configureFiberApp should configure without error")

	resp, err := app.Test(httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/", nil))
	require.NoError(t, err, "test request should succeed")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "catch-all route should respond 200")

	// The Order-0 middleware must run and, being in the before group sorted
	// ascending, runs after order -10.
	assert.Equal(t, []string{"before", "zero"}, ran,
		"order 0 must run (no longer dropped) and follow order -10 in the before group")
}
