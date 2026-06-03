package vef

import (
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/coldsmirk/vef-framework-go/internal/bootmodules"
	"github.com/coldsmirk/vef-framework-go/internal/config"
	"github.com/coldsmirk/vef-framework-go/internal/datasource"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Default timeout for framework startup and shutdown.
const defaultTimeout = 30 * time.Second

func newFxLogger() fxevent.Logger {
	return &fxevent.SlogLogger{
		Logger: ilogx.NewSLogger("vef", 5, logx.LevelWarn),
	}
}

// Run starts the VEF framework with the provided options.
// It initializes all core modules and runs the application.
func Run(options ...fx.Option) {
	// config, datasource, and the fx logger are wired here; the business
	// modules come from bootmodules.Core(), the single source of truth shared
	// with the test harness (internal/apptest) so the two graphs cannot drift.
	opts := []fx.Option{
		fx.WithLogger(newFxLogger),
		config.Module,
		datasource.Module,
	}

	opts = append(opts, bootmodules.Core()...)
	opts = append(opts, options...)
	opts = append(
		opts,
		fx.Invoke(startApp),
		fx.StartTimeout(defaultTimeout),
		fx.StopTimeout(defaultTimeout*2),
	)

	app := fx.New(opts...)
	app.Run()
}
