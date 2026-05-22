package middleware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/inbox"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var inboxLogger = logx.Named("event:inbox")

// Inbox is a consume-side ConsumeMiddleware that dedupes deliveries by
// (consumer_group, event_id). It activates only on transports whose
// Capabilities advertise AtLeastOnce so in-memory paths don't pay the
// database round-trip cost. Handlers must still make their business side
// effects idempotent: if processing exceeds the lease and the transport
// redelivers, another worker may acquire and run the same event.
type Inbox struct {
	repo            inbox.Repository
	processingLease time.Duration
}

// NewInbox constructs an Inbox middleware.
func NewInbox(repo inbox.Repository, processingLease time.Duration) *Inbox {
	return &Inbox{repo: repo, processingLease: processingLease}
}

// Name implements ConsumeMiddleware.
func (*Inbox) Name() string { return "inbox" }

// Applies attaches the middleware only when the transport may deliver
// the same message more than once.
func (*Inbox) Applies(caps transport.Capabilities) bool { return caps.AtLeastOnce }

// WrapConsume claims the (group, eventID) slot; on duplicate it
// short-circuits the handler chain with success so the transport Acks
// the message and skips redelivery downstream.
func (m *Inbox) WrapConsume(next middleware.ConsumeHandler) middleware.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) (err error) {
		group := consumerGroupFromContext(ctx)

		lockUntil := timex.Now().Add(m.processingLease)

		claim, lockID, err := m.repo.Acquire(ctx, group, env.ID, lockUntil)
		if err != nil {
			return err
		}

		switch claim {
		case inbox.AcquireResultAcquired:
		case inbox.AcquireResultCompleted:
			// Successful duplicate delivery — Ack without invoking the handler.
			return nil
		case inbox.AcquireResultInProgress:
			return fmt.Errorf("%w: group=%s event_id=%s", inbox.ErrInProgress, group, env.ID)
		default:
			return fmt.Errorf("%w: %q", inbox.ErrUnknownAcquireResult, claim)
		}

		if lockID == "" {
			return fmt.Errorf("%w: group=%s event_id=%s", inbox.ErrMissingLockID, group, env.ID)
		}

		acquired := true

		handlerDone := false
		defer func() {
			if r := recover(); r != nil {
				if acquired {
					_ = m.repo.Release(context.Background(), group, env.ID, lockID)
				}

				panic(r)
			}

			if err == nil || !acquired || handlerDone {
				return
			}

			if releaseErr := m.repo.Release(context.Background(), group, env.ID, lockID); releaseErr != nil {
				err = fmt.Errorf("event inbox: release after handler failure: %w (handler: %w)", releaseErr, err)
			}
		}()

		if err = next(ctx, d, env); err != nil {
			return err
		}

		handlerDone = true

		if err = m.repo.MarkCompleted(ctx, group, env.ID, lockID); err != nil {
			if errors.Is(err, inbox.ErrLockLost) {
				// The handler already succeeded. Ack this delivery and
				// leave the newer lock owner untouched; handler business
				// effects must be idempotent across such overlap.
				inboxLogger.Warnf(
					"event inbox lock lost after handler success: group=%s event_id=%s lock_id=%s",
					group, env.ID, lockID)

				return nil
			}

			return err
		}

		acquired = false

		return nil
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
