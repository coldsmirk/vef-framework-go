package binding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// SpyBus captures the most recent Subscribe arguments so tests can verify
// the listener supplies a stable consumer group via WithGroup. It is
// intentionally minimal: only Subscribe is exercised, and Publish /
// PublishBatch return nil without recording so the listener's failure
// branch can run without a real bus.
type SpyBus struct {
	subscribeCalls int
	capturedType   string
	capturedGroup  string
	publishErrs    []error
	publishCalls   []event.PublishConfig
}

func (b *SpyBus) Subscribe(eventType string, _ event.Handler, opts ...event.SubscribeOption) (event.Unsubscribe, error) {
	cfg := event.ApplySubscribeOptions(opts)
	b.subscribeCalls++
	b.capturedType = eventType
	b.capturedGroup = cfg.Group

	return func() {}, nil
}

func (b *SpyBus) Publish(_ context.Context, _ event.Event, opts ...event.PublishOption) error {
	b.publishCalls = append(b.publishCalls, event.ApplyPublishOptions(opts))
	if len(b.publishErrs) == 0 {
		return nil
	}

	err := b.publishErrs[0]
	b.publishErrs = b.publishErrs[1:]

	return err
}

func (*SpyBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

func TestListenerStartSubscribesWithStableGroup(t *testing.T) {
	bus := &SpyBus{}
	listener := NewListener(nil, bus, nil)

	require.NoError(t, listener.Start(), "Start should not return an error")
	assert.Equal(t, 1, bus.subscribeCalls, "Listener should subscribe exactly once")
	assert.Equal(t, approval.EventTypeInstanceCompleted, bus.capturedType,
		"Listener should subscribe to InstanceCompletedEvent")
	assert.Equal(t, bindingConsumerGroup, bus.capturedGroup,
		"Listener must set a stable consumer group for at-least-once routes")
	assert.Equal(t, "approval:binding", bus.capturedGroup,
		"Group name is the inbox dedupe scope and must remain stable")
}

func TestListenerPublishFailure(t *testing.T) {
	t.Run("UsesTxWhenDatabaseAvailable", func(t *testing.T) {
		bus := &SpyBus{}
		listener := NewListener(testx.NewTestDB(t), bus, nil)

		err := listener.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			"inst-1", "tenant-1", "flow-1", approval.InstanceApproved, "biz_table", "boom"))

		require.NoError(t, err, "Binding failure should publish through a short transaction when DB is available")
		require.Len(t, bus.publishCalls, 1, "Binding failure should publish once to avoid outbox plus memory double delivery")
		assert.NotNil(t, bus.publishCalls[0].Tx, "Publish options should include Tx so routing selects only outbox")
	})

	t.Run("FallsBackWhenNoTransactionalRoute", func(t *testing.T) {
		bus := &SpyBus{publishErrs: []error{event.ErrTxRequired}}
		listener := NewListener(testx.NewTestDB(t), bus, nil)

		err := listener.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			"inst-1", "tenant-1", "flow-1", approval.InstanceApproved, "biz_table", "boom"))

		require.NoError(t, err, "Binding failure should fall back to non-transactional publish when no Tx route exists")
		require.Len(t, bus.publishCalls, 2, "Binding failure should retry once without Tx after ErrTxRequired")
		assert.NotNil(t, bus.publishCalls[0].Tx, "First publish attempt should include Tx")
		assert.Nil(t, bus.publishCalls[1].Tx, "Fallback publish should not include Tx")
	})

	t.Run("PublishesDirectlyWithoutDatabase", func(t *testing.T) {
		bus := &SpyBus{}
		listener := NewListener(nil, bus, nil)

		err := listener.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			"inst-1", "tenant-1", "flow-1", approval.InstanceApproved, "biz_table", "boom"))

		require.NoError(t, err, "Binding failure should publish directly when DB is unavailable")
		require.Len(t, bus.publishCalls, 1, "Binding failure should publish exactly once")
		assert.Nil(t, bus.publishCalls[0].Tx, "Publish options should not include Tx when DB is unavailable")
	})
}
