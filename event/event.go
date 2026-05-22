package event

import (
	"context"
	"time"
)

// Event is the minimum contract for any event flowing through the bus.
// EventType must be safe to call on a nil receiver — implementations
// should return a static string literal rather than dereference fields.
// This convention lets SubscribeTyped[T] derive the topic from a zero
// value of T without constructing an instance.
type Event interface {
	EventType() string
}

// Envelope wraps an Event with transport-level metadata. The framework
// populates default values (ID, Source, PublishedAt) at publish time;
// business code may override individual fields via PublishOption.
type Envelope struct {
	// ID is the unique message ID generated on publish. It is stable
	// across retries and is the dedupe key for the Inbox middleware.
	ID string
	// Type mirrors Event.EventType() and drives routing and dispatch.
	Type string
	// Source identifies the producing service; defaults to the
	// application name configured under vef.app.
	Source string
	// OccurredAt is the business time of the event; defaults to now.
	OccurredAt time.Time
	// PublishedAt is when a transport first accepted the frame.
	PublishedAt time.Time
	// TraceID / SpanID propagate W3C tracing context across transports.
	TraceID string
	SpanID  string
	// CorrelationID groups related events; opaque to the framework.
	CorrelationID string
	// Headers carry arbitrary string metadata, forwarded verbatim.
	Headers map[string]string
	// Payload holds the original Event for in-process delivery, or a
	// RawPayload when the message has crossed a process boundary.
	// SubscribeTyped[T] transparently handles both shapes.
	Payload Event
}

// RawPayload carries a JSON-encoded event body. Cross-process
// transports deliver Envelope.Payload as a RawPayload; SubscribeTyped[T]
// decodes it into the concrete T transparently.
type RawPayload struct {
	// Type mirrors Envelope.Type and is what EventType() returns.
	Type string
	// Body is the canonical JSON encoding of the original Event.
	Body []byte
}

// EventType implements Event so RawPayload can sit inside Envelope.Payload.
func (r RawPayload) EventType() string { return r.Type }

// Handler is the untyped consumer signature. A non-nil error signals
// failure; transports with retry semantics will Nack and back off.
type Handler func(ctx context.Context, env Envelope) error

// ErrorSink receives errors from out-of-band publish paths (notably
// WithAsync). The default sink logs at error level; applications
// wanting alerting or metrics can supply their own via fx.Decorate.
type ErrorSink func(err error, env Envelope)

// Unsubscribe detaches a previously registered handler. Safe for
// concurrent use; subsequent calls are no-ops.
type Unsubscribe func()

// Bus is the single entry point for publishing and subscribing. All
// publishing modes are expressed through PublishOption — the interface
// stays narrow on purpose so future modes (e.g. delayed delivery)
// extend the option set rather than the method set.
type Bus interface {
	// Publish sends a single event. The returned error reflects whether
	// the configured transport accepted the frame, not whether
	// downstream handlers succeeded.
	Publish(ctx context.Context, evt Event, opts ...PublishOption) error
	// PublishBatch submits multiple events through the same route
	// resolution pass. Atomicity is transport-specific: transactional
	// transports participate in the caller's transaction, while
	// non-transactional transports may partially accept a batch before
	// returning an error.
	PublishBatch(ctx context.Context, evts []Event, opts ...PublishOption) error
	// Subscribe registers a handler for the given event type. The
	// registration is buffered when the bus has not started and
	// flushed during Bus.Start, so framework modules can subscribe
	// during fx Provide regardless of startup order.
	Subscribe(eventType string, h Handler, opts ...SubscribeOption) (Unsubscribe, error)
}
