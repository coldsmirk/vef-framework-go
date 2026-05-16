package timeout

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var (
	logger = logx.Named("approval:timeout")

	// Module provides the timeout scanner and cron job registration.
	Module = fx.Module(
		"vef:approval:timeout",

		fx.Provide(NewScanner),
		fx.Invoke(registerTimeoutJobs),
	)
)

func registerTimeoutJobs(scheduler cron.Scheduler, scanner *Scanner, cfg *config.ApprovalConfig) error {
	scanJob, err := scheduler.NewJob(cron.NewDurationJob(
		cfg.TimeoutScanInterval,
		cron.WithName("approval:timeout:scan"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanTimeouts),
	))
	if err != nil {
		return err
	}

	logger.Infof("Timeout scan job [%s] registered, polling every %s", scanJob.Name(), cfg.TimeoutScanInterval)

	preWarnJob, err := scheduler.NewJob(cron.NewDurationJob(
		cfg.PreWarningScanInterval,
		cron.WithName("approval:timeout:pre_warning"),
		cron.WithTags("approval", "timeout"),
		cron.WithTask(scanner.ScanPreWarnings),
	))
	if err != nil {
		return err
	}

	logger.Infof("Pre-warning scan job [%s] registered, polling every %s", preWarnJob.Name(), cfg.PreWarningScanInterval)

	cleanupJob, err := scheduler.NewJob(cron.NewDurationJob(
		cfg.CleanupScanInterval,
		cron.WithName("approval:cleanup"),
		cron.WithTags("approval", "cleanup"),
		cron.WithTask(scanner.CleanupExpiredRecords),
	))
	if err != nil {
		return err
	}

	logger.Infof("Cleanup job [%s] registered, polling every %s", cleanupJob.Name(), cfg.CleanupScanInterval)

	return nil
}
