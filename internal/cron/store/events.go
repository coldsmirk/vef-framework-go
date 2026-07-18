package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
)

// RunEventPublisher emits the store's operational notifications. Publishing
// is best-effort by design: the run journal is the durable truth, events
// only feed alerting, so a transport hiccup is logged and never propagated
// into scheduling.
type RunEventPublisher struct {
	bus event.Bus
}

// NewRunEventPublisher builds the publisher over the application event bus.
func NewRunEventPublisher(bus event.Bus) *RunEventPublisher {
	return &RunEventPublisher{bus: bus}
}

// RunFailed reports a failed run.
func (p *RunEventPublisher) RunFailed(ctx context.Context, run *cron.Run) {
	p.publish(ctx, cron.NewRunFailedEvent(run))
}

// RunAbandoned reports an abandoned run.
func (p *RunEventPublisher) RunAbandoned(ctx context.Context, run *cron.Run) {
	p.publish(ctx, cron.NewRunAbandonedEvent(run))
}

func (p *RunEventPublisher) publish(ctx context.Context, evt event.Event) {
	if err := p.bus.Publish(ctx, evt); err != nil {
		logger.Warnf("Publish %s: %v", evt.EventType(), err)
	}
}
