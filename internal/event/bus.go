package event

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	imiddleware "github.com/coldsmirk/vef-framework-go/internal/event/middleware"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// busLogger is the package-wide logger.
var busLogger = logx.Named("event")

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
//
// started / stopped are atomic so the publish hot path can check the
// lifecycle state without taking b.mu. The mutex remains the
// authoritative serialiser for state transitions (Start / Stop) and
// for protecting the mutable pending / active maps.
type Bus struct {
	mu sync.Mutex

	started atomic.Bool
	stopped atomic.Bool

	transports map[string]transport.Transport
	router     *router
	cfg        *config.EventConfig
	appSource  string

	publishMW []middleware.PublishMiddleware
	consumeMW []middleware.ConsumeMiddleware

	pending   []pendingSubscription
	active    map[uint64]activeSubscription
	nextSubID uint64

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

// HasTransactionalRoute implements event.RouteInspector. Returns false
// before Start (no router built yet) or when no transactional transport
// is among the resolved route for eventType.
func (b *Bus) HasTransactionalRoute(eventType string) bool {
	b.mu.Lock()
	r := b.router
	b.mu.Unlock()

	if r == nil {
		return false
	}

	for _, t := range r.Resolve(eventType) {
		if t.Capabilities().Transactional {
			return true
		}
	}

	return false
}

// HasSubscribableTransport implements event.RouteInspector. Returns
// false before Start (no router built yet) or when every transport
// resolving eventType is publish-only — in that case no Subscribe call
// against the route could ever succeed, so callers should fail fast.
func (b *Bus) HasSubscribableTransport(eventType string) bool {
	b.mu.Lock()
	r := b.router
	b.mu.Unlock()

	if r == nil {
		return false
	}

	for _, t := range r.Resolve(eventType) {
		if !t.Capabilities().PublishOnly {
			return true
		}
	}

	return false
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
		active:     make(map[uint64]activeSubscription),
	}
	b.async = newAsyncFanIn(
		cfg.EffectiveAsyncQueueSize(),
		cfg.EffectiveAsyncWorkers(),
		func(ctx context.Context, evt event.Event, opts []event.PublishOption) error {
			return b.publishMany(ctx, []event.Event{evt}, opts)
		},
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
	if b.started.Load() {
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
			cause := fmt.Errorf("event: start transport %s: %w", t.Name(), err)

			rollbackErrs := b.rollbackStartedTransports(ctx, started)

			b.mu.Lock()
			b.router = nil
			b.mu.Unlock()

			return joinStartFailure(cause, rollbackErrs)
		}

		started = append(started, t)
	}

	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.started.Store(true)
	b.mu.Unlock()

	var (
		flushErrs []error
		flushed   []event.Unsubscribe
	)
	for _, p := range pending {
		if p.canceled {
			continue
		}

		unsub, err := b.subscribeNow(p.eventType, p.handler, p.cfg)
		if err != nil {
			busLogger.Errorf("flush subscription %s failed: %v", p.eventType, err)
			flushErrs = append(flushErrs, fmt.Errorf("event: flush subscription %s: %w", p.eventType, err))

			continue
		}

		flushed = append(flushed, unsub)
	}

	if len(flushErrs) > 0 {
		flushErrs = append(flushErrs, b.revertFromFailedFlush(ctx, pending, flushed, started)...)

		return errors.Join(flushErrs...)
	}

	b.async.start()

	return nil
}

// rollbackStartedTransports stops every transport that has already
// been brought up during this Start attempt, returning the slice of
// Stop failures. Stop failures during rollback usually signal resource
// leaks (lingering goroutines, unreleased connections), so callers
// surface them alongside the triggering cause rather than swallowing
// them with a log line — partial visibility hides operational risk.
func (*Bus) rollbackStartedTransports(ctx context.Context, started []transport.Transport) []error {
	var errs []error
	for _, t := range started {
		if stopErr := t.Stop(ctx); stopErr != nil {
			errs = append(errs, fmt.Errorf("event: rollback stop %s: %w", t.Name(), stopErr))
		}
	}

	return errs
}

// revertFromFailedFlush rewinds the bus to its pre-Start state after
// the pending-subscription flush has produced one or more errors.
// Sequence:
//  1. Unsubscribe everything that flushed successfully in this attempt.
//  2. Snapshot and clear b.active while restoring b.pending under the
//     mutex. b.started has already flipped to true earlier in Start,
//     so any concurrent Subscribe call took the subscribeNow path and
//     installed itself into b.active; these stray entries must also
//     be unwound so a future retry starts from a clean slate.
//  3. Unsubscribe the stray entries outside the lock to keep the
//     critical section short.
//  4. Stop every transport that we had already brought up, joining
//     any Stop errors so they reach the caller.
func (b *Bus) revertFromFailedFlush(
	ctx context.Context,
	pending []pendingSubscription,
	flushed []event.Unsubscribe,
	started []transport.Transport,
) []error {
	for _, unsub := range flushed {
		unsub()
	}

	b.mu.Lock()
	straySubs := b.active
	b.active = make(map[uint64]activeSubscription)
	b.pending = pending
	b.started.Store(false)
	b.router = nil
	b.mu.Unlock()

	for _, sub := range straySubs {
		for _, u := range sub.unsubs {
			u()
		}
	}

	return b.rollbackStartedTransports(ctx, started)
}

// joinStartFailure combines the triggering cause with any rollback
// Stop errors. errors.Join with a single non-nil entry wraps the
// argument rather than returning it verbatim, which would interfere
// with callers using errors.Is — fall back to the raw cause when no
// rollback errors occurred.
func joinStartFailure(cause error, rollbackErrs []error) error {
	if len(rollbackErrs) == 0 {
		return cause
	}

	all := make([]error, 0, 1+len(rollbackErrs))
	all = append(all, cause)
	all = append(all, rollbackErrs...)

	return errors.Join(all...)
}

// Stop drains async work, unsubscribes all active subscriptions, and
// stops every transport. Idempotent; no-ops if Start never completed.
func (b *Bus) Stop(ctx context.Context) error {
	if !b.stopped.CompareAndSwap(false, true) {
		return nil
	}

	b.mu.Lock()
	wasStarted := b.started.Load()
	active := b.active
	b.active = nil
	b.mu.Unlock()

	if !wasStarted {
		return nil
	}

	var errs []error
	if err := b.async.shutdown(ctx); err != nil {
		busLogger.Warnf("async shutdown timed out: %v", err)
		errs = append(errs, fmt.Errorf("async shutdown: %w", err))
	}

	for _, sub := range active {
		for _, u := range sub.unsubs {
			u()
		}
	}

	for _, t := range b.transports {
		if err := t.Stop(ctx); err != nil {
			busLogger.Warnf("transport %s stop: %v", t.Name(), err)
			errs = append(errs, fmt.Errorf("transport %s: %w", t.Name(), err))
		}
	}

	return errors.Join(errs...)
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
	// Validate at the entry point so pending subscriptions registered
	// before Start fail immediately rather than blowing up at flush
	// time. The publish path validates symmetrically before encoding.
	if !transport.EventTypePattern.MatchString(eventType) {
		return nil, fmt.Errorf("%w: %q", event.ErrInvalidEventType, eventType)
	}

	cfg := event.ApplySubscribeOptions(opts)

	if b.started.Load() {
		return b.subscribeNow(eventType, h, cfg)
	}

	b.mu.Lock()
	// Re-check under the lock: Start may have flipped started between
	// the atomic load above and acquiring the mutex.
	if b.started.Load() {
		b.mu.Unlock()

		return b.subscribeNow(eventType, h, cfg)
	}

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

func (b *Bus) subscribeNow(eventType string, h event.Handler, cfg event.SubscribeConfig) (event.Unsubscribe, error) {
	transports := b.router.Resolve(eventType)
	if len(transports) == 0 {
		return nil, event.ErrNoRouteMatched
	}

	// Filter publish-only transports (e.g. transactional outbox) so the
	// bus does not attempt to subscribe on them. Subscribers must attach
	// to the downstream sink transport directly; the outbox relay is
	// what forwards records to that sink at delivery time.
	//
	// router.Resolve hands back the rule's internal slice — must allocate
	// a fresh slice here so filtering does not corrupt the route compiled
	// at start-up.
	subscribable := make([]transport.Transport, 0, len(transports))
	for _, t := range transports {
		if !t.Capabilities().PublishOnly {
			subscribable = append(subscribable, t)
		}
	}

	if len(subscribable) == 0 {
		return nil, fmt.Errorf("%w: %q resolves only to publish-only transports", event.ErrNoRouteMatched, eventType)
	}

	transports = subscribable

	// At-least-once transports rely on a stable consumer group: it is
	// the Inbox dedupe key and the Redis Streams XGROUP name. Auto-
	// synthesizing one would reset Redis Streams position on each
	// restart and detach historical dedupe records from new ones.
	// Force callers to pick a meaningful name.
	for _, t := range transports {
		if t.Capabilities().AtLeastOnce && cfg.Group == "" {
			return nil, fmt.Errorf("%w (event=%q, transport=%s)",
				event.ErrGroupRequired, eventType, t.Name())
		}
	}

	effectiveGroup := cfg.Group

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
	subID := b.nextSubID
	b.nextSubID++
	b.active[subID] = activeSubscription{unsubs: unsubs}
	b.mu.Unlock()

	var once sync.Once

	return func() {
		once.Do(func() {
			for _, u := range unsubs {
				u()
			}

			b.mu.Lock()
			delete(b.active, subID)
			b.mu.Unlock()
		})
	}, nil
}

func (b *Bus) buildConsumer(t transport.Transport, group string, h event.Handler) transport.ConsumeFunc {
	caps := t.Capabilities()
	wrapped := middleware.ChainConsume(b.consumeMW, caps, func(ctx context.Context, _ transport.Delivery, env event.Envelope) error {
		return h(ctx, env)
	})

	return func(ctx context.Context, d transport.Delivery) error {
		if group != "" {
			ctx = imiddleware.WithConsumerGroup(ctx, group)
		}

		env := decodeFrame(d.Frame())

		return wrapped(ctx, d, env)
	}
}

func (b *Bus) publishMany(ctx context.Context, evts []event.Event, opts []event.PublishOption) error {
	if len(evts) == 0 {
		return nil
	}

	if !b.started.Load() {
		return event.ErrBusNotStarted
	}

	cfg := event.ApplyPublishOptions(opts)

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
// The detached context survives request cancellation so an enqueued
// publish is not lost when the originating request returns.
func (b *Bus) publishAsync(ctx context.Context, evts []event.Event, opts []event.PublishOption, cfg event.PublishConfig) error {
	if cfg.Tx != nil {
		return event.ErrTxAsyncMutex
	}

	syncOpts := stripAsyncOption(opts)
	detached := context.WithoutCancel(ctx)

	for _, evt := range evts {
		if b.async.Enqueue(asyncJob{ctx: detached, evt: evt, opts: syncOpts}) {
			continue
		}

		if err := b.publishMany(detached, []event.Event{evt}, syncOpts); err != nil {
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
// destination transport. The publish-middleware chain is built once
// per batch and the base handler captures the post-middleware envelope
// into a shared slot — n events trigger one chain build, not n.
func (b *Bus) buildBuckets(ctx context.Context, evts []event.Event, cfg event.PublishConfig) (map[string]*pendingBucket, error) {
	buckets := make(map[string]*pendingBucket)

	var processed event.Envelope

	runMW := middleware.ChainPublish(b.publishMW, func(_ context.Context, e *event.Envelope) error {
		processed = *e

		return nil
	})

	for _, evt := range evts {
		if !transport.EventTypePattern.MatchString(evt.EventType()) {
			return nil, fmt.Errorf("%w: %q", event.ErrInvalidEventType, evt.EventType())
		}

		env := b.buildEnvelope(ctx, evt, cfg)
		if err := runMW(ctx, &env); err != nil {
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
	if imemory.IsQueueFull(err) {
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
