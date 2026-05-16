package memory

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// memoryDelivery wraps a Frame for the consume pipeline. Memory
// transport has no retry queue, so Ack/Nack are bookkeeping no-ops.
type memoryDelivery struct {
	frame   transport.Frame
	attempt int
}

func newDelivery(frame transport.Frame) *memoryDelivery {
	return &memoryDelivery{frame: frame, attempt: 1}
}

// Frame implements transport.Delivery.
func (d *memoryDelivery) Frame() transport.Frame { return d.frame }

// Attempt implements transport.Delivery.
func (d *memoryDelivery) Attempt() int { return d.attempt }

// Ack implements transport.Delivery.
func (*memoryDelivery) Ack(context.Context) error { return nil }

// Nack implements transport.Delivery.
func (*memoryDelivery) Nack(context.Context, time.Duration, error) error { return nil }
