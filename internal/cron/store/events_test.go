package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
)

// BlockingBus holds Publish until released and can optionally honor context
// cancellation.
type BlockingBus struct {
	entered        chan struct{}
	release        chan struct{}
	respectContext bool
	once           sync.Once
	releaseOnce    sync.Once
}

func (b *BlockingBus) releasePublish() {
	b.releaseOnce.Do(func() { close(b.release) })
}

func newBlockingBus(respectContext bool) *BlockingBus {
	return &BlockingBus{
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
		respectContext: respectContext,
	}
}

func (b *BlockingBus) Publish(ctx context.Context, _ event.Event, _ ...event.PublishOption) error {
	b.once.Do(func() { close(b.entered) })

	if b.respectContext {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.release:
			return nil
		}
	}

	<-b.release

	return nil
}

func (b *BlockingBus) PublishBatch(ctx context.Context, events []event.Event, opts ...event.PublishOption) error {
	for _, evt := range events {
		if err := b.Publish(ctx, evt, opts...); err != nil {
			return err
		}
	}

	return nil
}

func (*BlockingBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

func TestRunEventPublisherStopCancelsPublish(t *testing.T) {
	bus := newBlockingBus(true)
	publisher := NewRunEventPublisher(bus)
	t.Cleanup(func() {
		bus.releasePublish()
		require.NoError(t, publisher.Stop(context.Background()), "The publisher should stop during cleanup")
	})
	publisher.Start()
	publisher.RunFailed(new(cron.Run))

	select {
	case <-bus.entered:
	case <-time.After(time.Second):
		t.Fatal("The worker must enter the blocking publish")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, publisher.Stop(stopCtx),
		"Stopping should cancel a context-aware in-flight publish")
}

func TestRunEventPublisherStopObeysDeadline(t *testing.T) {
	bus := newBlockingBus(false)
	publisher := NewRunEventPublisher(bus)
	t.Cleanup(func() {
		bus.releasePublish()
		require.NoError(t, publisher.Stop(context.Background()), "The publisher should stop during cleanup")
	})
	publisher.Start()
	publisher.RunFailed(new(cron.Run))

	select {
	case <-bus.entered:
	case <-time.After(time.Second):
		t.Fatal("The worker must enter the blocking publish")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := publisher.Stop(stopCtx)

	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"A bus that ignores cancellation must not outlive the caller's stop budget")

	bus.releasePublish()
	require.NoError(t, publisher.Stop(context.Background()),
		"Stopping should finish after the non-compliant bus is released")
}

func TestRunEventPublisherStopFlushesTheQueue(t *testing.T) {
	t.Run("PublishesQueuedEvents", func(t *testing.T) {
		bus := eventtest.NewFakeBus()
		publisher := NewRunEventPublisher(bus)

		publisher.RunFailed(new(cron.Run))
		publisher.RunAbandoned(new(cron.Run))

		require.NoError(t, publisher.Stop(context.Background()),
			"Stopping should flush the queue within its grace")
		assert.Len(t, bus.Captured(), 2,
			"Shutdown must publish the notifications the queue still holds, not discard them")
	})

	t.Run("BoundsTheFlushByItsGrace", func(t *testing.T) {
		bus := newBlockingBus(true)
		publisher := NewRunEventPublisher(bus)
		t.Cleanup(bus.releasePublish)

		publisher.RunFailed(new(cron.Run))
		publisher.RunFailed(new(cron.Run))

		startedAt := time.Now()

		require.NoError(t, publisher.Stop(context.Background()),
			"A bus that never answers must not hold the publisher open beyond its grace")

		elapsed := time.Since(startedAt)
		assert.GreaterOrEqual(t, elapsed, publisherStopGrace, "The flush should use the whole grace before giving up")
		assert.Less(t, elapsed, 3*publisherStopGrace, "The flush must stop at its grace instead of retrying the queue")
	})
}

func TestRunEventPublisherDropsWhenQueueIsFull(t *testing.T) {
	bus := newBlockingBus(false)
	publisher := NewRunEventPublisher(bus)
	t.Cleanup(func() {
		bus.releasePublish()
		require.NoError(t, publisher.Stop(context.Background()), "The publisher should stop during cleanup")
	})
	publisher.Start()
	publisher.RunFailed(new(cron.Run))

	select {
	case <-bus.entered:
	case <-time.After(time.Second):
		t.Fatal("The worker must enter the blocking publish")
	}

	for range runEventQueueSize {
		publisher.RunFailed(new(cron.Run))
	}

	assert.Len(t, publisher.queue, runEventQueueSize, "The blocked worker should leave the bounded queue full")

	returned := make(chan struct{})
	go func() {
		publisher.RunFailed(new(cron.Run))
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Offering beyond queue capacity must never block the caller")
	}

	assert.Len(t, publisher.queue, runEventQueueSize, "The overflow event must be dropped")

	bus.releasePublish()
	require.NoError(t, publisher.Stop(context.Background()), "The publisher should stop after the bus is released")
}
