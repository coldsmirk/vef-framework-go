package txmemory

import (
	"context"
	"slices"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/event/transport/txmemory"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Transport delivers events in-process, holding transactional publishes
// back until the caller's transaction commits.
//
// Fan-out is delegated to a private memory transport instance rather than
// the one in the transport registry: owning it keeps the two routes
// independent and avoids the deferred sink binding the outbox needs to
// break its circular fx dependency.
type Transport struct {
	inner  *imemory.Transport
	logger logx.Logger
}

// New constructs a Transport. cfg configures the private in-process
// transport that performs the delivery. A nil logger is replaced with
// logx.Discard so tests can omit it.
func New(cfg memory.Config, log logx.Logger) *Transport {
	if log == nil {
		log = ilogx.Discard()
	}

	return &Transport{inner: imemory.New(cfg), logger: log}
}

// Name implements transport.Transport.
func (*Transport) Name() string { return txmemory.Name }

// Capabilities reports in-process transactional delivery.
//
// Transactional is true — a publish under WithTx becomes visible iff the
// caller commits — while Durable stays false: the frames live only in
// this process between commit and delivery.
//
// Ordered is false. The inner transport hands frames to a subscription in
// the order it receives them, but transactional publishes are released at
// commit time, so concurrent transactions deliver in commit order rather
// than publish order and no per-key ordering can be promised.
//
// AtLeastOnce is false: there is no redelivery, so subscribers written
// for a production outbox route keep working unchanged (the bus only
// *requires* WithGroup on at-least-once transports, and never rejects a
// group elsewhere), and the Inbox dedupe middleware stays out of the way.
func (*Transport) Capabilities() transport.Capabilities {
	return transport.Capabilities{
		Durable:        false,
		Transactional:  true,
		Ordered:        false,
		AtLeastOnce:    false,
		SupportsGroups: false,
	}
}

// Start implements transport.Transport.
func (t *Transport) Start(ctx context.Context) error { return t.inner.Start(ctx) }

// Stop implements transport.Transport.
func (t *Transport) Stop(ctx context.Context) error { return t.inner.Stop(ctx) }

// Publish delivers frames immediately. Callers outside a transaction get
// the plain in-process semantics of the memory transport.
func (t *Transport) Publish(ctx context.Context, frames []transport.Frame) error {
	return t.inner.Publish(ctx, frames)
}

// Subscribe implements transport.Transport. Group is accepted and ignored,
// as on the memory transport.
func (t *Transport) Subscribe(
	eventType, group string,
	fn transport.ConsumeFunc,
	cfg transport.SubscribeConfig,
) (transport.Unsubscribe, error) {
	return t.inner.Subscribe(eventType, group, fn, cfg)
}

// PublishTx implements transport.TxTransport by deferring delivery to the
// commit of the transaction owning ctx. A rolled-back transaction delivers
// nothing, so a failed business operation cannot emit a phantom event.
//
// The transaction handle is unused: the hook rides the context, which is
// what lets the deferral compose with nested savepoints.
func (t *Transport) PublishTx(ctx context.Context, _ orm.DB, frames []transport.Frame) error {
	// Unlike every other transport, which consumes the batch synchronously,
	// this one retains it past the call. Clone so a caller reusing the
	// backing array cannot alter what is delivered after the commit.
	pending := slices.Clone(frames)

	return orm.OnCommit(ctx, func(ctx context.Context) {
		if err := t.inner.Publish(ctx, pending); err != nil {
			// The transaction has already committed and there is nothing left
			// to roll back, so a failed hand-off is reported, not returned.
			t.logger.Errorf("Post-commit dispatch of %d frame(s) failed: %v", len(pending), err)
		}
	})
}
