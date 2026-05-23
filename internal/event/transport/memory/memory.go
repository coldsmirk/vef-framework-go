package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
)

// ErrBusStopped indicates Publish was called after Stop. Distinct from
// the framework-level ErrQueueFull and surfaces only on this transport.
var ErrBusStopped = errors.New("memory transport: stopped")

// Transport is the in-process implementation of transport.Transport.
type Transport struct {
	cfg memory.Config

	// ctx is the transport-scoped lifecycle context. Stop cancels it so
	// in-flight consumer goroutines observe the shutdown signal in
	// addition to the per-subscription stopCh. Without this, handlers
	// blocking on downstream I/O would only see the cancellation when
	// the per-subscription stopCh closed — which is fine for unsubscribe
	// but not for transport Stop where the bus wants every handler to
	// unwind on the supplied shutdown deadline.
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.RWMutex
	subs    map[string]map[string]*subscription // eventType → subID → sub
	stopped atomic.Bool
}

type subscription struct {
	id          string
	eventType   string
	consume     transport.ConsumeFunc
	queue       chan transport.Frame
	workers     int
	wg          sync.WaitGroup
	stopOnce    sync.Once
	stopCh      chan struct{}
	fullPolicy  memory.FullPolicy
	publishWait time.Duration

	// ctx carries the transport's lifecycle. deliver hands it to the
	// consume callback so handlers see Done on transport Stop.
	ctx context.Context
}

// New constructs a memory Transport with the given config.
func New(cfg memory.Config) *Transport {
	ctx, cancel := context.WithCancel(context.Background())

	return &Transport{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		subs:   make(map[string]map[string]*subscription),
	}
}

// Name implements transport.Transport.
func (*Transport) Name() string { return memory.Name }

// Capabilities reports the semantic guarantees of the memory transport.
func (*Transport) Capabilities() transport.Capabilities {
	return transport.Capabilities{
		Durable:        false,
		Transactional:  false,
		Ordered:        true,
		AtLeastOnce:    false,
		SupportsGroups: false,
	}
}

// Start is a no-op for the memory transport.
func (*Transport) Start(context.Context) error { return nil }

// Stop drains all subscriptions and rejects further publishes.
func (t *Transport) Stop(ctx context.Context) error {
	if !t.stopped.CompareAndSwap(false, true) {
		return nil
	}

	// Cancel the transport-scoped context before draining so any handler
	// currently inside its consume callback observes the shutdown via
	// ctx.Done() rather than only on the next loop iteration.
	t.cancel()

	t.mu.Lock()

	subs := make([]*subscription, 0)
	for _, byType := range t.subs {
		for _, s := range byType {
			subs = append(subs, s)
		}
	}

	t.subs = make(map[string]map[string]*subscription)
	t.mu.Unlock()

	done := make(chan struct{})
	go func() {
		for _, s := range subs {
			s.stop()
		}

		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Publish fans the frames out to all matching subscriptions according
// to the configured FullPolicy.
func (t *Transport) Publish(ctx context.Context, frames []transport.Frame) error {
	if t.stopped.Load() {
		return ErrBusStopped
	}

	for _, frame := range frames {
		t.mu.RLock()
		byType := t.subs[frame.Type]

		targets := make([]*subscription, 0, len(byType))
		for _, s := range byType {
			targets = append(targets, s)
		}

		t.mu.RUnlock()

		for _, sub := range targets {
			if err := sub.enqueue(ctx, frame); err != nil {
				return err
			}
		}
	}

	return nil
}

// Subscribe registers a consumer for the given event type. Group is
// ignored — the memory transport does not support consumer groups.
func (t *Transport) Subscribe(eventType, _ string, fn transport.ConsumeFunc, cfg transport.SubscribeConfig) (transport.Unsubscribe, error) {
	if t.stopped.Load() {
		return nil, ErrBusStopped
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	sub := &subscription{
		id:          newSubID(),
		eventType:   eventType,
		consume:     fn,
		queue:       make(chan transport.Frame, t.cfg.EffectiveQueueSize()),
		workers:     concurrency,
		stopCh:      make(chan struct{}),
		fullPolicy:  t.cfg.EffectiveFullPolicy(),
		publishWait: t.cfg.PublishTimeout,
		ctx:         t.ctx,
	}

	t.mu.Lock()
	if t.subs[eventType] == nil {
		t.subs[eventType] = make(map[string]*subscription)
	}

	t.subs[eventType][sub.id] = sub
	t.mu.Unlock()

	sub.start()

	return func() {
		t.mu.Lock()
		if byType, ok := t.subs[eventType]; ok {
			delete(byType, sub.id)

			if len(byType) == 0 {
				delete(t.subs, eventType)
			}
		}
		t.mu.Unlock()
		sub.stop()
	}, nil
}

func (s *subscription) start() {
	for range s.workers {
		s.wg.Add(1)
		go s.loop()
	}
}

func (s *subscription) loop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ctx.Done():
			return
		case frame, ok := <-s.queue:
			if !ok {
				return
			}

			s.deliver(frame)
		}
	}
}

func (s *subscription) deliver(frame transport.Frame) {
	d := newDelivery(frame)
	// Memory transport ignores Ack/Nack outcomes (no retry semantics);
	// the bus's consume middleware (recover, logging) handles errors.
	// The transport-scoped context carries Stop cancellation so
	// handlers blocking on downstream I/O can unwind cleanly.
	_ = s.consume(s.ctx, d)
}

func (s *subscription) enqueue(ctx context.Context, frame transport.Frame) error {
	switch s.fullPolicy {
	case memory.FullPolicyBlock:
		if s.publishWait > 0 {
			t := time.NewTimer(s.publishWait)
			defer t.Stop()

			select {
			case s.queue <- frame:
				return nil
			case <-t.C:
				return errQueueFull
			case <-ctx.Done():
				return ctx.Err()
			case <-s.stopCh:
				return ErrBusStopped
			}
		}

		select {
		case s.queue <- frame:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return ErrBusStopped
		}

	case memory.FullPolicyDropOldest:
		for {
			select {
			case <-s.stopCh:
				return ErrBusStopped
			case <-ctx.Done():
				return ctx.Err()
			case s.queue <- frame:
				return nil
			default:
				// Evict oldest then retry. The stopCh / ctx checks above
				// prevent the loop from spinning if no consumer is left
				// to drain the queue.
				select {
				case <-s.queue:
				default:
				}
			}
		}

	default: // FullPolicyError
		select {
		case s.queue <- frame:
			return nil
		default:
			return errQueueFull
		}
	}
}

func (s *subscription) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

var errQueueFull = errors.New("memory transport: queue full")

// IsQueueFull reports whether an error from Publish indicates a full
// subscription queue, suitable for callers that want to translate it
// to event.ErrQueueFull.
func IsQueueFull(err error) bool { return errors.Is(err, errQueueFull) }
