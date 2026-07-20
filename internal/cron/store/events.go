package store

import (
	"context"
	"sync"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
)

const runEventQueueSize = 256

// RunEventPublisher emits the store's operational notifications. Publishing
// is best-effort by design: the run journal is the durable truth, events only
// feed alerting. A bounded worker isolates transport back-pressure from
// claiming and executor slots; a full queue drops the notification.
type RunEventPublisher struct {
	bus    event.Bus
	queue  chan event.Event
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	start  sync.Once
}

// NewRunEventPublisher builds the publisher over the application event bus.
func NewRunEventPublisher(bus event.Bus) *RunEventPublisher {
	ctx, cancel := context.WithCancel(context.Background())

	return &RunEventPublisher{
		bus:    bus,
		queue:  make(chan event.Event, runEventQueueSize),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// Start launches the single publish worker.
func (p *RunEventPublisher) Start() {
	p.start.Do(func() { go p.loop() })
}

// Stop cancels the in-flight publish and waits within the caller's budget.
func (p *RunEventPublisher) Stop(ctx context.Context) error {
	p.cancel()
	// Starting after cancellation makes Stop safe for an engine that was built
	// but never started: the worker observes cancellation and closes done.
	p.Start()

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunFailed reports a failed run.
func (p *RunEventPublisher) RunFailed(run *cron.Run) {
	p.offer(cron.NewRunFailedEvent(run))
}

// RunAbandoned reports an abandoned run.
func (p *RunEventPublisher) RunAbandoned(run *cron.Run) {
	p.offer(cron.NewRunAbandonedEvent(run))
}

func (p *RunEventPublisher) offer(evt event.Event) {
	if p.ctx.Err() != nil {
		return
	}

	select {
	case p.queue <- evt:
	default:
		logger.Warnf("Drop %s: cron run event queue is full", evt.EventType())
	}
}

func (p *RunEventPublisher) loop() {
	defer close(p.done)

	for {
		if p.ctx.Err() != nil {
			return
		}

		select {
		case <-p.ctx.Done():
			return
		case evt := <-p.queue:
			if err := p.bus.Publish(p.ctx, evt); err != nil && p.ctx.Err() == nil {
				logger.Warnf("Publish %s: %v", evt.EventType(), err)
			}
		}
	}
}
