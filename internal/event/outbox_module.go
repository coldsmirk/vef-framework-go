package event

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	ioutbox "github.com/coldsmirk/vef-framework-go/internal/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var (
	outboxLogger = logx.Named("event:outbox")

	errOutboxSinkMissing      = errors.New("event outbox: configured sink not found in transports")
	errOutboxTransportMissing = errors.New("event outbox: transport instance not in registry")
)

// OutboxModule wires the outbox transport, its repository, the
// database migration, and the relay cron job. Sink binding is deferred
// until after the transport registry is fully populated to avoid a
// circular fx dependency.
//
// All components are gated by vef.event.transports.outbox.enabled —
// when disabled, no Transport is contributed to the registry and the
// relay cron job is not registered.
var OutboxModule = fx.Module(
	"vef:event:outbox",
	fx.Provide(
		fx.Annotate(
			newOutboxRepository,
			fx.As(fx.Self()),
			fx.As(new(outbox.Repository)),
		),
		// The outbox transport is constructed without its sink; the
		// fx.Invoke at the bottom of this module binds the sink once
		// every transport has been provided into the group.
		fx.Annotate(
			newOutboxTransport,
			fx.ResultTags(`group:"vef:event:transports"`),
			fx.As(new(transport.Transport)),
		),
	),
	fx.Invoke(runOutboxMigration),
	fx.Invoke(registerOutboxCleanup),
	fx.Invoke(
		fx.Annotate(
			bindOutboxSinkAndRelay,
			fx.ParamTags(``, ``, ``, `group:"vef:event:transports"`),
		),
	),
)

func newOutboxRepository(db orm.DB) *ioutbox.DefaultRepository {
	return ioutbox.NewRepository(db)
}

// outboxConfig collapses the framework-level config into the transport
// package's Config struct, applying configured overrides.
func outboxConfig(cfg *config.EventConfig) outbox.Config {
	c := cfg.Transports.Outbox

	return outbox.Config{
		RelayInterval:   c.RelayInterval,
		MaxRetries:      c.MaxRetries,
		BatchSize:       c.BatchSize,
		LeaseMultiplier: c.LeaseMultiplier,
		MinLease:        c.MinLease,
		SinkName:        c.SinkName,
	}
}

func newOutboxTransport(cfg *config.EventConfig, repo outbox.Repository) transport.Transport {
	if !cfg.Transports.Outbox.Enabled {
		return nil
	}

	return ioutbox.NewTransport(repo, outboxConfig(cfg))
}

func runOutboxMigration(
	lc fx.Lifecycle,
	eventCfg *config.EventConfig,
	dsCfg *config.DataSourceConfig,
	db orm.DB,
) {
	if !eventCfg.Transports.Outbox.Enabled {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := ioutbox.Migrate(ctx, db, dsCfg.Kind); err != nil {
				return fmt.Errorf("outbox migration: %w", err)
			}

			return nil
		},
	})
}

// bindOutboxSinkAndRelay locates the outbox transport in the registry
// and binds it to its configured sink. It then registers the relay
// cron job so dispatch begins automatically.
func bindOutboxSinkAndRelay(
	eventCfg *config.EventConfig,
	scheduler cron.Scheduler,
	repo outbox.Repository,
	transports []transport.Transport,
) error {
	if !eventCfg.Transports.Outbox.Enabled {
		return nil
	}

	sinkName := eventCfg.Transports.Outbox.SinkName
	if sinkName == "" {
		sinkName = "memory"
	}

	// Sanity check: outbox sink="memory" while redis_stream is enabled is
	// usually a configuration oversight — the outbox writes records in
	// the caller's transaction but the relay only dispatches them in-
	// process, so cross-node subscribers never see them. Surface this
	// explicitly so operators can correct the routing.
	if sinkName == "memory" && eventCfg.Transports.RedisStream.Enabled {
		outboxLogger.Warnf(
			"outbox sink is 'memory' but redis_stream transport is enabled; "+
				"cross-process subscribers will not receive outbox events. "+
				"Set vef.event.transports.outbox.sink = %q to dispatch across nodes.",
			"redis_stream")
	}

	var (
		sink    transport.Transport
		outboxT *ioutbox.Transport
	)
	for _, t := range transports {
		if t == nil {
			continue
		}

		if t.Name() == sinkName {
			sink = t
		}

		if ot, ok := t.(*ioutbox.Transport); ok {
			outboxT = ot
		}
	}

	if sink == nil {
		return fmt.Errorf("%w: %q", errOutboxSinkMissing, sinkName)
	}

	if outboxT == nil {
		return errOutboxTransportMissing
	}

	outboxT.SetSink(sink)

	cfg := outboxConfig(eventCfg)
	relay := ioutbox.NewRelay(repo, outboxT.Sink, cfg, outboxLogger, nil)

	interval := cfg.EffectiveRelayInterval()

	job, err := scheduler.NewJob(cron.NewDurationJob(
		interval,
		cron.WithName("vef:event:outbox:relay"),
		cron.WithTags("vef", "event", "outbox"),
		cron.WithTask(relay.RelayPending),
	))
	if err != nil {
		return fmt.Errorf("register outbox relay job: %w", err)
	}

	outboxLogger.Infof("Outbox relay job [%s] registered, polling every %s", job.Name(), interval)

	return nil
}

func registerOutboxCleanup(
	eventCfg *config.EventConfig,
	scheduler cron.Scheduler,
	repo outbox.Repository,
) error {
	if !eventCfg.Transports.Outbox.Enabled {
		return nil
	}

	cleaner := ioutbox.NewCleaner(repo, eventCfg.Transports.Outbox.EffectiveCompletedTTL(), outboxLogger)
	interval := eventCfg.Transports.Outbox.EffectiveCleanupInterval()

	job, err := scheduler.NewJob(cron.NewDurationJob(
		interval,
		cron.WithName("vef:event:outbox:cleanup"),
		cron.WithTags("vef", "event", "outbox"),
		cron.WithTask(cleaner.Cleanup),
	))
	if err != nil {
		return fmt.Errorf("register outbox cleanup job: %w", err)
	}

	outboxLogger.Infof("Outbox cleanup job [%s] registered, polling every %s", job.Name(), interval)

	return nil
}
