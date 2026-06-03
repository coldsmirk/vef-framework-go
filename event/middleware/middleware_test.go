package middleware_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// RecordingPublishMW appends its name to a shared log when its wrapper
// runs, so tests can assert composition order.
type RecordingPublishMW struct {
	name  string
	order int
	log   *[]string
}

func (m *RecordingPublishMW) Name() string { return m.name }
func (m *RecordingPublishMW) Order() int   { return m.order }
func (m *RecordingPublishMW) WrapPublish(next middleware.PublishHandler) middleware.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		*m.log = append(*m.log, m.name)

		return next(ctx, env)
	}
}

// RecordingConsumeMW is the consume-side analog, with a configurable
// Applies result so tests can verify capability filtering.
type RecordingConsumeMW struct {
	name    string
	order   int
	applies bool
	log     *[]string
}

func (m *RecordingConsumeMW) Name() string                        { return m.name }
func (m *RecordingConsumeMW) Order() int                          { return m.order }
func (m *RecordingConsumeMW) Applies(transport.Capabilities) bool { return m.applies }
func (m *RecordingConsumeMW) WrapConsume(next middleware.ConsumeHandler) middleware.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		*m.log = append(*m.log, m.name)

		return next(ctx, d, env)
	}
}

func TestChainPublish(t *testing.T) {
	t.Run("OrdersByAscendingOrder", func(t *testing.T) {
		var order []string

		mws := []middleware.PublishMiddleware{
			&RecordingPublishMW{name: "metrics", order: middleware.OrderMetrics, log: &order},
			&RecordingPublishMW{name: "tracing", order: middleware.OrderTracing, log: &order},
			&RecordingPublishMW{name: "logging", order: middleware.OrderLogging, log: &order},
			nil, // nil entries must be skipped
		}

		h := middleware.ChainPublish(mws, func(context.Context, *event.Envelope) error { return nil })
		require.NoError(t, h(context.Background(), &event.Envelope{}), "chained handler should succeed")
		require.Equal(t, []string{"tracing", "logging", "metrics"}, order,
			"lower Order must wrap outermost regardless of registration order")
	})

	t.Run("StableForEqualOrder", func(t *testing.T) {
		var order []string

		mws := []middleware.PublishMiddleware{
			&RecordingPublishMW{name: "a", order: 0, log: &order},
			&RecordingPublishMW{name: "b", order: 0, log: &order},
			&RecordingPublishMW{name: "c", order: 0, log: &order},
		}

		h := middleware.ChainPublish(mws, func(context.Context, *event.Envelope) error { return nil })
		require.NoError(t, h(context.Background(), &event.Envelope{}), "chained handler should succeed")
		require.Equal(t, []string{"a", "b", "c"}, order, "equal Order must preserve registration order")
	})
}

func TestChainConsume(t *testing.T) {
	t.Run("OrdersAndFiltersByApplies", func(t *testing.T) {
		var order []string

		mws := []middleware.ConsumeMiddleware{
			&RecordingConsumeMW{name: "inbox", order: middleware.OrderInbox, applies: true, log: &order},
			&RecordingConsumeMW{name: "recover", order: middleware.OrderRecover, applies: true, log: &order},
			&RecordingConsumeMW{name: "skipped", order: -1000, applies: false, log: &order},
			nil, // nil entries must be skipped
		}

		h := middleware.ChainConsume(mws, transport.Capabilities{}, func(context.Context, transport.Delivery, event.Envelope) error { return nil })
		require.NoError(t, h(context.Background(), nil, event.Envelope{}), "chained handler should succeed")
		require.Equal(t, []string{"recover", "inbox"}, order,
			"recover (lowest Order) wraps outermost; a non-applicable middleware is filtered out")
	})
}
