package binding

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
)

// Module provides version-pinned business projection, its default reference
// provider/resolver, and the durable eventual-consistency worker. Hosts can
// replace the provider or resolver; projection ownership and write-back remain
// engine-owned.
var Module = fx.Module(
	"vef:approval:binding",

	fx.Provide(
		NewNoopRefProvider,
		NewIdentityResolver,
		NewConfigValidator,
		NewWriter,
		fx.Annotate(
			NewProjector,
			fx.As(fx.Self()),
			fx.As(new(engine.InstanceProjector)),
		),
		NewWorker,
	),

	fx.Invoke(registerProjectionJob),
)

func registerProjectionJob(scheduler cron.Scheduler, worker *Worker, cfg *config.ApprovalConfig) error {
	interval := cfg.BusinessBinding.EffectiveScanInterval()

	job, err := scheduler.NewJob(cron.NewDurationJob(
		interval,
		cron.WithName("approval:binding:project"),
		cron.WithTags("approval", "binding"),
		cron.WithStartImmediately(),
		cron.WithTask(worker.Run),
	))
	if err != nil {
		return err
	}

	logger.Infof("Business projection job [%s] registered, polling every %s", job.Name(), interval)

	return nil
}
