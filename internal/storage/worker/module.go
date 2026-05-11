package worker

import (
	"time"

	"go.uber.org/fx"

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

func registerJobs(scheduler cron.Scheduler, sweeper *ClaimSweeper, deleter *DeleteWorker) error {
	sweepJob, err := scheduler.NewJob(cron.NewDurationJob(
		5*time.Minute,
		cron.WithName("storage:claim-sweep"),
		cron.WithTags("storage", "claim"),
		cron.WithTask(sweeper.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Claim sweep job [%s] registered, polling every 5m", sweepJob.Name())

	deleteJob, err := scheduler.NewJob(cron.NewDurationJob(
		5*time.Minute,
		cron.WithName("storage:delete-worker"),
		cron.WithTags("storage", "delete"),
		cron.WithTask(deleter.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Delete worker job [%s] registered, polling every 5m", deleteJob.Name())

	return nil
}
