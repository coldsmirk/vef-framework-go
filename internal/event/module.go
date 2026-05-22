package event

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	imiddleware "github.com/coldsmirk/vef-framework-go/internal/event/middleware"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
)

// Module wires the Bus, registers the always-on memory transport, and
// exposes fx groups for downstream modules (outbox, redis_stream, inbox)
// to plug additional transports and middleware.
var Module = fx.Module(
	"vef:event",
	fx.Provide(
		fx.Annotate(
			newMemoryTransport,
			fx.ResultTags(`group:"vef:event:transports"`),
			fx.As(new(transport.Transport)),
		),
		defaultErrorSink,
		defaultMetricsRecorder,
		// Built-in cross-cutting middlewares; each constructor consults
		// the EventConfig.Middleware toggles and returns nil to opt out.
		fx.Annotate(
			newRecoverMiddleware,
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
			fx.As(new(middleware.ConsumeMiddleware)),
		),
		fx.Annotate(
			newLoggingPublishMiddleware,
			fx.ResultTags(`group:"vef:event:publish-middlewares"`),
			fx.As(new(middleware.PublishMiddleware)),
		),
		fx.Annotate(
			newLoggingConsumeMiddleware,
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
			fx.As(new(middleware.ConsumeMiddleware)),
		),
		fx.Annotate(
			newTracingPublishMiddleware,
			fx.ResultTags(`group:"vef:event:publish-middlewares"`),
			fx.As(new(middleware.PublishMiddleware)),
		),
		fx.Annotate(
			newTracingConsumeMiddleware,
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
			fx.As(new(middleware.ConsumeMiddleware)),
		),
		fx.Annotate(
			newMetricsPublishMiddleware,
			fx.ResultTags(`group:"vef:event:publish-middlewares"`),
			fx.As(new(middleware.PublishMiddleware)),
		),
		fx.Annotate(
			newMetricsConsumeMiddleware,
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
			fx.As(new(middleware.ConsumeMiddleware)),
		),
		fx.Annotate(
			newBus,
			fx.ParamTags(
				``,
				``,
				`group:"vef:event:transports"`,
				`group:"vef:event:publish-middlewares"`,
				`group:"vef:event:consume-middlewares"`,
				``,
				``,
			),
			fx.As(fx.Self()),
			fx.As(new(event.Bus)),
			fx.As(new(event.RouteInspector)),
		),
	),
)

func newMemoryTransport(cfg *config.EventConfig) transport.Transport {
	return imemory.New(memory.Config{
		QueueSize:      cfg.Transports.Memory.QueueSize,
		FullPolicy:     memory.FullPolicy(cfg.Transports.Memory.FullPolicy),
		PublishTimeout: cfg.Transports.Memory.PublishTimeout,
	})
}

func newBus(
	eventCfg *config.EventConfig,
	appCfg *config.AppConfig,
	transports []transport.Transport,
	publishMW []middleware.PublishMiddleware,
	consumeMW []middleware.ConsumeMiddleware,
	sink event.ErrorSink,
	lc fx.Lifecycle,
) *Bus {
	bus := NewBus(eventCfg, appCfg.Name, transports, publishMW, consumeMW, sink)
	lc.Append(fx.StartStopHook(bus.Start, bus.Stop))

	return bus
}

func defaultErrorSink() event.ErrorSink {
	return func(err error, env event.Envelope) {
		busLogger.Errorf("async publish failed (type=%s, id=%s): %v", env.Type, env.ID, err)
	}
}

// Middleware constructors. Each returns nil when the corresponding
// config toggle is off, and fx silently drops nil entries from the
// group so the bus's middleware chain stays clean.

func newRecoverMiddleware(cfg *config.EventConfig) middleware.ConsumeMiddleware {
	if !cfg.Middleware.Recover {
		return nil
	}

	return imiddleware.NewRecover(busLogger)
}

func newLoggingPublishMiddleware(cfg *config.EventConfig) middleware.PublishMiddleware {
	if !cfg.Middleware.Logging {
		return nil
	}

	return imiddleware.NewLogging(busLogger)
}

func newLoggingConsumeMiddleware(cfg *config.EventConfig) middleware.ConsumeMiddleware {
	if !cfg.Middleware.Logging {
		return nil
	}

	return imiddleware.NewLogging(busLogger)
}

func newTracingPublishMiddleware(cfg *config.EventConfig) middleware.PublishMiddleware {
	if !cfg.Middleware.Tracing {
		return nil
	}

	if cfg.Middleware.TracingStrict {
		return imiddleware.NewTracingStrict()
	}

	return imiddleware.NewTracing()
}

func newTracingConsumeMiddleware(cfg *config.EventConfig) middleware.ConsumeMiddleware {
	if !cfg.Middleware.Tracing {
		return nil
	}

	if cfg.Middleware.TracingStrict {
		return imiddleware.NewTracingStrict()
	}

	return imiddleware.NewTracing()
}

func newMetricsPublishMiddleware(cfg *config.EventConfig, rec event.MetricsRecorder) middleware.PublishMiddleware {
	if !cfg.Middleware.Metrics {
		return nil
	}

	return imiddleware.NewMetrics(rec)
}

func newMetricsConsumeMiddleware(cfg *config.EventConfig, rec event.MetricsRecorder) middleware.ConsumeMiddleware {
	if !cfg.Middleware.Metrics {
		return nil
	}

	return imiddleware.NewMetrics(rec)
}

// defaultMetricsRecorder publishes lightweight expvar maps. Override
// via fx.Decorate (see vef.ProvideEventMetricsRecorder) to forward
// observations to Prometheus / OpenTelemetry / vendor SDKs.
func defaultMetricsRecorder() event.MetricsRecorder {
	return imiddleware.NewExpvarMetricsRecorder()
}
