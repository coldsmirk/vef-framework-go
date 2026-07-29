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
	fx.New(bootOptions(options...)...).Run()
}

// bootOptions assembles the complete option set Run hands to fx. It is split
// out of Run so the graph can be validated without starting an application:
// fx resolves dependencies lazily, so a constructor asking for a type nobody
// provides stays invisible until something boots that module.
//
// config, datasource, and the fx logger are the environment prefix; everything
// after them is ordered by bootmodules.Assemble, the single authority shared
// with the test harness (internal/apptest) so the two graphs cannot drift.
// startApp is appended last so the HTTP server starts after the scheduler and,
// on the way down, drains before it.
func bootOptions(options ...fx.Option) []fx.Option {
	prefix := []fx.Option{
		fx.WithLogger(newFxLogger),
		config.Module,
		datasource.Module,
	}

	return append(
		bootmodules.Assemble(prefix, options),
		fx.Invoke(startApp),
		fx.StartTimeout(defaultTimeout),
		fx.StopTimeout(defaultTimeout*2),
	)
}
