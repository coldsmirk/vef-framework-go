package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
)

// fakeDelivery is a minimal transport.Delivery for consume-side tests.
type fakeDelivery struct{ attempt int }

func (fakeDelivery) Frame() transport.Frame                           { return transport.Frame{} }
func (d fakeDelivery) Attempt() int                                   { return d.attempt }
func (fakeDelivery) Ack(context.Context) error                        { return nil }
func (fakeDelivery) Nack(context.Context, time.Duration, error) error { return nil }

func TestLogging(t *testing.T) {
	t.Run("Order", func(t *testing.T) {
		require.Equal(t, pubmw.OrderLogging, NewLogging(ilogx.Discard()).Order(), "logging Order must equal OrderLogging")
	})

	t.Run("PublishPassesThrough", func(t *testing.T) {
		sentinel := errors.New("boom")
		h := NewLogging(ilogx.Discard()).WrapPublish(func(context.Context, *event.Envelope) error { return sentinel })
		require.ErrorIs(t, h(context.Background(), &event.Envelope{Type: "x"}), sentinel, "publish errors must pass through unchanged")
	})

	t.Run("ConsumePassesThrough", func(t *testing.T) {
		h := NewLogging(ilogx.Discard()).WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error { return nil })
		require.NoError(t, h(context.Background(), fakeDelivery{attempt: 1}, event.Envelope{Type: "x"}), "consume should pass through")
	})
}
