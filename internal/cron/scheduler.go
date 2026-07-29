package cron

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("cron")

// newScheduler creates a new gocron scheduler with optimal configuration for production use.
func newScheduler() (gocron.Scheduler, error) {
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(time.Local),
		gocron.WithStopTimeout(30*time.Second),
		gocron.WithLogger(newCronLogger()),
		gocron.WithMonitorStatus(newJobMonitor()),
		gocron.WithLimitConcurrentJobs(1000, gocron.LimitModeWait),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cron scheduler: %w", err)
	}

	return scheduler, nil
}

// StartScheduler runs the scheduler for the application's lifetime. It is
// deliberately not registered from newScheduler: fx runs start hooks in append
// order, and a hook appended from a constructor lands wherever that constructor
// first resolves — for the scheduler, at the first job registration, which is
// ahead of the hooks that provision the tables those jobs read. Registering it
// from an invoke instead makes the position explicit, and bootmodules.Assemble
// places that invoke after every module and after the host's own options, so a
// job cannot tick against a schema that does not exist yet — not even one
// registered with cron.WithStartImmediately, as the approval business-projection
// worker is.
//
// It must stay a plain fx.Invoke rather than an fx.Module: fx runs every
// child-module invoke before any root-level one (fx/module.go, invokeAll), so a
// module here would still be overtaken by a host option written as a bare
// fx.Invoke.
//
// The one class this does not order ahead of is constructors nothing resolves
// until startApp builds app.App — host API resources, middleware, decorators
// over them. Moving this invoke after startApp would cover them but flip
// shutdown: the scheduler would stop before HTTP drained, leaving a request
// handler holding a dead scheduler. As placed, appending late means stopping
// early, which is the order shutdown wants — jobs stop before the event bus and
// the data sources they use, while startApp is appended after and drains first.
// Provisioning belongs in a module, not in an API resource's constructor.
func StartScheduler(lc fx.Lifecycle, scheduler gocron.Scheduler) {
	lc.Append(fx.StartStopHook(
		func() {
			scheduler.Start()
			logger.Info("Cron scheduler started")
		},
		func() error {
			if err := scheduler.Shutdown(); err != nil {
				return fmt.Errorf("failed to stop scheduler: %w", err)
			}

			logger.Info("Cron scheduler stopped")

			return nil
		},
	))
}
