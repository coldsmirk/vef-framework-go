package vef

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/app"
)

// startApp starts the application.
// It registers the application stop hook with the fx lifecycle manager.
func startApp(lc fx.Lifecycle, app *app.App) error {
	if err := <-app.Start(); err != nil {
		return err
	}

	lc.Append(fx.StopHook(app.Stop))

	return nil
}
