// Package transport defines the pluggable delivery contract for the
// event bus. Transports are responsible for accepting outgoing frames
// and pumping incoming frames to consumers; the bus handles routing,
// middleware, and envelope encoding.
package transport

import (
	"context"
	"errors"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrSubscribeUnsupported indicates a transport refuses Subscribe calls.
// Publish-only transports (e.g. transactional outbox) return this so
// the bus can detect the misuse and so the routing layer can filter
// them out during subscription resolution.
var ErrSubscribeUnsupported = errors.New("transport: subscribe is not supported on this transport")

// Frame is the wire-level representation of an event. The bus encodes
// Envelope into Frame on publish and decodes back on consume.
type Frame struct {
	// ID is the unique message identifier, stable across retries.
	ID string
	// Type mirrors Event.EventType() and drives routing & subscription.
	Type string
	// Source identifies the producing service.
	Source string
	// OccurredAt is the business time of the event.
	OccurredAt time.Time
	// PublishedAt is when the transport first accepted the frame.
	PublishedAt time.Time
	// TraceID / SpanID propagate W3C tracing context.
	TraceID string
	SpanID  string
	// CorrelationID groups related events; opaque to the framework.
	CorrelationID string
	// Headers carry arbitrary metadata, forwarded verbatim.
	Headers map[string]string
	// Body is the canonical JSON encoding of the original Event.
	Body []byte
}

// Delivery represents a single in-flight message handed to a consumer.
// Implementations must guarantee Ack and Nack are idempotent.
type Delivery interface {
	// Frame returns the underlying message frame.
	Frame() Frame
	// Attempt returns the 1-based delivery attempt counter.
	Attempt() int
	// Ack marks the delivery as successfully handled.
	Ack(ctx context.Context) error
	// Nack signals handler failure and requests a retry after the
	// supplied delay; the transport may clamp or override the hint.
	Nack(ctx context.Context, retryAfter time.Duration, err error) error
}

// ConsumeFunc is the function shape Transport.Subscribe accepts.
type ConsumeFunc func(ctx context.Context, d Delivery) error

// SubscribeConfig carries the transport-relevant subset of
// event.SubscribeConfig. Kept separate to avoid import cycles.
type SubscribeConfig struct {
	// Group is the consumer group name; transports that
	// SupportsGroups balance deliveries between same-group consumers.
	Group string
	// Concurrency is the desired worker count per subscription.
	Concurrency int
}

// Capabilities advertises the semantic guarantees a Transport offers.
// The bus consults these to apply middleware selectively (e.g. Inbox
// middleware only attaches to AtLeastOnce transports).
type Capabilities struct {
	// Durable means messages survive process restart.
	Durable bool
	// Transactional means the transport implements TxTransport.
	Transactional bool
	// Ordered means messages arrive in publish order within a single
	// consumer for the same key/partition.
	Ordered bool
	// AtLeastOnce means messages may be delivered more than once;
	// consumers must dedupe and Inbox middleware activates.
	AtLeastOnce bool
	// SupportsGroups means SubscribeConfig.Group affects delivery
	// semantics.
	SupportsGroups bool
	// PublishOnly marks transports that accept publishes but cannot
	// deliver to subscribers themselves. The classic example is the
	// transactional outbox: it persists records that a relay later
	// forwards to a downstream sink transport. The bus filters out
	// publish-only transports when resolving Subscribe targets so
	// fan-out routes do not pick them up; subscribers must attach to
	// the sink transport directly.
	PublishOnly bool
}

// Unsubscribe detaches a previously registered consumer. Safe for
// concurrent use; subsequent calls are no-ops.
type Unsubscribe func()

// Transport is the pluggable delivery backend. Implementations must be
// safe for concurrent Publish and Subscribe and must complete Stop
// cleanly when its context deadline expires.
type Transport interface {
	// Name returns the stable identifier referenced from routing config.
	Name() string
	// Capabilities advertises the semantic guarantees of this transport.
	Capabilities() Capabilities
	// Start hooks the transport into the fx lifecycle. Implementations
	// must be idempotent under repeated Start calls.
	Start(ctx context.Context) error
	// Stop drains in-flight work and releases resources. Idempotent.
	Stop(ctx context.Context) error
	// Publish submits frames. Returns once the transport has accepted
	// the frames it can store or buffer; non-transactional transports
	// may partially accept a batch before returning an error.
	Publish(ctx context.Context, frames []Frame) error
	// Subscribe attaches a consumer to the given event type within an
	// optional consumer group. Returns an unsubscribe closure.
	Subscribe(eventType, group string, fn ConsumeFunc, cfg SubscribeConfig) (Unsubscribe, error)
}

// TxTransport is the optional extension implemented by transports that
// participate in caller transactions. A Capabilities.Transactional value
// of true implies the type assertion to TxTransport will succeed.
type TxTransport interface {
	Transport
	// PublishTx submits frames within the supplied transaction. The
	// frames become visible iff the caller commits.
	PublishTx(ctx context.Context, tx orm.DB, frames []Frame) error
}
