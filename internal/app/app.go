package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/muesli/termenv"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/version"
)

// Middleware is a middleware for the app.
type Middleware interface {
	// Name returns the name of the middleware.
	Name() string
	// Order returns the order of the middleware.
	Order() int
	// Apply applies the middleware to the router.
	Apply(router fiber.Router)
}

var logger = log.Named("app")

// App represents the VEF application server.
// It wraps a Fiber application and manages the HTTP server lifecycle.
type App struct {
	app  *fiber.App
	port uint16
}

// Start starts the VEF application HTTP server.
// It returns a channel that will receive nil when the server is ready,
// or an error if the server fails to start.
// The server runs in a goroutine and can be stopped using the Stop method.
func (a *App) Start() <-chan error {
	logger.Info("Starting VEF application...")

	// errChan is a buffered channel for error communication
	errChan := make(chan error, 1)

	go func() {
		if err := a.app.Listen(
			fmt.Sprintf(":%d", a.port),
			fiber.ListenConfig{
				EnablePrintRoutes: false,
				ShutdownTimeout:   30 * time.Second,
				BeforeServeFunc: func(*fiber.App) error {
					errChan <- nil

					output := termenv.DefaultOutput()
					fmt.Printf(` _    ______________
| |  / / ____/ ____/
| | / / __/ / /_    
| |/ / /___/ __/    
|___/_____/_/                   %s
--------------------------------------------------
`, output.String(version.VEFVersion).Foreground(termenv.ANSIBrightGreen).String())

					logger.Infof("VEF application started successfully on port %d", a.port)

					return nil
				},
			},
		); err != nil {
			logger.Errorf("Failed to start VEF application: %v", err)

			errChan <- err
		}
	}()

	return errChan
}

// Stop gracefully shuts down the VEF application server.
// It waits up to 30 seconds for active connections to close.
func (a *App) Stop() error {
	logger.Info("Stopping VEF application...")

	return a.app.ShutdownWithTimeout(time.Second * 30)
}

// Test sends an HTTP request to the application for testing purposes.
// This method is designed for unit and integration tests.
// The optional timeout parameter specifies the maximum duration to wait for a response.
// If no timeout is provided or timeout is zero, a default timeout of 5 seconds is used.
// This is higher than Fiber's default (1s) to accommodate CI environments.
func (a *App) Test(req *http.Request, timeout ...time.Duration) (*http.Response, error) {
	t := 5 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		t = timeout[0]
	}

	return a.app.Test(req, fiber.TestConfig{
		Timeout: t,
	})
}

// AppParams contains all dependencies required to create a VEF application.
type AppParams struct {
	fx.In

	Config      *config.AppConfig
	Middlewares []Middleware `group:"vef:app:middlewares"`
	APIEngine   api.Engine
}

// New creates a new VEF application instance with the provided dependencies.
// Returns an error if the application cannot be configured properly.
func New(params AppParams) (*App, error) {
	logger.Info("Initializing VEF application...")

	fiberApp, err := createFiberApp(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create fiber app: %w", err)
	}

	if err := configureFiberApp(fiberApp, params.Middlewares, params.APIEngine); err != nil {
		return nil, err
	}

	return &App{
		app:  fiberApp,
		port: params.Config.Port,
	}, nil
}
