package binding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
)

// spyBus captures the most recent Subscribe arguments so tests can verify
// the listener supplies a stable consumer group via WithGroup. It is
// intentionally minimal: only Subscribe is exercised, and Publish /
// PublishBatch return nil without recording so the listener's failure
// branch can run without a real bus.
type spyBus struct {
	subscribeCalls int
	capturedType   string
	capturedGroup  string
}

func (b *spyBus) Subscribe(eventType string, _ event.Handler, opts ...event.SubscribeOption) (event.Unsubscribe, error) {
	cfg := event.ApplySubscribeOptions(opts)
	b.subscribeCalls++
	b.capturedType = eventType
	b.capturedGroup = cfg.Group

	return func() {}, nil
}

func (*spyBus) Publish(context.Context, event.Event, ...event.PublishOption) error {
	return nil
}

func (*spyBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

func TestListenerStartSubscribesWithStableGroup(t *testing.T) {
	bus := &spyBus{}
	listener := NewListener(nil, bus, nil)

	require.NoError(t, listener.Start(), "Start 不应返回错误")
	assert.Equal(t, 1, bus.subscribeCalls, "应当只订阅一次")
	assert.Equal(t, approval.EventTypeInstanceCompleted, bus.capturedType,
		"应当订阅 InstanceCompleted 事件")
	assert.Equal(t, bindingConsumerGroup, bus.capturedGroup,
		"必须带上稳定的 consumer group 名称，否则在 at-least-once 路由下会触发 ErrGroupRequired")
	assert.Equal(t, "approval:binding", bus.capturedGroup,
		"group 名称是 inbox dedupe scope，必须保持稳定")
}
