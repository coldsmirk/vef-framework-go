package middleware

import (
	"context"
	"expvar"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// Metrics is a thin pass-through middleware that delegates every
// observation to a pluggable event.MetricsRecorder. The default
// expvar-backed recorder is installed by the fx module; applications
// can decorate the recorder to forward observations to Prometheus,
// OpenTelemetry, or their own sink.
type Metrics struct {
	rec event.MetricsRecorder
}

// NewMetrics constructs a Metrics middleware that forwards observations
// to the supplied recorder. A nil recorder is replaced with a no-op so
// the middleware is always safe to attach.
func NewMetrics(rec event.MetricsRecorder) *Metrics {
	if rec == nil {
		rec = new(NoopMetricsRecorder)
	}

	return &Metrics{rec: rec}
}

// Name implements both middleware interfaces.
func (*Metrics) Name() string { return "metrics" }

// Applies always returns true.
func (*Metrics) Applies(transport.Capabilities) bool { return true }

// WrapPublish reports every publish outcome through the recorder.
func (m *Metrics) WrapPublish(next middleware.PublishHandler) middleware.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		err := next(ctx, env)
		m.rec.PublishObserved(env.Type, err)

		return err
	}
}

// WrapConsume reports every consume outcome plus elapsed time through
// the recorder.
func (m *Metrics) WrapConsume(next middleware.ConsumeHandler) middleware.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		start := time.Now()
		err := next(ctx, d, env)
		m.rec.ConsumeObserved(env.Type, time.Since(start), err)

		return err
	}
}

// NoopMetricsRecorder satisfies event.MetricsRecorder but discards
// every observation. Useful as a fallback when an application disables
// metrics entirely while leaving the middleware enabled.
type NoopMetricsRecorder struct{}

// PublishObserved implements event.MetricsRecorder.
func (*NoopMetricsRecorder) PublishObserved(string, error) {}

// ConsumeObserved implements event.MetricsRecorder.
func (*NoopMetricsRecorder) ConsumeObserved(string, time.Duration, error) {}

// ExpvarMetricsRecorder publishes counts and total latency under
// stable expvar names: vef_event_publish_count, vef_event_publish_error,
// vef_event_consume_count, vef_event_consume_error,
// vef_event_consume_latency_ms. Suitable for default observability via
// /debug/vars or an expvar-to-Prometheus collector.
type ExpvarMetricsRecorder struct {
	publishCount *expvar.Map
	publishErr   *expvar.Map
	consumeCount *expvar.Map
	consumeErr   *expvar.Map
	consumeMs    *expvar.Map
}

// NewExpvarMetricsRecorder constructs the framework's default recorder.
// expvar.Publish panics on duplicate names so existing maps are reused
// when constructed more than once (e.g. across test re-runs).
func NewExpvarMetricsRecorder() *ExpvarMetricsRecorder {
	return &ExpvarMetricsRecorder{
		publishCount: lookupOrNewMap("vef_event_publish_count"),
		publishErr:   lookupOrNewMap("vef_event_publish_error"),
		consumeCount: lookupOrNewMap("vef_event_consume_count"),
		consumeErr:   lookupOrNewMap("vef_event_consume_error"),
		consumeMs:    lookupOrNewMap("vef_event_consume_latency_ms"),
	}
}

// PublishObserved implements event.MetricsRecorder.
func (r *ExpvarMetricsRecorder) PublishObserved(eventType string, err error) {
	key := normalizeKey(eventType)
	r.publishCount.Add(key, 1)

	if err != nil {
		r.publishErr.Add(key, 1)
	}
}

// ConsumeObserved implements event.MetricsRecorder.
func (r *ExpvarMetricsRecorder) ConsumeObserved(eventType string, elapsed time.Duration, err error) {
	key := normalizeKey(eventType)
	r.consumeCount.Add(key, 1)
	r.consumeMs.Add(key, elapsed.Milliseconds())

	if err != nil {
		r.consumeErr.Add(key, 1)
	}
}

// lookupOrNewMap returns an existing expvar.Map by name or creates one;
// expvar.Publish panics on duplicate names so we guard against it for
// repeated NewExpvarMetricsRecorder calls in tests.
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
