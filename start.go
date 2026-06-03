package vef

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/app"
)

// startApp registers the HTTP server with the fx lifecycle. Appending the
// hook from this fx.Invoke (which runs after every module's constructor and
// its own lifecycle registrations) makes the server's OnStart the last to
// run, so it only begins accepting requests once the event bus, transports,
// and every other module have started. The OnStart also honors the start
// timeout via ctx instead of blocking unconditionally.
func startApp(lc fx.Lifecycle, application *app.App) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			select {
			case err := <-application.Start():
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		OnStop: func(context.Context) error {
			return application.Stop()
		},
	})
}
