package dispatcher

import (
	"time"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var (
	logger = log.Named("approval:dispatcher")

	// Module provides the event publisher, dispatcher, and relay.
	Module = fx.Module(
		"vef:approval:dispatcher",

		fx.Provide(
			NewEventPublisher,
			NewBusDispatcher,
			NewRelay,
		),
		fx.Invoke(registerRelayJob),
	)
)

func registerRelayJob(scheduler cron.Scheduler, relay *Relay, cfg *config.ApprovalConfig) error {
	interval := time.Duration(cfg.OutboxRelayIntervalOrDefault()) * time.Second

	job, err := scheduler.NewJob(cron.NewDurationJob(
		interval,
		cron.WithName("approval:outbox:relay"),
		cron.WithTags("approval", "outbox"),
		cron.WithTask(relay.RelayPending),
	))
	if err != nil {
		return err
	}

	logger.Infof("Outbox relay job [%s] registered, polling every %s", job.Name(), interval)

	return nil
}
