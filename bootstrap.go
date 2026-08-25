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
//
// When VEF_EXPORT_API names a destination the application describes its API
// surface there and exits instead of serving, so a build pipeline can read the
// contract out of the very binary that implements it without the host writing
// a second entry point.
func Run(options ...fx.Option) {
	if exportAPIIfRequested(options...) {
		return
	}

	fx.New(bootOptions(options...)...).Run()
}

// baseOptions assembles every module of the framework graph without the invoke
// that starts serving. It is the shared half of Run and ExportAPI: the module
// list and its order come from one place, so what a manifest describes cannot
// drift from what the application answers.
//
// config, datasource, and the fx logger are the environment prefix; everything
// after them is ordered by bootmodules.Assemble, the single authority also
// shared with the test harness (internal/apptest).
func baseOptions(options ...fx.Option) []fx.Option {
	prefix := []fx.Option{
		fx.WithLogger(newFxLogger),
		config.Module,
		datasource.Module,
	}

	return bootmodules.Assemble(prefix, options)
}

// bootOptions assembles the complete option set Run hands to fx. It is split
// out of Run so the graph can be validated without starting an application:
// fx resolves dependencies lazily, so a constructor asking for a type nobody
// provides stays invisible until something boots that module.
//
// startApp is appended last so the HTTP server starts after the scheduler and,
// on the way down, drains before it.
func bootOptions(options ...fx.Option) []fx.Option {
	return append(
		baseOptions(options...),
		fx.Invoke(startApp),
		fx.StartTimeout(defaultTimeout),
		fx.StopTimeout(defaultTimeout*2),
	)
}
