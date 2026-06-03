package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
)

func TestRecover(t *testing.T) {
	t.Run("Order", func(t *testing.T) {
		require.Equal(t, pubmw.OrderRecover, NewRecover(ilogx.Discard()).Order(), "recover Order must equal OrderRecover")
	})

	t.Run("PanicBecomesError", func(t *testing.T) {
		h := NewRecover(ilogx.Discard()).WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
			panic("kaboom")
		})

		err := h(context.Background(), nil, event.Envelope{Type: "test.created"})
		require.Error(t, err, "a handler panic must be converted into an error")
		require.ErrorIs(t, err, event.ErrHandlerPanic, "the converted error must wrap ErrHandlerPanic")
	})

	t.Run("NoPanicPassesThrough", func(t *testing.T) {
		sentinel := errors.New("normal failure")
		h := NewRecover(ilogx.Discard()).WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
			return sentinel
		})

		require.ErrorIs(t, h(context.Background(), nil, event.Envelope{}), sentinel, "non-panic errors must pass through unchanged")
	})
}
