package redisstream

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// streamDelivery wraps a Redis Streams message for the consume pipeline.
type streamDelivery struct {
	frame   transport.Frame
	attempt int
	msgID   string
}

// Frame implements transport.Delivery.
func (d *streamDelivery) Frame() transport.Frame { return d.frame }

// Attempt implements transport.Delivery. Redis Streams does not track
// per-message retry counts inside the transport surface; the bus
// middleware chain owns retry budgeting via SubscribeConfig.MaxAttempts.
func (d *streamDelivery) Attempt() int { return d.attempt }

// Ack is a no-op here because the transport.consumerLoop XACKs after
// the handler returns nil. Surfaced for interface conformance and for
// future middlewares that may want to ack early (e.g. after Inbox
// dedupe).
func (*streamDelivery) Ack(context.Context) error { return nil }

// Nack is a no-op; the message simply stays pending and the reaper
// re-claims it after the idle threshold.
func (*streamDelivery) Nack(context.Context, time.Duration, error) error { return nil }
