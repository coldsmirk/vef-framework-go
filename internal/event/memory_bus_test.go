package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

// TestMemoryEventBusBasicPublishSubscribe tests MemoryEventBus basic publish subscribe scenarios.
func TestMemoryEventBusBasicPublishSubscribe(t *testing.T) {
	t.Run("SingleSubscriberReceivesEvent", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			receivedEvent event.Event
			wg            sync.WaitGroup
		)

		wg.Add(1)

		unsubscribe := bus.Subscribe("user.created", func(_ context.Context, evt event.Event) {
			receivedEvent = evt

			wg.Done()
		})
		defer unsubscribe()

		testEvent := event.NewBaseEvent("user.created", event.WithSource("test-service"))
		bus.Publish(testEvent)

		// Wait for event delivery with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			require.NotNil(t, receivedEvent, "Received event should not be nil")
			assert.Equal(t, "user.created", receivedEvent.Type(), "Event type should match")
			assert.Equal(t, "test-service", receivedEvent.Source(), "Event source should match")
			assert.Equal(t, testEvent.ID(), receivedEvent.ID(), "Event ID should match")
			t.Logf("✓ Event delivered - Type: %s, Source: %s, ID: %s",
				receivedEvent.Type(), receivedEvent.Source(), receivedEvent.ID())

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for event delivery")
		}
	})

	t.Run("MultipleSubscribersReceiveSameEvent", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			receivedEvents []event.Event
			mu             sync.Mutex
			wg             sync.WaitGroup
		)

		subscriberCount := 3
		wg.Add(subscriberCount)

		// Create multiple subscribers
		var unsubscribers []event.UnsubscribeFunc
		for range subscriberCount {
			unsub := bus.Subscribe("order.placed", func(_ context.Context, evt event.Event) {
				mu.Lock()

				receivedEvents = append(receivedEvents, evt)

				mu.Unlock()
				wg.Done()
			})
			unsubscribers = append(unsubscribers, unsub)
		}

		defer func() {
			for _, unsub := range unsubscribers {
				unsub()
			}
		}()

		testEvent := event.NewBaseEvent("order.placed",
			event.WithSource("order-service"),
			event.WithMeta("orderId", "12345"),
		)
		bus.Publish(testEvent)

		// Wait for all subscribers to receive the event
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			mu.Lock()
			assert.Equal(t, subscriberCount, len(receivedEvents), "All subscribers should receive the event")

			for i, evt := range receivedEvents {
				assert.Equal(t, "order.placed", evt.Type(), "Event type should match for subscriber %d", i)
				assert.Equal(t, "order-service", evt.Source(), "Event source should match for subscriber %d", i)
				assert.Equal(t, testEvent.ID(), evt.ID(), "Event ID should match for subscriber %d", i)
			}

			mu.Unlock()

			t.Logf("✓ Event delivered to %d subscribers - Type: %s, Source: %s",
				subscriberCount, testEvent.Type(), testEvent.Source())

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for event delivery to all subscribers")
		}
	})

	t.Run("SubscribersForDifferentEventTypes", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			userEvents  []event.Event
			orderEvents []event.Event
			mu          sync.Mutex
			wg          sync.WaitGroup
		)

		wg.Add(2) // Expecting 2 events

		unsubUser := bus.Subscribe("user.registered", func(_ context.Context, evt event.Event) {
			mu.Lock()

			userEvents = append(userEvents, evt)

			mu.Unlock()
			wg.Done()
		})
		defer unsubUser()

		unsubOrder := bus.Subscribe("order.created", func(_ context.Context, evt event.Event) {
			mu.Lock()

			orderEvents = append(orderEvents, evt)

			mu.Unlock()
			wg.Done()
		})
		defer unsubOrder()

		// Publish events of different types
		userEvent := event.NewBaseEvent("user.registered")
		orderEvent := event.NewBaseEvent("order.created")

		bus.Publish(userEvent)
		bus.Publish(orderEvent)

		// Wait for both events
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			mu.Lock()
			assert.Equal(t, 1, len(userEvents), "Should receive exactly one user event")
			assert.Equal(t, 1, len(orderEvents), "Should receive exactly one order event")
			assert.Equal(t, "user.registered", userEvents[0].Type(), "User event type should match")
			assert.Equal(t, "order.created", orderEvents[0].Type(), "Order event type should match")
			mu.Unlock()

			t.Logf("✓ Events delivered correctly - User: %s, Order: %s",
				userEvents[0].Type(), orderEvents[0].Type())

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for events")
		}
	})
}

// TestMemoryEventBusUnsubscribe tests MemoryEventBus unsubscribe scenarios.
func TestMemoryEventBusUnsubscribe(t *testing.T) {
	t.Run("UnsubscribePreventsFurtherEventDelivery", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			eventCount int
			mu         sync.Mutex
		)

		unsubscribe := bus.Subscribe("payment.processed", func(context.Context, event.Event) {
			mu.Lock()

			eventCount++

			mu.Unlock()
		})

		// First event should be delivered
		event1 := event.NewBaseEvent("payment.processed")
		bus.Publish(event1)

		// Give some time for delivery
		time.Sleep(10 * time.Millisecond)

		mu.Lock()

		firstCount := eventCount

		mu.Unlock()
		t.Logf("Events received before unsubscribe: %d", firstCount)

		// Unsubscribe
		unsubscribe()

		// Second event should not be delivered
		event2 := event.NewBaseEvent("payment.processed")
		bus.Publish(event2)

		// Give some time for potential delivery
		time.Sleep(10 * time.Millisecond)

		mu.Lock()

		finalCount := eventCount

		mu.Unlock()

		assert.Equal(t, 1, finalCount, "Should only receive the first event")
		t.Logf("✓ Unsubscribe successful - Events before: %d, after: %d", firstCount, finalCount)
	})

	t.Run("UnsubscribeOneOfMultipleSubscribers", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			subscriber1Count int
			subscriber2Count int
			mu               sync.Mutex
		)

		// First subscriber
		unsubscribe1 := bus.Subscribe("notification.sent", func(context.Context, event.Event) {
			mu.Lock()

			subscriber1Count++

			mu.Unlock()
		})

		// Second subscriber
		unsubscribe2 := bus.Subscribe("notification.sent", func(context.Context, event.Event) {
			mu.Lock()

			subscriber2Count++

			mu.Unlock()
		})
		defer unsubscribe2()

		// Publish first event - both should receive
		event1 := event.NewBaseEvent("notification.sent")
		bus.Publish(event1)
		time.Sleep(10 * time.Millisecond)

		mu.Lock()

		count1After1 := subscriber1Count
		count2After1 := subscriber2Count

		mu.Unlock()

		t.Logf("After first event - Subscriber1: %d, Subscriber2: %d", count1After1, count2After1)

		// Unsubscribe first subscriber
		unsubscribe1()

		// Publish second event - only second subscriber should receive
		event2 := event.NewBaseEvent("notification.sent")
		bus.Publish(event2)
		time.Sleep(10 * time.Millisecond)

		mu.Lock()

		count1Final := subscriber1Count
		count2Final := subscriber2Count

		mu.Unlock()

		assert.Equal(t, 1, count1Final, "Subscriber1 should only receive first event")
		assert.Equal(t, 2, count2Final, "Subscriber2 should receive both events")
		t.Logf("✓ Partial unsubscribe successful - Subscriber1: %d, Subscriber2: %d", count1Final, count2Final)
	})

	t.Run("UnsubscribeFunctionIsIdempotent", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			eventCount int
			mu         sync.Mutex
		)

		unsubscribe := bus.Subscribe("test.event", func(context.Context, event.Event) {
			mu.Lock()

			eventCount++

			mu.Unlock()
		})

		// Call unsubscribe multiple times - should not panic
		unsubscribe()
		unsubscribe()
		unsubscribe()
		t.Log("Unsubscribe called 3 times without panic")

		// Event should not be delivered
		testEvent := event.NewBaseEvent("test.event")
		bus.Publish(testEvent)
		time.Sleep(10 * time.Millisecond)

		mu.Lock()

		finalCount := eventCount

		mu.Unlock()

		assert.Equal(t, 0, finalCount, "No events should be received after unsubscribe")
		t.Logf("✓ Idempotent unsubscribe verified - Event count: %d", finalCount)
	})
}

// TestMemoryEventBusLifecycle tests MemoryEventBus lifecycle scenarios.
func TestMemoryEventBusLifecycle(t *testing.T) {
	t.Run("StartAndShutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		bus := &MemoryBus{
			middlewares: []event.Middleware{},
			subscribers: make(map[string]map[string]*subscription),
			eventCh:     make(chan event.Event, 1000),
			ctx:         ctx,
			cancel:      cancel,
		}

		// Start the bus
		err := bus.Start()
		require.NoError(t, err, "Bus should start successfully")
		t.Log("Bus started successfully")

		// Verify it's started
		assert.True(t, bus.started, "Bus should be marked as started")

		// Try to start again - should return error
		err = bus.Start()
		assert.Error(t, err, "Starting an already started bus should return error")
		assert.Contains(t, err.Error(), "already started", "Error should indicate bus is already started")
		t.Log("Correctly prevented double start")

		// Shutdown the bus
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		err = bus.Shutdown(shutdownCtx)
		require.NoError(t, err, "Bus should shutdown successfully")
		t.Log("✓ Bus lifecycle completed - Start → Shutdown")
	})

	t.Run("ShutdownWithoutStart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		bus := &MemoryBus{
			middlewares: []event.Middleware{},
			subscribers: make(map[string]map[string]*subscription),
			eventCh:     make(chan event.Event, 1000),
			ctx:         ctx,
			cancel:      cancel,
		}

		// Shutdown without starting - should not error
		err := bus.Shutdown(context.Background())
		assert.NoError(t, err, "Shutdown without start should not return error")
		t.Log("✓ Graceful shutdown without start verified")
	})

	t.Run("EventsAreProcessedAfterStart", func(t *testing.T) {
		bus := createTestEventBus(t)

		var (
			receivedEvent event.Event
			wg            sync.WaitGroup
		)

		wg.Add(1)

		unsubscribe := bus.Subscribe("lifecycle.test", func(_ context.Context, evt event.Event) {
			receivedEvent = evt

			wg.Done()
		})
		defer unsubscribe()

		testEvent := event.NewBaseEvent("lifecycle.test")
		bus.Publish(testEvent)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			assert.Equal(t, testEvent.ID(), receivedEvent.ID(), "Event ID should match")
			t.Logf("✓ Event processed after start - ID: %s", receivedEvent.ID())

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for event after start")
		}
	})
}

// TestMemoryEventBusMiddleware tests MemoryEventBus middleware scenarios.
func TestMemoryEventBusMiddleware(t *testing.T) {
	t.Run("MiddlewareProcessesEvents", func(t *testing.T) {
		var (
			processedEvents []event.Event
			mu              sync.Mutex
		)

		middleware := &TestMiddleware{
			processFunc: func(ctx context.Context, evt event.Event, next event.MiddlewareFunc) error {
				mu.Lock()

				processedEvents = append(processedEvents, evt)

				mu.Unlock()

				return next(ctx, evt)
			},
		}

		bus := createTestEventBusWithMiddleware(t, []event.Middleware{middleware})

		var (
			receivedEvent event.Event
			wg            sync.WaitGroup
		)

		wg.Add(1)

		unsubscribe := bus.Subscribe("middleware.test", func(_ context.Context, evt event.Event) {
			receivedEvent = evt

			wg.Done()
		})
		defer unsubscribe()

		testEvent := event.NewBaseEvent("middleware.test")
		bus.Publish(testEvent)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			mu.Lock()

			processedCount := len(processedEvents)
			processedID := processedEvents[0].ID()

			mu.Unlock()

			assert.Equal(t, 1, processedCount, "Middleware should process exactly one event")
			assert.Equal(t, testEvent.ID(), processedID, "Processed event ID should match")
			assert.Equal(t, testEvent.ID(), receivedEvent.ID(), "Received event ID should match")
			t.Logf("✓ Middleware processed event - ID: %s", processedID)

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for middleware processing")
		}
	})

	t.Run("MiddlewareChainProcessesInOrder", func(t *testing.T) {
		var (
			processingOrder []string
			mu              sync.Mutex
		)

		middleware1 := &TestMiddleware{
			processFunc: func(ctx context.Context, evt event.Event, next event.MiddlewareFunc) error {
				mu.Lock()

				processingOrder = append(processingOrder, "middleware1")

				mu.Unlock()

				return next(ctx, evt)
			},
		}

		middleware2 := &TestMiddleware{
			processFunc: func(ctx context.Context, evt event.Event, next event.MiddlewareFunc) error {
				mu.Lock()

				processingOrder = append(processingOrder, "middleware2")

				mu.Unlock()

				return next(ctx, evt)
			},
		}

		bus := createTestEventBusWithMiddleware(t, []event.Middleware{middleware1, middleware2})

		var wg sync.WaitGroup
		wg.Add(1)

		unsubscribe := bus.Subscribe("chain.test", func(context.Context, event.Event) {
			wg.Done()
		})
		defer unsubscribe()

		testEvent := event.NewBaseEvent("chain.test")
		bus.Publish(testEvent)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			mu.Lock()

			actualOrder := make([]string, len(processingOrder))
			copy(actualOrder, processingOrder)
			mu.Unlock()

			expectedOrder := []string{"middleware1", "middleware2"}
			assert.Equal(t, expectedOrder, actualOrder, "Middleware should process in registration order")
			t.Logf("✓ Middleware chain processed in order: %v", actualOrder)

		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "Timeout waiting for middleware chain processing")
		}
	})
}

// TestMemoryEventBusConcurrency tests MemoryEventBus concurrency scenarios.
func TestMemoryEventBusConcurrency(t *testing.T) {
	t.Run("ConcurrentPublishAndSubscribe", func(t *testing.T) {
		bus := createTestEventBus(t)

		const (
			numPublishers      = 10
			numSubscribers     = 5
			eventsPerPublisher = 20
		)

		var (
			totalReceived int
			mu            sync.Mutex
			wg            sync.WaitGroup
		)

		// Create subscribers
		var unsubscribers []event.UnsubscribeFunc
		for range numSubscribers {
			wg.Add(eventsPerPublisher * numPublishers) // Each subscriber should receive all events

			unsub := bus.Subscribe("concurrent.test", func(context.Context, event.Event) {
				mu.Lock()

				totalReceived++

				mu.Unlock()
				wg.Done()
			})
			unsubscribers = append(unsubscribers, unsub)
		}

		defer func() {
			for _, unsub := range unsubscribers {
				unsub()
			}
		}()

		// Create publishers
		for i := range numPublishers {
			go func(publisherID int) {
				for j := range eventsPerPublisher {
					evt := event.NewBaseEvent("concurrent.test",
						event.WithMeta("publisherID", string(rune(publisherID+'0'))),
						event.WithMeta("eventNum", string(rune(j+'0'))),
					)
					bus.Publish(evt)
				}
			}(i)
		}

		// Wait for all events to be processed
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			expectedTotal := numPublishers * eventsPerPublisher * numSubscribers

			mu.Lock()

			actualTotal := totalReceived

			mu.Unlock()

			assert.Equal(t, expectedTotal, actualTotal, "All events should be received by all subscribers")
			t.Logf("✓ Concurrent processing verified - Publishers: %d, Subscribers: %d, Events/Publisher: %d, Total received: %d",
				numPublishers, numSubscribers, eventsPerPublisher, actualTotal)

		case <-time.After(5 * time.Second):
			require.Fail(t, "Timeout waiting for concurrent event processing")
		}
	})

	t.Run("ConcurrentSubscribeAndUnsubscribe", func(t *testing.T) {
		bus := createTestEventBus(t)

		const numRoutines = 50

		var wg sync.WaitGroup

		// Concurrently subscribe and unsubscribe
		for range numRoutines {
			wg.Go(func() {
				unsubscribe := bus.Subscribe("concurrent.unsub.test", func(context.Context, event.Event) {
					// Do nothing
				})

				// Immediately unsubscribe
				unsubscribe()
			})
		}

		// Wait for all routines to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Logf("✓ Concurrent subscribe/unsubscribe verified - %d routines completed without deadlock or panic", numRoutines)

		case <-time.After(5 * time.Second):
			require.Fail(t, "Timeout during concurrent subscribe/unsubscribe")
		}
	})
}

// Helper functions and test utilities

// TestMiddleware implements the Middleware interface for testing.
type TestMiddleware struct {
	processFunc func(ctx context.Context, event event.Event, next event.MiddlewareFunc) error
}

func (m *TestMiddleware) Process(ctx context.Context, evt event.Event, next event.MiddlewareFunc) error {
	return m.processFunc(ctx, evt, next)
}

// createTestEventBus creates a memory event bus for testing.
func createTestEventBus(t *testing.T) event.Bus {
	return createTestEventBusWithMiddleware(t, []event.Middleware{})
}

// createTestEventBusWithMiddleware creates a memory event bus with custom middleware for testing.
func createTestEventBusWithMiddleware(t *testing.T, middlewares []event.Middleware) event.Bus {
	bus := NewMemoryBus(middlewares)

	err := bus.Start()
	require.NoError(t, err, "Should not return error")

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = bus.Shutdown(shutdownCtx)
	})

	return bus
}
