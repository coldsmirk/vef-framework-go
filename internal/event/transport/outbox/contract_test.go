package outbox_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	pubmemory "github.com/coldsmirk/vef-framework-go/event/transport/memory"
	puboutbox "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/transporttest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// TestOutboxTransportContract drives the shared contract suite against
// the outbox transport. The outbox needs a continuous relay loop for
// publish→consume to round-trip, so the factory starts a background
// pump that drives RelayPending until the test cleans up.
func TestOutboxTransportContract(t *testing.T) {
	factory := func(t *testing.T) (transport.Transport, func()) {
		ctx := context.Background()
		db := testx.NewTestDB(t)
		require.NoError(t, outbox.Migrate(ctx, db, config.SQLite), "outbox migration should succeed")

		repo := outbox.NewRepository(db)

		sink := memory.New(pubmemory.Config{QueueSize: 64, FullPolicy: pubmemory.FullPolicyError})

		cfg := puboutbox.Config{
			RelayInterval:   50 * time.Millisecond,
			MaxRetries:      5,
			BatchSize:       50,
			LeaseMultiplier: 4,
			MinLease:        time.Second,
		}
		tp := outbox.NewTransport(repo, cfg)
		tp.SetSink(sink)

		stopped := new(atomic.Bool)

		stopCh := make(chan struct{})
		go func() {
			relay := outbox.NewRelay(repo, tp.Sink, cfg, nil, nil)

			ticker := time.NewTicker(cfg.RelayInterval)
			defer ticker.Stop()

			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					if stopped.Load() {
						return
					}

					relay.RelayPending(ctx)
				}
			}
		}()

		cleanup := func() {
			stopped.Store(true)
			close(stopCh)
		}

		return tp, cleanup
	}
	transporttest.Suite(t, "Outbox", factory)
}
