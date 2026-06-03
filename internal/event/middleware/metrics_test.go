package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// CountingRecorder records observations so tests can assert what the
// Metrics middleware forwarded.
type CountingRecorder struct {
	publishTypes []string
	publishErrs  int
	consumeTypes []string
	consumeErrs  int
}

func (r *CountingRecorder) PublishObserved(eventType string, err error) {
	r.publishTypes = append(r.publishTypes, eventType)

	if err != nil {
		r.publishErrs++
	}
}

func (r *CountingRecorder) ConsumeObserved(eventType string, _ time.Duration, err error) {
	r.consumeTypes = append(r.consumeTypes, eventType)

	if err != nil {
		r.consumeErrs++
	}
}

func TestMetrics(t *testing.T) {
	t.Run("Order", func(t *testing.T) {
		require.Equal(t, pubmw.OrderMetrics, NewMetrics(nil).Order(), "metrics Order must equal OrderMetrics")
	})

	t.Run("PublishAndConsumeObserved", func(t *testing.T) {
		rec := &CountingRecorder{}
		m := NewMetrics(rec)

		ph := m.WrapPublish(func(context.Context, *event.Envelope) error { return errors.New("boom") })
		_ = ph(context.Background(), &event.Envelope{Type: "test.created"})

		ch := m.WrapConsume(func(context.Context, transport.Delivery, event.Envelope) error { return nil })
		require.NoError(t, ch(context.Background(), nil, event.Envelope{Type: "test.created"}), "consume should succeed")

		require.Equal(t, []string{"test.created"}, rec.publishTypes, "publish observation should record the type")
		require.Equal(t, 1, rec.publishErrs, "a failed publish should be observed as an error")
		require.Equal(t, []string{"test.created"}, rec.consumeTypes, "consume observation should record the type")
		require.Equal(t, 0, rec.consumeErrs, "a successful consume should not record an error")
	})

	t.Run("NilRecorderIsSafe", func(t *testing.T) {
		ph := NewMetrics(nil).WrapPublish(func(context.Context, *event.Envelope) error { return nil })
		require.NoError(t, ph(context.Background(), &event.Envelope{Type: "x"}), "a nil recorder must be replaced with a no-op")
	})

	t.Run("NormalizeKeyCapsCardinality", func(t *testing.T) {
		key := normalizeKey(strings.Repeat("a", metricsKeyMaxLen+50))
		require.Len(t, key, metricsKeyMaxLen, "an over-long event type must be truncated to the cardinality cap")
	})

	t.Run("NormalizeKeyReplacesDots", func(t *testing.T) {
		require.Equal(t, "a_b_c", normalizeKey("a.b.c"), "dots must become underscores for expvar JSON consumers")
	})
}
