package worker

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var (
	logger = logx.Named("storage:worker")

	// Module wires the claim sweeper and delete worker into the fx graph
	// and registers their cron jobs.
	Module = fx.Module(
		"vef:storage:worker",

		fx.Provide(NewClaimSweeper),
		fx.Provide(NewDeleteWorker),
		fx.Invoke(registerJobs),
	)
)

func registerJobs(
	scheduler cron.Scheduler,
	sweeper *ClaimSweeper,
	deleter *DeleteWorker,
	cfg *config.StorageConfig,
) error {
	sweepInterval := cfg.EffectiveSweepInterval()

	sweepJob, err := scheduler.NewJob(cron.NewDurationJob(
		sweepInterval,
		cron.WithName("storage:claim-sweep"),
		cron.WithTags("storage", "claim"),
		cron.WithTask(sweeper.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Claim sweep job [%s] registered, polling every %s", sweepJob.Name(), sweepInterval)

	deleteInterval := cfg.EffectiveDeleteWorkerInterval()

	deleteJob, err := scheduler.NewJob(cron.NewDurationJob(
		deleteInterval,
		cron.WithName("storage:delete-worker"),
		cron.WithTags("storage", "delete"),
		cron.WithTask(deleter.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Delete worker job [%s] registered, polling every %s", deleteJob.Name(), deleteInterval)

	return nil
}
