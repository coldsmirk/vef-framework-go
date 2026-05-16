package middleware

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
	pubinbox "github.com/coldsmirk/vef-framework-go/event/inbox"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// Inbox is a consume-side ConsumeMiddleware that dedupes deliveries by
// (consumer_group, event_id). It activates only on transports whose
// Capabilities advertise AtLeastOnce so in-memory paths don't pay the
// database round-trip cost.
type Inbox struct {
	repo pubinbox.Repository
}

// NewInbox constructs an Inbox middleware.
func NewInbox(repo pubinbox.Repository) *Inbox {
	return &Inbox{repo: repo}
}

// Name implements ConsumeMiddleware.
func (*Inbox) Name() string { return "inbox" }

// Applies attaches the middleware only when the transport may deliver
// the same message more than once.
func (*Inbox) Applies(caps transport.Capabilities) bool { return caps.AtLeastOnce }

// WrapConsume claims the (group, eventID) slot; on duplicate it
// short-circuits the handler chain with success so the transport Acks
// the message and skips redelivery downstream.
func (m *Inbox) WrapConsume(next pubmw.ConsumeHandler) pubmw.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		group := consumerGroupFromContext(ctx)

		acquired, err := m.repo.TryInsert(ctx, group, env.ID)
		if err != nil {
			return err
		}

		if !acquired {
			// Duplicate delivery — Ack without invoking the handler.
			return nil
		}

		return next(ctx, d, env)
	}
}

type consumerGroupKey struct{}

// WithConsumerGroup stores the consumer group name on ctx so the inbox
// middleware can scope its dedupe key. The framework Bus is responsible
// for populating this value before invoking the consume pipeline.
func WithConsumerGroup(ctx context.Context, group string) context.Context {
	return context.WithValue(ctx, consumerGroupKey{}, group)
}

func consumerGroupFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(consumerGroupKey{}).(string)

	return v
}
