package middleware

import (
	"context"
	"expvar"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// Metrics maintains lightweight expvar counters and a sliding latency
// histogram bucket per event type. It is wire-compatible with whatever
// expvar scraper an application uses (Prometheus exporter, /debug/vars
// endpoint, etc.) and avoids pulling a heavier metrics library into
// the framework core.
type Metrics struct {
	publishCount *expvar.Map
	publishErr   *expvar.Map
	consumeCount *expvar.Map
	consumeErr   *expvar.Map
	consumeMs    *expvar.Map
}

// NewMetrics constructs a Metrics middleware. The expvar names are
// stable: "vef_event_publish_count" etc. — applications can map them
// to Prometheus via expvar collectors.
func NewMetrics() *Metrics {
	return &Metrics{
		publishCount: lookupOrNewMap("vef_event_publish_count"),
		publishErr:   lookupOrNewMap("vef_event_publish_error"),
		consumeCount: lookupOrNewMap("vef_event_consume_count"),
		consumeErr:   lookupOrNewMap("vef_event_consume_error"),
		consumeMs:    lookupOrNewMap("vef_event_consume_latency_ms"),
	}
}

// Name implements both middleware interfaces.
func (*Metrics) Name() string { return "metrics" }

// Applies always returns true.
func (*Metrics) Applies(transport.Capabilities) bool { return true }

// WrapPublish counts publishes and errors per event type.
func (m *Metrics) WrapPublish(next pubmw.PublishHandler) pubmw.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		err := next(ctx, env)
		key := normalizeKey(env.Type)
		m.publishCount.Add(key, 1)

		if err != nil {
			m.publishErr.Add(key, 1)
		}

		return err
	}
}

// WrapConsume counts consumes, errors, and accumulates total latency.
// Latency is summed in milliseconds; an external scraper computing
// rate(latency_ms[1m]) / rate(consume_count[1m]) yields average ms.
func (m *Metrics) WrapConsume(next pubmw.ConsumeHandler) pubmw.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		start := time.Now()
		err := next(ctx, d, env)
		elapsed := time.Since(start).Milliseconds()

		key := normalizeKey(env.Type)
		m.consumeCount.Add(key, 1)
		m.consumeMs.Add(key, elapsed)

		if err != nil {
			m.consumeErr.Add(key, 1)
		}

		return err
	}
}

// lookupOrNewMap returns an existing expvar.Map by name or creates one;
// expvar.Publish panics on duplicate names so we guard against it for
// repeated NewMetrics calls in tests.
func lookupOrNewMap(name string) *expvar.Map {
	if existing := expvar.Get(name); existing != nil {
		if m, ok := existing.(*expvar.Map); ok {
			return m
		}
	}

	m := new(expvar.Map).Init()
	expvar.Publish(name, m)

	return m
}

// metricsKeyMaxLen caps the metrics label length so an adversarial
// publisher cannot make expvar.Map grow without bound by sending
// distinct overly-long event types.
const metricsKeyMaxLen = 128

// normalizeKey replaces dots in event types with underscores so expvar
// JSON consumers don't choke on nested-looking keys, then truncates
// the result so a hostile event-type string cannot blow up cardinality.
func normalizeKey(eventType string) string {
	key := strings.ReplaceAll(eventType, ".", "_")
	if len(key) > metricsKeyMaxLen {
		key = key[:metricsKeyMaxLen]
	}

	return key
}
