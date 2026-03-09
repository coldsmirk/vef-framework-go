package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/go-streams"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
)

// createFiberApp creates a new Fiber application with the given configuration.
func createFiberApp(cfg *config.AppConfig) (*fiber.App, error) {
	bodyLimitStr := lo.CoalesceOrEmpty(strings.TrimSpace(cfg.BodyLimit), "10mib")

	bodyLimit, err := humanize.ParseBytes(bodyLimitStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse body limit: %w", err)
	}

	return fiber.NewWithCustomCtx(
		func(app *fiber.App) fiber.CustomCtx {
			return &CustomCtx{
				DefaultCtx: *fiber.NewDefaultCtx(app),
			}
		},
		fiber.Config{
			AppName:         lo.CoalesceOrEmpty(cfg.Name, "vef-app"),
			BodyLimit:       int(bodyLimit),
			CaseSensitive:   true,
			IdleTimeout:     30 * time.Second,
			ErrorHandler:    handleError,
			StrictRouting:   false,
			StructValidator: newStructValidator(),
			ServerHeader:    "vef",
			Concurrency:     1024 * 1024,
			ReadBufferSize:  8192,
			WriteBufferSize: 8192,
			Immutable:       false,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    120 * time.Second,
		},
	), nil
}

// configureFiberApp configures the Fiber application with middlewares and routes.
// Middlewares are separated into before (order < 0) and after (order > 0) groups,
// sorted by order, and applied around the API engine registration.
// This ensures proper middleware execution order relative to route handlers.
func configureFiberApp(
	app *fiber.App,
	middlewares []Middleware,
	apiEngine api.Engine,
) error {
	beforeMiddlewares := streams.FromSlice(middlewares).
		Filter(func(m Middleware) bool {
			return m != nil && m.Order() < 0
		}).
		Sorted(func(a, b Middleware) int {
			return a.Order() - b.Order()
		})

	afterMiddlewares := streams.FromSlice(middlewares).
		Filter(func(m Middleware) bool {
			return m != nil && m.Order() > 0
		}).
		Sorted(func(a, b Middleware) int {
			return a.Order() - b.Order()
		})

	beforeMiddlewares.ForEach(func(m Middleware) {
		logger.Infof("Applying before middleware %q", m.Name())
		m.Apply(app)
	})

	if err := apiEngine.Mount(app); err != nil {
		return fmt.Errorf("failed to mount api engine: %w", err)
	}

	afterMiddlewares.ForEach(func(m Middleware) {
		logger.Infof("Applying after middleware %q", m.Name())
		m.Apply(app)
	})

	return nil
}
