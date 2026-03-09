package timeout

import (
	"time"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var (
	logger = log.Named("approval:timeout")

	// Module provides the timeout scanner and cron job registration.
	Module = fx.Module(
		"vef:approval:timeout",

		fx.Provide(NewScanner),
		fx.Invoke(registerTimeoutJobs),
	)
)

func registerTimeoutJobs(scheduler cron.Scheduler, scanner *Scanner) error {
	scanJob, err := scheduler.NewJob(cron.NewDurationJob(
		1*time.Minute,
		cron.WithName("approval:timeout:scan"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanTimeouts),
	))
	if err != nil {
		return err
	}

	logger.Infof("Timeout scan job [%s] registered, polling every 1m", scanJob.Name())

	preWarnJob, err := scheduler.NewJob(cron.NewDurationJob(
		5*time.Minute,
		cron.WithName("approval:timeout:pre_warning"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanPreWarnings),
	))
	if err != nil {
		return err
	}

	logger.Infof("Pre-warning scan job [%s] registered, polling every 5m", preWarnJob.Name())

	return nil
}
