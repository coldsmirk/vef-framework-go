package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/event/transport/txmemory"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TxMemoryTestEvent is the event type used by the bus-level tests below.
type TxMemoryTestEvent struct {
	Value string `json:"value"`
}

func (*TxMemoryTestEvent) EventType() string { return "tx_memory.test" }

func TestTxMemoryConfigMapping(t *testing.T) {
	cfg := &config.EventConfig{
		Transports: config.EventTransportsConfig{
			TxMemory: config.EventTxMemoryTransportConfig{
				Enabled:        true,
				QueueSize:      17,
				FullPolicy:     "drop_oldest",
				PublishTimeout: 3 * time.Second,
			},
		},
	}

	got := txMemoryConfig(cfg)

	require.Equal(t, 17, got.QueueSize, "Queue size should be copied")
	require.Equal(t, memory.FullPolicyDropOldest, got.FullPolicy, "Full policy should be copied")
	require.Equal(t, 3*time.Second, got.PublishTimeout, "Publish timeout should be copied")
}

func TestNewTxMemoryTransport(t *testing.T) {
	t.Run("NilWhenDisabled", func(t *testing.T) {
		got := newTxMemoryTransport(&config.EventConfig{})

		require.Nil(t, got, "A disabled transport must not join the registry")
	})

	t.Run("NamedWhenEnabled", func(t *testing.T) {
		got := newTxMemoryTransport(&config.EventConfig{
			Transports: config.EventTransportsConfig{
				TxMemory: config.EventTxMemoryTransportConfig{Enabled: true},
			},
		})

		require.NotNil(t, got, "An enabled transport must join the registry")
		require.Equal(t, txmemory.Name, got.Name(), "The transport should register under its routing name")
	})
}

// TestTxMemoryRouteSatisfiesTransactionalPublish exercises the reason the
// transport exists: routing to it must satisfy the transactional-route
// assertion that the approval and storage modules make at start-up, and a
// WithTx publish must reach an in-process subscriber — in the publishing
// process — once the business transaction commits.
func TestTxMemoryRouteSatisfiesTransactionalPublish(t *testing.T) {
	newStartedBus := func(t *testing.T) (*Bus, orm.DB) {
		t.Helper()

		tp := newTxMemoryTransport(&config.EventConfig{
			Transports: config.EventTransportsConfig{
				TxMemory: config.EventTxMemoryTransportConfig{Enabled: true},
			},
		})

		bus := NewBus(
			&config.EventConfig{DefaultTransport: txmemory.Name},
			"test-app",
			[]transport.Transport{tp},
			nil, nil, nil,
		)
		require.NoError(t, bus.Start(t.Context()), "Bus should start")
		t.Cleanup(func() { _ = bus.Stop(context.Background()) })

		return bus, testx.NewTestDB(t)
	}

	t.Run("HasTransactionalRoute", func(t *testing.T) {
		bus, _ := newStartedBus(t)

		require.True(t, bus.HasTransactionalRoute("tx_memory.test"),
			"Routing to tx_memory must satisfy modules that publish with WithTx")
		require.True(t, bus.HasSubscribableTransport("tx_memory.test"),
			"Unlike the outbox, tx_memory delivers to its own subscribers")
	})

	t.Run("DeliveredInProcessAfterCommit", func(t *testing.T) {
		bus, db := newStartedBus(t)

		received := make(chan string, 1)
		unsub, err := bus.Subscribe("tx_memory.test", func(_ context.Context, env event.Envelope) error {
			received <- env.ID

			return nil
		})
		require.NoError(t, err, "Subscribe should succeed")
		t.Cleanup(unsub)

		require.NoError(t, db.RunInTx(t.Context(), func(txCtx context.Context, tx orm.DB) error {
			// The transaction context is what carries the deferral, exactly as
			// a business handler publishing inside RunInTx would pass it.
			return bus.Publish(txCtx, &TxMemoryTestEvent{Value: "committed"}, event.WithTx(tx))
		}), "Transactional publish should succeed")

		select {
		case <-received:
		case <-time.After(2 * time.Second):
			t.Fatal("the committed event never reached the in-process subscriber")
		}
	})

	t.Run("NotDeliveredOnRollback", func(t *testing.T) {
		bus, db := newStartedBus(t)

		received := make(chan string, 1)
		unsub, err := bus.Subscribe("tx_memory.test", func(_ context.Context, env event.Envelope) error {
			received <- env.ID

			return nil
		})
		require.NoError(t, err, "Subscribe should succeed")
		t.Cleanup(unsub)

		failure := errors.New("business failure")

		err = db.RunInTx(t.Context(), func(txCtx context.Context, tx orm.DB) error {
			if err := bus.Publish(txCtx, &TxMemoryTestEvent{Value: "rolled-back"}, event.WithTx(tx)); err != nil {
				return err
			}

			return failure
		})
		require.ErrorIs(t, err, failure, "RunInTx should surface the business failure")

		select {
		case id := <-received:
			t.Fatalf("a rolled-back transaction emitted a phantom event (%s)", id)
		case <-time.After(150 * time.Millisecond):
		}
	})
}
