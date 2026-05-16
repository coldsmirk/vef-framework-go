package event

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	internalmw "github.com/coldsmirk/vef-framework-go/internal/event/middleware"
	memimpl "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// eventTypePattern restricts event type strings to a safe alphabet so
// they can be safely composed into stream keys, DLQ topic names, and
// metrics labels without escaping concerns.
var eventTypePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// busLogger is the package-wide logger.
var busLogger = logx.Named("event")

// errTxAsyncMutex indicates the caller combined WithTx and WithAsync,
// which is contradictory: a transactional publish must complete inside
// the caller's transaction window.
var errTxAsyncMutex = errors.New("event: WithTx and WithAsync are mutually exclusive")

// maxFrameBodyBytes caps the JSON-encoded payload size accepted by the
// bus before encoding. Cross-process transports inherit this limit so a
// hostile or misconfigured publisher cannot push arbitrarily large
// frames through the pipeline.
const maxFrameBodyBytes = 1 << 20 // 1 MiB

// maxHeaderEntries / maxHeaderValueBytes bound the per-envelope header
// map so headers stay metadata-sized rather than payload-sized.
const (
	maxHeaderEntries    = 32
	maxHeaderKeyBytes   = 128
	maxHeaderValueBytes = 1024
)

// Bus is the framework's event Bus implementation. It wires routing,
// per-transport delivery, async fan-in, and middleware composition.
type Bus struct {
	mu sync.Mutex

	started bool
	stopped bool

	transports map[string]transport.Transport
	router     *router
	cfg        *config.EventConfig
	appSource  string

	publishMW []middleware.PublishMiddleware
	consumeMW []middleware.ConsumeMiddleware

	pending []pendingSubscription
	active  []activeSubscription

	async     *asyncFanIn
	errorSink event.ErrorSink
}

type pendingSubscription struct {
	eventType string
	handler   event.Handler
	cfg       event.SubscribeConfig
	canceled  bool
}

type activeSubscription struct {
	unsubs []transport.Unsubscribe
}

// NewBus constructs a Bus from the supplied configuration, transport
// registry, and middleware groups. Subscribe calls are accepted before
// Start; they are flushed during Start once the router is resolved.
func NewBus(
	cfg *config.EventConfig,
	appName string,
	transports []transport.Transport,
	publishMW []middleware.PublishMiddleware,
	consumeMW []middleware.ConsumeMiddleware,
	sink event.ErrorSink,
) *Bus {
	registry := make(map[string]transport.Transport, len(transports))
	for _, t := range transports {
		if t == nil {
			continue
		}

		registry[t.Name()] = t
	}

	b := &Bus{
		transports: registry,
		cfg:        cfg,
		appSource:  appName,
		publishMW:  publishMW,
		consumeMW:  consumeMW,
		errorSink:  sink,
	}
	b.async = newAsyncFanIn(
		cfg.EffectiveAsyncQueueSize(),
		cfg.EffectiveAsyncWorkers(),
		b.publishOne,
		b.reportAsyncError,
	)

	return b
}

// Start hooks the bus into the fx lifecycle. It resolves the router,
// starts every transport, flushes pending subscriptions, then begins
// the async worker pool. If any transport fails to Start, all
// previously-started transports are Stopped so the bus does not leak
// goroutines.
func (b *Bus) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()

		return event.ErrBusAlreadyStarted
	}

	if err := b.cfg.Validate(); err != nil {
		b.mu.Unlock()

		return fmt.Errorf("event: invalid config: %w", err)
	}

	r, err := buildRouter(b.cfg, b.transports)
	if err != nil {
		b.mu.Unlock()

		return fmt.Errorf("event: build router: %w", err)
	}

	b.router = r
	b.mu.Unlock()

	started := make([]transport.Transport, 0, len(b.transports))
	for _, t := range b.transports {
		if err := t.Start(ctx); err != nil {
			// Roll back already-started transports so partial Start
			// failures don't leak goroutines or leave the bus in a
			// half-initialized state.
			for _, ts := range started {
				if stopErr := ts.Stop(ctx); stopErr != nil {
					busLogger.Warnf("rollback stop %s: %v", ts.Name(), stopErr)
				}
			}

			b.mu.Lock()
			b.router = nil
			b.mu.Unlock()

			return fmt.Errorf("event: start transport %s: %w", t.Name(), err)
		}

		started = append(started, t)
	}

	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.started = true
	b.mu.Unlock()

	for _, p := range pending {
		if p.canceled {
			continue
		}

		if _, err := b.subscribeNow(p.eventType, p.handler, p.cfg); err != nil {
			busLogger.Errorf("flush subscription %s failed: %v", p.eventType, err)
		}
	}

	b.async.start()

	return nil
}

// Stop drains async work, unsubscribes all active subscriptions, and
// stops every transport. Idempotent; no-ops if Start never completed.
func (b *Bus) Stop(ctx context.Context) error {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()

		return nil
	}

	b.stopped = true
	wasStarted := b.started
	active := b.active
	b.active = nil
	b.mu.Unlock()

	if wasStarted {
		if err := b.async.shutdown(ctx); err != nil {
			busLogger.Warnf("async shutdown timed out: %v", err)
		}

		for _, sub := range active {
			for _, u := range sub.unsubs {
				u()
			}
		}

		for _, t := range b.transports {
			if err := t.Stop(ctx); err != nil {
				busLogger.Warnf("transport %s stop: %v", t.Name(), err)
			}
		}
	}

	return nil
}

// Publish implements event.Bus.
func (b *Bus) Publish(ctx context.Context, evt event.Event, opts ...event.PublishOption) error {
	return b.publishMany(ctx, []event.Event{evt}, opts)
}

// PublishBatch implements event.Bus.
func (b *Bus) PublishBatch(ctx context.Context, evts []event.Event, opts ...event.PublishOption) error {
	return b.publishMany(ctx, evts, opts)
}

// Subscribe implements event.Bus. When called before Start the
// registration is buffered; the returned Unsubscribe cancels the
// pending registration. After Start the call attaches handlers to all
// matched transports.
func (b *Bus) Subscribe(eventType string, h event.Handler, opts ...event.SubscribeOption) (event.Unsubscribe, error) {
	cfg := event.ApplySubscribeOptions(opts)

	b.mu.Lock()
	if !b.started {
		idx := len(b.pending)
		b.pending = append(b.pending, pendingSubscription{
			eventType: eventType,
			handler:   h,
			cfg:       cfg,
		})
		b.mu.Unlock()

		return func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if idx < len(b.pending) {
				b.pending[idx].canceled = true
			}
		}, nil
	}
	b.mu.Unlock()

	return b.subscribeNow(eventType, h, cfg)
}

func (b *Bus) subscribeNow(eventType string, h event.Handler, cfg event.SubscribeConfig) (event.Unsubscribe, error) {
	transports := b.router.Resolve(eventType)
	if len(transports) == 0 {
		return nil, event.ErrNoRouteMatched
	}

	// Synthesize a per-subscription group when the caller did not
	// supply one and any AtLeastOnce transport is in the route. This
	// ensures the Inbox middleware's dedupe key is scoped to this
	// subscription rather than shared across all anonymous consumers.
	effectiveGroup := cfg.Group
	if effectiveGroup == "" {
		for _, t := range transports {
			if t.Capabilities().AtLeastOnce {
				effectiveGroup = "vef:anon:" + newEventID()

				break
			}
		}
	}

	unsubs := make([]transport.Unsubscribe, 0, len(transports))
	for _, t := range transports {
		consumer := b.buildConsumer(t, effectiveGroup, h)

		unsub, err := t.Subscribe(eventType, effectiveGroup, consumer, transport.SubscribeConfig{
			Group:       effectiveGroup,
			Concurrency: cfg.Concurrency,
		})
		if err != nil {
			for _, u := range unsubs {
				u()
			}

			return nil, fmt.Errorf("event: subscribe %s on %s: %w", eventType, t.Name(), err)
		}

		unsubs = append(unsubs, unsub)
	}

	b.mu.Lock()
	b.active = append(b.active, activeSubscription{unsubs: unsubs})
	b.mu.Unlock()

	return func() {
		for _, u := range unsubs {
			u()
		}
	}, nil
}

func (b *Bus) buildConsumer(t transport.Transport, group string, h event.Handler) transport.ConsumeFunc {
	caps := t.Capabilities()
	wrapped := middleware.ChainConsume(b.consumeMW, caps, func(ctx context.Context, _ transport.Delivery, env event.Envelope) error {
		return h(ctx, env)
	})

	return func(ctx context.Context, d transport.Delivery) error {
		if group != "" {
			ctx = internalmw.WithConsumerGroup(ctx, group)
		}

		env := decodeFrame(d.Frame())

		return wrapped(ctx, d, env)
	}
}

func (b *Bus) publishMany(ctx context.Context, evts []event.Event, opts []event.PublishOption) error {
	if len(evts) == 0 {
		return nil
	}

	cfg := event.ApplyPublishOptions(opts)

	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()

		return event.ErrBusNotStarted
	}
	b.mu.Unlock()

	if cfg.Async {
		return b.publishAsync(ctx, evts, opts, cfg)
	}

	buckets, err := b.buildBuckets(ctx, evts, cfg)
	if err != nil {
		return err
	}

	timeout := b.cfg.EffectivePublishTimeout()
	for _, p := range buckets {
		if err := b.publishBucket(ctx, p.t, p.frames, cfg.Tx, timeout); err != nil {
			return err
		}
	}

	return nil
}

// publishAsync enqueues each event onto the async fan-in, falling back
// to a synchronous publish (with WithAsync stripped) when the queue is
// full so security-critical events are not silently dropped under load.
func (b *Bus) publishAsync(ctx context.Context, evts []event.Event, opts []event.PublishOption, cfg event.PublishConfig) error {
	if cfg.Tx != nil {
		return errTxAsyncMutex
	}

	for _, evt := range evts {
		detached := context.WithoutCancel(ctx)

		if b.async.Enqueue(asyncJob{ctx: detached, evt: evt, opts: opts}) {
			continue
		}

		if err := b.publishSync(detached, evt, stripAsyncOption(opts)); err != nil {
			b.reportAsyncError(fmt.Errorf("%w: fallback sync publish: %w", event.ErrAsyncQueueFull, err),
				event.Envelope{Type: evt.EventType()})
		}
	}

	return nil
}

// pendingBucket holds the frames destined for a single transport during
// a single publish call.
type pendingBucket struct {
	t      transport.Transport
	frames []transport.Frame
}

// buildBuckets validates and encodes each event, applies publish
// middleware, resolves routing, then groups the resulting frames by
// destination transport.
func (b *Bus) buildBuckets(ctx context.Context, evts []event.Event, cfg event.PublishConfig) (map[string]*pendingBucket, error) {
	buckets := make(map[string]*pendingBucket)

	for _, evt := range evts {
		if !eventTypePattern.MatchString(evt.EventType()) {
			return nil, fmt.Errorf("%w: %q", event.ErrInvalidEventType, evt.EventType())
		}

		processed, err := b.runPublishMiddleware(ctx, b.buildEnvelope(ctx, evt, cfg))
		if err != nil {
			return nil, err
		}

		if err := validateHeaders(processed.Headers); err != nil {
			return nil, err
		}

		transports, err := b.routeForPublish(processed.Type, cfg.Tx != nil)
		if err != nil {
			return nil, err
		}

		frame, err := encodeFrame(processed)
		if err != nil {
			return nil, err
		}

		if len(frame.Body) > maxFrameBodyBytes {
			return nil, fmt.Errorf("%w: frame body %d bytes exceeds %d (type=%s)",
				event.ErrPayloadTooLarge, len(frame.Body), maxFrameBodyBytes, frame.Type)
		}

		for _, t := range transports {
			entry, ok := buckets[t.Name()]
			if !ok {
				entry = &pendingBucket{t: t}
				buckets[t.Name()] = entry
			}

			entry.frames = append(entry.frames, frame)
		}
	}

	return buckets, nil
}

// runPublishMiddleware runs the publish-side middleware chain and
// returns the resulting envelope. The middleware may mutate Headers
// (e.g. tracing injects traceparent), so we capture the final state
// rather than relying on the caller's pointer.
func (b *Bus) runPublishMiddleware(ctx context.Context, env event.Envelope) (event.Envelope, error) {
	var processed event.Envelope

	run := middleware.ChainPublish(b.publishMW, func(_ context.Context, e *event.Envelope) error {
		processed = *e

		return nil
	})
	if err := run(ctx, &env); err != nil {
		return event.Envelope{}, err
	}

	return processed, nil
}

// routeForPublish returns the transports for the supplied event type,
// narrowing to transactional transports when the caller opted into a
// shared transaction.
func (b *Bus) routeForPublish(eventType string, requireTx bool) ([]transport.Transport, error) {
	transports := b.router.Resolve(eventType)
	if len(transports) == 0 {
		return nil, fmt.Errorf("%w: %s", event.ErrNoRouteMatched, eventType)
	}

	if !requireTx {
		return transports, nil
	}

	txCapable := make([]transport.Transport, 0, len(transports))
	for _, t := range transports {
		if t.Capabilities().Transactional {
			txCapable = append(txCapable, t)
		}
	}

	if len(txCapable) == 0 {
		return nil, fmt.Errorf("%w: %s", event.ErrTxRequired, eventType)
	}

	return txCapable, nil
}

func (b *Bus) publishBucket(ctx context.Context, t transport.Transport, frames []transport.Frame, tx orm.DB, timeout time.Duration) error {
	pubCtx := ctx

	var cancel context.CancelFunc
	if timeout > 0 {
		pubCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if tx != nil {
		txT, ok := t.(transport.TxTransport)
		if !ok {
			return fmt.Errorf("%w: %s", event.ErrTxRequired, t.Name())
		}

		if err := txT.PublishTx(pubCtx, tx, frames); err != nil {
			return fmt.Errorf("event: publish-tx on %s: %w", t.Name(), err)
		}

		return nil
	}

	if err := t.Publish(pubCtx, frames); err != nil {
		return b.translateTransportError(err, t)
	}

	return nil
}

// publishOne is the async worker's entry point. Worker-driven calls
// must not loop back through the async queue; the WithAsync flag is
// stripped here as a defensive measure (the job ctor already drops it
// when falling back synchronously from a full queue).
func (b *Bus) publishOne(ctx context.Context, evt event.Event, opts ...event.PublishOption) error {
	return b.publishSync(ctx, evt, stripAsyncOption(opts))
}

// publishSync is the synchronous publish path shared by the worker and
// the async-queue-full fallback. It bypasses the async branch entirely.
func (b *Bus) publishSync(ctx context.Context, evt event.Event, opts []event.PublishOption) error {
	return b.publishMany(ctx, []event.Event{evt}, opts)
}

// stripAsyncOption returns opts with any WithAsync option removed. It
// guards against the async-fallback path re-entering the async branch
// and spinning into infinite recursion when the queue is full.
func stripAsyncOption(opts []event.PublishOption) []event.PublishOption {
	if len(opts) == 0 {
		return opts
	}

	out := make([]event.PublishOption, 0, len(opts))
	for _, opt := range opts {
		var probe event.PublishConfig
		opt(&probe)

		if probe.Async {
			continue
		}

		out = append(out, opt)
	}

	return out
}

func (b *Bus) buildEnvelope(ctx context.Context, evt event.Event, cfg event.PublishConfig) event.Envelope {
	now := time.Now()

	occurred := cfg.OccurredAt
	if occurred.IsZero() {
		occurred = now
	}

	source := cfg.Source
	if source == "" {
		source = b.appSource
	}

	correlationID := cfg.CorrelationID
	if correlationID == "" {
		// Inherit the per-request trace ID by default so any downstream
		// subscriber can correlate events with the originating HTTP/RPC
		// request. Explicit WithCorrelationID still wins.
		//
		// Privacy note: CorrelationID is part of the envelope, so it
		// crosses every transport boundary (in-memory, outbox, Redis
		// stream, …). For setups where RequestID is sensitive — e.g.
		// when it doubles as a user-session correlator — register a
		// publish middleware that strips Envelope.CorrelationID before
		// the persistent transport sees it (see event/middleware).
		correlationID = contextx.RequestID(ctx)
	}

	return event.Envelope{
		ID:            newEventID(),
		Type:          evt.EventType(),
		Source:        source,
		OccurredAt:    occurred,
		PublishedAt:   now,
		CorrelationID: correlationID,
		Headers:       cfg.Headers,
		Payload:       evt,
	}
}

func (b *Bus) reportAsyncError(err error, env event.Envelope) {
	if b.errorSink != nil {
		b.errorSink(err, env)

		return
	}

	busLogger.Errorf("async publish failed (type=%s): %v", env.Type, err)
}

func (*Bus) translateTransportError(err error, t transport.Transport) error {
	if memimpl.IsQueueFull(err) {
		return fmt.Errorf("%w: %s", event.ErrQueueFull, t.Name())
	}

	return fmt.Errorf("event: publish on %s: %w", t.Name(), err)
}

// validateHeaders enforces a small upper bound on per-envelope header
// volume so headers stay metadata-sized. The limits are intentionally
// generous for legitimate use yet small enough to make abusive payloads
// fail fast at the publish boundary.
func validateHeaders(h map[string]string) error {
	if len(h) > maxHeaderEntries {
		return fmt.Errorf("%w: %d headers exceeds limit %d",
			event.ErrPayloadTooLarge, len(h), maxHeaderEntries)
	}

	for k, v := range h {
		if len(k) > maxHeaderKeyBytes {
			return fmt.Errorf("%w: header key %q length %d exceeds limit %d",
				event.ErrPayloadTooLarge, k, len(k), maxHeaderKeyBytes)
		}

		if len(v) > maxHeaderValueBytes {
			return fmt.Errorf("%w: header %q value length %d exceeds limit %d",
				event.ErrPayloadTooLarge, k, len(v), maxHeaderValueBytes)
		}
	}

	return nil
}
