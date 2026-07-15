package worker

import (
	"time"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// logPruneInterval is the sweep cadence of the invocation log pruner; the
// retention window itself comes from vef.integration.log.retention.
const logPruneInterval = time.Hour

var (
	logger = logx.Named("integration:worker")

	// Module wires the invocation log pruner into the fx graph and registers
	// its cron job when a retention window is configured.
	Module = fx.Module(
		"vef:integration:worker",

		fx.Provide(NewLogPruner),
		fx.Invoke(registerJobs),
	)
)

func registerJobs(scheduler cron.Scheduler, pruner *LogPruner, cfg *config.IntegrationConfig) error {
	if cfg.Log.Retention <= 0 {
		return nil
	}

	job, err := scheduler.NewJob(cron.NewDurationJob(
		logPruneInterval,
		cron.WithName("integration:log-prune"),
		cron.WithTags("integration", "log"),
		cron.WithTask(pruner.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Invocation log prune job [%s] registered, polling every %s (retention %s)",
		job.Name(), logPruneInterval, cfg.Log.Retention)

	return nil
}
