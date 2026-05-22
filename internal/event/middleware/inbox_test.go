package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/inbox"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	iinbox "github.com/coldsmirk/vef-framework-go/internal/event/inbox"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type InboxDelivery struct {
	frame transport.Frame
}

type LockLostInboxRepository struct {
	released bool
}

type CapturingInboxRepository struct {
	lockUntil timex.DateTime
}

func (*LockLostInboxRepository) Acquire(
	context.Context,
	string,
	string,
	timex.DateTime,
) (inbox.AcquireResult, string, error) {
	return inbox.AcquireResultAcquired, "lock-lost", nil
}

func (r *CapturingInboxRepository) Acquire(
	_ context.Context,
	_ string,
	_ string,
	lockUntil timex.DateTime,
) (inbox.AcquireResult, string, error) {
	r.lockUntil = lockUntil

	return inbox.AcquireResultAcquired, "captured-lock", nil
}

func (*LockLostInboxRepository) MarkCompleted(context.Context, string, string, string) error {
	return inbox.ErrLockLost
}

func (*CapturingInboxRepository) MarkCompleted(context.Context, string, string, string) error {
	return nil
}

func (r *LockLostInboxRepository) Release(context.Context, string, string, string) error {
	r.released = true

	return nil
}

func (*CapturingInboxRepository) Release(context.Context, string, string, string) error {
	return nil
}

func (*LockLostInboxRepository) DeleteOlderThan(context.Context, timex.DateTime) (int64, error) {
	return 0, nil
}

func (*CapturingInboxRepository) DeleteOlderThan(context.Context, timex.DateTime) (int64, error) {
	return 0, nil
}

func (d InboxDelivery) Frame() transport.Frame { return d.frame }

func (InboxDelivery) Attempt() int { return 1 }

func (InboxDelivery) Ack(context.Context) error { return nil }

func (InboxDelivery) Nack(context.Context, time.Duration, error) error { return nil }

func setupInboxMiddleware(t *testing.T) *Inbox {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, iinbox.Migrate(ctx, db, config.SQLite), "Inbox migration should succeed")

	return NewInbox(iinbox.NewRepository(db), 10*time.Minute)
}

func inboxEnvelope() event.Envelope {
	return event.Envelope{ID: "evt-inbox", Type: "test.inbox"}
}

func inboxDelivery() transport.Delivery {
	return InboxDelivery{frame: transport.Frame{ID: "evt-inbox", Type: "test.inbox"}}
}

func TestInboxMiddlewareReleasesClaimOnHandlerFailure(t *testing.T) {
	mw := setupInboxMiddleware(t)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	expected := errors.New("handler failed")
	calls := 0

	handler := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		calls++
		if calls == 1 {
			return expected
		}

		return nil
	})

	err := handler(ctx, inboxDelivery(), inboxEnvelope())
	require.ErrorIs(t, err, expected, "First failed delivery should return the handler error")

	err = handler(ctx, inboxDelivery(), inboxEnvelope())
	require.NoError(t, err, "Released failed delivery should be processed again")
	require.Equal(t, 2, calls, "Handler should be invoked again after a failed delivery")
}

func TestInboxMiddlewareReleasesClaimOnPanic(t *testing.T) {
	mw := setupInboxMiddleware(t)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	expected := "panic while handling"

	func() {
		defer func() {
			require.Equal(t, expected, recover(), "Panic should propagate after release")
		}()

		_ = mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
			panic(expected)
		})(ctx, inboxDelivery(), inboxEnvelope())
	}()

	calls := 0
	err := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		calls++

		return nil
	})(ctx, inboxDelivery(), inboxEnvelope())

	require.NoError(t, err, "Delivery should be claimable again after panic release")
	require.Equal(t, 1, calls, "Retry handler should run after panic release")
}

func TestInboxMiddlewareSkipsCompletedDuplicate(t *testing.T) {
	mw := setupInboxMiddleware(t)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	calls := 0

	handler := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		calls++

		return nil
	})

	require.NoError(t, handler(ctx, inboxDelivery(), inboxEnvelope()), "First delivery should complete")
	require.NoError(t, handler(ctx, inboxDelivery(), inboxEnvelope()), "Completed duplicate should be acknowledged")
	require.Equal(t, 1, calls, "Completed duplicate should not invoke the handler")
}

func TestInboxMiddlewareReturnsErrorForActiveDuplicate(t *testing.T) {
	mw := setupInboxMiddleware(t)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	block := errors.New("keep processing")

	handler := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		return block
	})

	err := handler(ctx, inboxDelivery(), inboxEnvelope())
	require.ErrorIs(t, err, block, "First delivery should surface handler failure")

	// Manually acquire a live claim to model a concurrent consumer that
	// still owns the same event.
	repo := mw.repo
	_, _, err = repo.Acquire(context.Background(), "consumer-a", "evt-active", timex.Now().Add(10*time.Minute))
	require.NoError(t, err, "Manual claim should succeed")

	activeEnv := event.Envelope{ID: "evt-active", Type: "test.inbox"}
	err = mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		t.Fatal("Handler must not run for active duplicate")

		return nil
	})(ctx, InboxDelivery{frame: transport.Frame{ID: "evt-active", Type: "test.inbox"}}, activeEnv)
	require.ErrorIs(t, err, inbox.ErrInProgress, "Active duplicate should stay pending for retry")
}

func TestInboxMiddlewareAcknowledgesLostLockAfterHandlerSuccess(t *testing.T) {
	repo := new(LockLostInboxRepository)
	mw := NewInbox(repo, 10*time.Minute)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	calls := 0

	err := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		calls++

		return nil
	})(ctx, inboxDelivery(), inboxEnvelope())

	require.NoError(t, err, "Lost lock after handler success should be acknowledged")
	require.Equal(t, 1, calls, "Handler should run before the lock loss is detected")
	require.False(t, repo.released, "Successful handler with lost lock should not release a newer claim")
}

func TestInboxMiddlewareUsesConfiguredProcessingLease(t *testing.T) {
	repo := new(CapturingInboxRepository)
	lease := 37 * time.Second
	mw := NewInbox(repo, lease)
	ctx := WithConsumerGroup(context.Background(), "consumer-a")
	before := time.Now().Add(lease)

	err := mw.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error {
		return nil
	})(ctx, inboxDelivery(), inboxEnvelope())

	after := time.Now().Add(lease)

	require.NoError(t, err, "Handler should complete with captured lease repository")

	got := repo.lockUntil.Unwrap()
	require.False(t, got.Before(before), "Processing lease deadline should not precede now+lease at call start")
	require.False(t, got.After(after), "Processing lease deadline should not exceed now+lease at call end")
}

var _ middleware.ConsumeMiddleware = (*Inbox)(nil)
