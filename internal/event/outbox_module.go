package event

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
	// ErrOutboxSinkRouteMismatch indicates a routing rule references the
	// outbox transport but excludes the configured outbox.sink from its
	// subscribable transports. Subscribers attached via such a route
	// would silently miss every event: the bus filters them onto the
	// route's non publish-only transports while the outbox relay
	// dispatches into the (unrelated) sink. The check runs once during
	// fx Start so misconfigurations fail loudly instead of producing a
	// silent broken pipe at runtime.
	ErrOutboxSinkRouteMismatch = errors.New("event outbox: sink missing from outbox-bearing route")
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
	dataSources *config.DataSourcesConfig,
	db orm.DB,
) {
	if !eventCfg.Transports.Outbox.Enabled {
		return
	}

	kind := dataSources.Primary().Kind

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := ioutbox.Migrate(ctx, db, kind); err != nil {
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

	if err := validateOutboxSinkRoute(eventCfg, sinkName, transports); err != nil {
		return err
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

// validateOutboxSinkRoute asserts that every routing rule referencing
// the outbox transport keeps the configured outbox.sink as one of its
// subscribable members. Without this, a route like
// ["outbox", "redis_stream"] paired with outbox.sink="memory" passes
// HasSubscribableTransport — the route does contain a subscribable
// transport (redis_stream) — yet subscribers attached via the route
// would never see events: the relay dispatches into memory while the
// bus routes subscribers onto redis_stream.
//
// Routes that resolve only to publish-only transports (the rare
// ["outbox"]-only case used by publishers without internal
// subscribers) are skipped: there is no subscribable target to mis-
// align with, so no silent disconnect is possible.
//
// Transports unknown to the registry are skipped here; buildRouter
// surfaces them with a dedicated error during Bus.Start.
func validateOutboxSinkRoute(
	eventCfg *config.EventConfig,
	sinkName string,
	transports []transport.Transport,
) error {
	byName := make(map[string]transport.Transport, len(transports))
	for _, t := range transports {
		if t == nil {
			continue
		}

		byName[t.Name()] = t
	}

	for _, rule := range eventCfg.Routing {
		if !slices.Contains(rule.Transports, outbox.Name) {
			continue
		}

		var (
			subscribable       []string
			sinkInSubscribable bool
		)
		for _, name := range rule.Transports {
			t, ok := byName[name]
			if !ok {
				continue
			}

			if t.Capabilities().PublishOnly {
				continue
			}

			subscribable = append(subscribable, name)

			if name == sinkName {
				sinkInSubscribable = true
			}
		}

		if len(subscribable) == 0 || sinkInSubscribable {
			continue
		}

		return fmt.Errorf(
			"%w: pattern %q routes through %v but outbox.sink=%q is not among the subscribable transports %v; "+
				"the relay would dispatch to %q while subscribers attach to %v — set "+
				"vef.event.transports.outbox.sink to one of %v",
			ErrOutboxSinkRouteMismatch,
			rule.Pattern, rule.Transports, sinkName, subscribable,
			sinkName, subscribable, subscribable)
	}

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
