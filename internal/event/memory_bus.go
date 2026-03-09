package event

import (
	"context"
	"sync"
	"time"

	"github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/id"
)

// MemoryBus is a simple, thread-safe in-memory event bus implementation.
type MemoryBus struct {
	middlewares []event.Middleware
	subscribers map[string]map[string]*subscription
	eventCh     chan event.Event
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	started     bool
}

// subscription represents an event subscription.
type subscription struct {
	id      string
	handler event.HandlerFunc
}

// NewMemoryBus creates an in-memory event bus.
func NewMemoryBus(middlewares []event.Middleware) event.Bus {
	ctx, cancel := context.WithCancel(context.Background())

	return &MemoryBus{
		middlewares: middlewares,
		subscribers: make(map[string]map[string]*subscription),
		eventCh:     make(chan event.Event, 1000),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes and starts the event bus.
func (b *MemoryBus) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return ErrEventBusAlreadyStarted
	}

	b.wg.Go(b.processEvents)
	b.started = true

	return nil
}

// Shutdown gracefully shuts down the event bus.
func (b *MemoryBus) Shutdown(ctx context.Context) error {
	b.mu.Lock()

	if !b.started {
		b.mu.Unlock()

		return nil
	}

	b.mu.Unlock()

	b.cancel()
	close(b.eventCh)

	done := make(chan struct{})

	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		return ErrShutdownTimeoutExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Publish publishes an event asynchronously.
func (b *MemoryBus) Publish(evt event.Event) {
	b.eventCh <- evt
}

// Subscribe registers a handler for specific event types.
func (b *MemoryBus) Subscribe(eventType string, handler event.HandlerFunc) event.UnsubscribeFunc {
	subID := id.GenerateUUID()
	sub := &subscription{
		id:      subID,
		handler: handler,
	}

	b.mu.Lock()

	if b.subscribers[eventType] == nil {
		b.subscribers[eventType] = make(map[string]*subscription)
	}

	b.subscribers[eventType][subID] = sub
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if subs, exists := b.subscribers[eventType]; exists {
			delete(subs, subID)

			if len(subs) == 0 {
				delete(b.subscribers, eventType)
			}
		}
	}
}

// processEvents is the main event processing goroutine.
func (b *MemoryBus) processEvents() {
	for {
		select {
		case evt, ok := <-b.eventCh:
			if !ok {
				return
			}

			go b.deliverEvent(evt)

		case <-b.ctx.Done():
			return
		}
	}
}

// deliverEvent delivers an event to all matching subscribers.
func (b *MemoryBus) deliverEvent(evt event.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	processedEvent := evt
	if err := streams.FromSlice(b.middlewares).ForEachErr(func(middleware event.Middleware) error {
		return middleware.Process(b.ctx, processedEvent, func(_ context.Context, e event.Event) error {
			processedEvent = e

			return nil
		})
	}); err != nil {
		logger.Errorf("Error processing event middleware: %v", err)

		return
	}

	if subs, exists := b.subscribers[evt.Type()]; exists {
		for _, sub := range subs {
			sub.handler(b.ctx, processedEvent)
		}
	}
}
