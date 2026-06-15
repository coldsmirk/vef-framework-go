package redisstream

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// newReaperTestTransport spins up a Redis container and returns a started
// transport plus its client. ClaimInterval is long so the reaper loop does
// not auto-fire — tests drive reapOnce directly for determinism.
func newReaperTestTransport(t *testing.T, cfg redisstream.Config) (*Transport, *goredis.Client) {
	t.Helper()

	container := testx.NewRedisContainer(context.Background(), t)
	client := goredis.NewClient(&goredis.Options{
		Addr: net.JoinHostPort(container.Redis.Host, strconv.Itoa(int(container.Redis.Port))),
	})
	t.Cleanup(func() { _ = client.Close() })

	cfg.StreamPrefix = "reaper:test:"

	cfg.ClaimInterval = time.Hour // never auto-fire; tests call reapOnce
	if cfg.ClaimIdle == 0 {
		cfg.ClaimIdle = time.Millisecond
	}

	tp := New(client, cfg, nil)
	require.NoError(t, tp.Start(context.Background()), "transport should start")
	t.Cleanup(func() { _ = tp.Stop(context.Background()) })

	return tp, client
}

// seedPending publishes one frame to the subscription's stream and reads
// it with a throwaway "dead" consumer, leaving it pending and idle so the
// reaper reclaims it for the live consumer on the next reapOnce.
func seedPending(t *testing.T, client *goredis.Client, sub *subscription) {
	t.Helper()

	ctx := context.Background()
	require.NoError(t, client.XGroupCreateMkStream(ctx, sub.stream, sub.group, "0").Err(),
		"group create should succeed")

	frame := transport.Frame{ID: "seed-" + sub.stream, Type: "reaper.test", Body: []byte(`{}`)}
	body, err := json.Marshal(frame)
	require.NoError(t, err, "frame marshal should succeed")

	require.NoError(t, client.XAdd(ctx, &goredis.XAddArgs{
		Stream: sub.stream,
		Values: map[string]any{"frame": body},
	}).Err(), "xadd should succeed")

	// A dead consumer reads (and never ACKs) the message so it becomes
	// pending under a consumer other than sub.consumer.
	_, err = client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    sub.group,
		Consumer: "dead-" + sub.stream,
		Streams:  []string{sub.stream, ">"},
		Count:    1,
	}).Result()
	require.NoError(t, err, "dead-consumer read should succeed")

	// Ensure the pending entry is older than ClaimIdle.
	time.Sleep(5 * time.Millisecond)
}

// TestReaperFanOutIsolatesSlowHandler is the head-of-line regression: a
// hung handler reclaiming one stream must not serialize reclaim for an
// unrelated stream. With the old single-goroutine sequential reaper, the
// fast stream could only be reclaimed after the slow handler returned.
func TestReaperFanOutIsolatesSlowHandler(t *testing.T) {
	// HandlerTimeout disabled so the slow handler stays hung for the
	// duration of the assertion; ReaperConcurrency default (4) lets both
	// subscriptions reap in parallel.
	tp, client := newReaperTestTransport(t, redisstream.Config{})

	release := make(chan struct{})
	fastReceived := make(chan struct{}, 1)

	t.Cleanup(func() { close(release) })

	slowSub := &subscription{
		stream:   tp.cfg.StreamKey("slow"),
		group:    "g-slow",
		consumer: "live-slow",
		stopCh:   make(chan struct{}),
		fn: func(ctx context.Context, _ transport.Delivery) error {
			select {
			case <-release:
			case <-ctx.Done():
			}

			return nil
		},
	}
	fastSub := &subscription{
		stream:   tp.cfg.StreamKey("fast"),
		group:    "g-fast",
		consumer: "live-fast",
		stopCh:   make(chan struct{}),
		fn: func(context.Context, transport.Delivery) error {
			fastReceived <- struct{}{}

			return nil
		},
	}

	seedPending(t, client, slowSub)
	seedPending(t, client, fastSub)

	tp.mu.Lock()
	tp.subs = []*subscription{slowSub, fastSub}
	tp.mu.Unlock()

	// reapOnce blocks until every reap finishes; the slow handler is hung,
	// so run it in the background and assert the fast stream is reclaimed
	// concurrently rather than serialized behind the slow one.
	go tp.reapOnce()

	select {
	case <-fastReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("fast stream was not reclaimed while the slow handler was blocked — reaper serialized failover")
	}
}

// TestReaperHandlerTimeoutUnblocksStuckHandler verifies the per-delivery
// deadline: a handler that ignores release is still canceled via context
// once HandlerTimeout elapses, freeing the reaper worker.
func TestReaperHandlerTimeoutUnblocksStuckHandler(t *testing.T) {
	tp, client := newReaperTestTransport(t, redisstream.Config{HandlerTimeout: 200 * time.Millisecond})

	observed := make(chan error, 1)

	sub := &subscription{
		stream:   tp.cfg.StreamKey("timeout"),
		group:    "g-timeout",
		consumer: "live-timeout",
		stopCh:   make(chan struct{}),
		fn: func(ctx context.Context, _ transport.Delivery) error {
			<-ctx.Done() // never returns on its own; only the deadline frees it

			observed <- ctx.Err()

			return ctx.Err()
		},
	}

	seedPending(t, client, sub)

	tp.mu.Lock()
	tp.subs = []*subscription{sub}
	tp.mu.Unlock()

	done := make(chan struct{})
	go func() {
		tp.reapOnce()
		close(done)
	}()

	select {
	case err := <-observed:
		require.ErrorIs(t, err, context.DeadlineExceeded,
			"a hung handler must be canceled by the configured HandlerTimeout")
	case <-time.After(5 * time.Second):
		t.Fatal("handler was never canceled by HandlerTimeout")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reapOnce did not return after the handler deadline freed the worker")
	}
}
