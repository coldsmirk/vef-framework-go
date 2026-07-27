package txmemory_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/event/transport/txmemory"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/transporttest"
	itxmemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/txmemory"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// deliveryWait bounds how long a test waits for the inner transport's
// worker goroutine to hand a frame to the subscriber.
const deliveryWait = 2 * time.Second

// quietWait is how long a test watches for a frame that must never arrive.
const quietWait = 150 * time.Millisecond

// ErrorRecordingLogger captures Errorf calls so a test can assert that a
// post-commit dispatch failure is reported rather than swallowed. The
// embedded nil Logger is never exercised: only Errorf is called.
type ErrorRecordingLogger struct {
	logx.Logger

	mu     sync.Mutex
	errors []string
}

func (l *ErrorRecordingLogger) Errorf(template string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, fmt.Sprintf(template, args...))
}

func (l *ErrorRecordingLogger) Errors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.errors...)
}

// startTransport brings up a transport with a subscription on eventType
// and returns the channel its handler feeds.
func startTransport(t *testing.T, tp *itxmemory.Transport, eventType string) <-chan transport.Frame {
	t.Helper()

	ctx := context.Background()
	require.NoError(t, tp.Start(ctx), "Start should succeed")
	t.Cleanup(func() { require.NoError(t, tp.Stop(context.Background()), "Stop should succeed") })

	received := make(chan transport.Frame, 8)

	unsub, err := tp.Subscribe(eventType, "", func(ctx context.Context, d transport.Delivery) error {
		received <- d.Frame()

		return d.Ack(ctx)
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe should succeed")
	t.Cleanup(unsub)

	return received
}

func frame(id, eventType string) transport.Frame {
	return transport.Frame{ID: id, Type: eventType, Body: []byte(`{}`)}
}

func requireFrame(t *testing.T, ch <-chan transport.Frame) transport.Frame {
	t.Helper()

	select {
	case f := <-ch:
		return f
	case <-time.After(deliveryWait):
		t.Fatal("timed out waiting for a frame that should have been delivered")

		return transport.Frame{}
	}
}

func requireNoFrame(t *testing.T, ch <-chan transport.Frame, msg string) {
	t.Helper()

	select {
	case f := <-ch:
		t.Fatalf("%s (got frame %s)", msg, f.ID)
	case <-time.After(quietWait):
	}
}

// TestTransportContract runs the shared transport contract over the
// non-transactional path, which must behave exactly like the memory
// transport it delegates to.
func TestTransportContract(t *testing.T) {
	transporttest.Suite(t, "tx_memory", func(*testing.T) (transport.Transport, func()) {
		tp := itxmemory.New(memory.Config{}, nil)

		return tp, func() { _ = tp.Stop(context.Background()) }
	})
}

func TestCapabilities(t *testing.T) {
	caps := itxmemory.New(memory.Config{}, nil).Capabilities()

	assert.True(t, caps.Transactional, "the transport exists to satisfy transactional routes")
	assert.False(t, caps.Durable, "nothing is persisted, so durability must not be advertised")
	assert.False(t, caps.AtLeastOnce, "no redelivery, so subscribers must not be forced to declare a group")
	assert.False(t, caps.Ordered, "commit order is not publish order across concurrent transactions")
	assert.False(t, caps.PublishOnly, "the transport delivers to its own subscribers")

	_, ok := any(itxmemory.New(memory.Config{}, nil)).(transport.TxTransport)
	assert.True(t, ok, "Transactional=true implies the TxTransport assertion succeeds")
}

func TestName(t *testing.T) {
	assert.Equal(t, txmemory.Name, itxmemory.New(memory.Config{}, nil).Name(),
		"the transport must answer to the name used in routing config")
}

func TestPublishTx(t *testing.T) {
	const eventType = "test.tx"

	ctx := context.Background()

	t.Run("DeliversAfterCommitNotBefore", func(t *testing.T) {
		db := testx.NewTestDB(t)
		tp := itxmemory.New(memory.Config{}, nil)
		received := startTransport(t, tp, eventType)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			if err := tp.PublishTx(txCtx, tx, []transport.Frame{frame("evt-1", eventType)}); err != nil {
				return err
			}

			requireNoFrame(t, received, "a frame must not be delivered while its transaction is still open")

			return nil
		}), "RunInTx should succeed")

		assert.Equal(t, "evt-1", requireFrame(t, received).ID, "the frame is delivered once the transaction commits")
	})

	t.Run("DeliversNothingOnRollback", func(t *testing.T) {
		db := testx.NewTestDB(t)
		tp := itxmemory.New(memory.Config{}, nil)
		received := startTransport(t, tp, eventType)

		failure := errors.New("business failure")

		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			require.NoError(t, tp.PublishTx(txCtx, tx, []transport.Frame{frame("evt-1", eventType)}),
				"PublishTx should accept the frame")

			return failure
		})

		require.ErrorIs(t, err, failure, "RunInTx should surface the business failure")
		requireNoFrame(t, received, "a rolled-back transaction must not emit a phantom event")
	})

	t.Run("OutsideCommitScopeReports", func(t *testing.T) {
		tp := itxmemory.New(memory.Config{}, nil)
		received := startTransport(t, tp, eventType)

		err := tp.PublishTx(ctx, nil, []transport.Frame{frame("evt-1", eventType)})

		require.ErrorIs(t, err, orm.ErrNoCommitScope,
			"without a commit to defer to, the publish must fail loudly rather than deliver immediately")
		requireNoFrame(t, received, "a failed PublishTx must not deliver anything")
	})

	t.Run("SnapshotsFramesAgainstCallerReuse", func(t *testing.T) {
		db := testx.NewTestDB(t)
		tp := itxmemory.New(memory.Config{}, nil)
		received := startTransport(t, tp, eventType)

		frames := []transport.Frame{frame("original", eventType)}

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			if err := tp.PublishTx(txCtx, tx, frames); err != nil {
				return err
			}

			// The batch outlives this call, so the transport must not read
			// the caller's slice again at commit time.
			frames[0] = frame("mutated", eventType)

			return nil
		}), "RunInTx should succeed")

		assert.Equal(t, "original", requireFrame(t, received).ID,
			"the delivered frame must be the one submitted, not the caller's later mutation")
	})

	t.Run("DispatchFailureIsReportedNotReturned", func(t *testing.T) {
		db := testx.NewTestDB(t)
		spy := new(ErrorRecordingLogger)
		tp := itxmemory.New(memory.Config{}, spy)

		startTransport(t, tp, eventType)

		// Stopping mid-transaction makes the post-commit hand-off fail
		// deterministically — the shutdown-between-commit-and-dispatch case.
		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			if err := tp.PublishTx(txCtx, tx, []transport.Frame{frame("evt-1", eventType)}); err != nil {
				return err
			}

			return tp.Stop(context.Background())
		}), "a failed hand-off must not fail the committed business transaction")

		errs := spy.Errors()
		require.Len(t, errs, 1, "the failed post-commit dispatch must be reported exactly once")
		assert.Contains(t, errs[0], "1 frame(s)", "the report should say how much was lost")
	})
}
