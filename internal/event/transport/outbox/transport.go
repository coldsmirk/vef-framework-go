package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	puboutbox "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ErrSinkNotConfigured indicates the outbox transport was started
// without a sink bound via SetSink. Bus.Start refuses to start in this
// state.
var ErrSinkNotConfigured = errors.New("outbox transport: sink not configured")

// ErrDLQReentry indicates a DLQ-forwarded frame (carrying dlqHeader) is
// being re-published into the outbox, which would create an unbounded
// retry loop. The condition implies a routing misconfiguration where
// the outbox is its own sink.
var ErrDLQReentry = errors.New("outbox: refusing to persist DLQ-forwarded frame (sink loop)")

// ErrInvalidFrameBody indicates a frame body is not valid JSON. The
// outbox stores payloads as JSONB so non-JSON bodies cannot be
// reconstructed by the relay.
var ErrInvalidFrameBody = errors.New("outbox: frame body is not valid JSON")

// Transport is the persistent outbox implementation of
// transport.Transport (and transport.TxTransport). Publishes write
// pending records; a background relay claims them and forwards to the
// configured sink Transport.
//
// Sink binding is two-phase to break the circular fx dependency that
// would otherwise arise from the outbox being a member of the same
// transport registry it consumes: construct with NewTransport, then
// SetSink once the registry is fully populated.
type Transport struct {
	repo puboutbox.Repository
	cfg  puboutbox.Config

	mu   sync.RWMutex
	sink transport.Transport
}

// NewTransport constructs an outbox Transport. The sink must be bound
// via SetSink before Start is called.
func NewTransport(repo puboutbox.Repository, cfg puboutbox.Config) *Transport {
	return &Transport{repo: repo, cfg: cfg}
}

// SetSink installs the downstream sink the relay dispatches to. Safe
// to call once after construction; subsequent calls overwrite the sink.
func (t *Transport) SetSink(sink transport.Transport) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sink = sink
}

// Sink returns the bound sink Transport, or nil when unbound.
func (t *Transport) Sink() transport.Transport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.sink
}

// Name implements transport.Transport.
func (*Transport) Name() string { return puboutbox.Name }

// Capabilities reports outbox semantics: durable, transactional,
// ordered per-key, and at-least-once.
func (*Transport) Capabilities() transport.Capabilities {
	return transport.Capabilities{
		Durable:        true,
		Transactional:  true,
		Ordered:        true,
		AtLeastOnce:    true,
		SupportsGroups: false,
	}
}

// Start verifies the sink is configured. The relay loop is driven by
// the outer cron module rather than this transport so its lifecycle
// stays observable to operators.
func (t *Transport) Start(context.Context) error {
	if t.Sink() == nil {
		return ErrSinkNotConfigured
	}

	return nil
}

// Stop is a no-op for the outbox transport; the cron-managed relay
// stops with the scheduler.
func (*Transport) Stop(context.Context) error { return nil }

// Publish persists frames on the outer (non-transactional) connection.
// Use the framework's TxTransport assertion + WithTx to participate
// in a business transaction.
//
// Frames carrying the DLQ marker header (set by the relay when
// forwarding to a dead-letter topic) are rejected here to defend
// against a misconfigured "sink-back-to-outbox" routing that would
// otherwise create an unbounded retry loop.
func (t *Transport) Publish(ctx context.Context, frames []transport.Frame) error {
	if err := rejectDLQReentry(frames); err != nil {
		return err
	}

	records, err := framesToRecords(frames)
	if err != nil {
		return err
	}

	return t.repo.InsertBatch(ctx, records)
}

// PublishTx implements transport.TxTransport.
func (t *Transport) PublishTx(ctx context.Context, tx orm.DB, frames []transport.Frame) error {
	if err := rejectDLQReentry(frames); err != nil {
		return err
	}

	records, err := framesToRecords(frames)
	if err != nil {
		return err
	}

	return t.repo.InsertBatchTx(ctx, tx, records)
}

func rejectDLQReentry(frames []transport.Frame) error {
	for _, f := range frames {
		if _, ok := f.Headers[dlqHeader]; ok {
			return ErrDLQReentry
		}
	}

	return nil
}

// Subscribe forwards the registration to the sink Transport, so the
// outbox layer remains a transparent persistence shim from the
// consumer's perspective.
func (t *Transport) Subscribe(eventType, group string, fn transport.ConsumeFunc, cfg transport.SubscribeConfig) (transport.Unsubscribe, error) {
	sink := t.Sink()
	if sink == nil {
		return nil, ErrSinkNotConfigured
	}

	return sink.Subscribe(eventType, group, fn, cfg)
}

// framesToRecords converts inbound transport frames into pending outbox
// rows. Headers are JSON-shape-tolerant: nil headers map to a nil
// column rather than an empty object.
func framesToRecords(frames []transport.Frame) ([]puboutbox.Record, error) {
	records := make([]puboutbox.Record, len(frames))
	for i, frame := range frames {
		body := frame.Body
		if !json.Valid(body) {
			return nil, fmt.Errorf("%w: frame %s", ErrInvalidFrameBody, frame.ID)
		}

		records[i] = puboutbox.Record{
			EventID:       frame.ID,
			EventType:     frame.Type,
			Source:        frame.Source,
			TraceID:       frame.TraceID,
			SpanID:        frame.SpanID,
			CorrelationID: frame.CorrelationID,
			Headers:       frame.Headers,
			Payload:       json.RawMessage(body),
			Status:        puboutbox.StatusPending,
			OccurredAt:    timex.DateTime(frame.OccurredAt),
		}
		records[i].ID = id.Generate()
	}

	return records, nil
}
