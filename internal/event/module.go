package event

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var (
	logger = log.Named("event")
	Module = fx.Module(
		"vef:event",
		fx.Provide(
			fx.Annotate(
				createMemoryBus,
				fx.ParamTags(``, `group:"vef:event:middlewares"`),
				fx.As(fx.Self()),
				fx.As(new(event.Subscriber)),
				fx.As(new(event.Publisher)),
			),
		),
	)
)

func createMemoryBus(lc fx.Lifecycle, middlewares []event.Middleware) event.Bus {
	bus := NewMemoryBus(middlewares)

	lc.Append(fx.StartStopHook(
		func() error {
			if err := bus.Start(); err != nil {
				return fmt.Errorf("failed to start event bus: %w", err)
			}

			logger.Infof("Memory event bus started (middlewares=%d)", len(middlewares))

			return nil
		},
		func(ctx context.Context) error {
			if err := bus.Shutdown(ctx); err != nil {
				return fmt.Errorf("failed to stop event bus: %w", err)
			}

			logger.Infof("Memory event bus stopped")

			return nil
		},
	))

	return bus
}
