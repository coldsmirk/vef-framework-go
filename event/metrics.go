package event

import "time"

// MetricsRecorder receives publish and consume observations from the
// framework's built-in Metrics middleware. The default implementation
// publishes lightweight expvar maps; applications that need richer
// instrumentation (Prometheus, OpenTelemetry, vendor SDKs) can supply
// their own via fx.Decorate.
//
// Implementations must be safe for concurrent use. The framework
// invokes these hooks from the request-serving goroutine, so they
// should be O(1) and non-blocking.
type MetricsRecorder interface {
	// PublishObserved is called once per publish attempt with the
	// resolved event type and the publish outcome. err is non-nil when
	// the underlying transport rejected the frame.
	PublishObserved(eventType string, err error)
	// ConsumeObserved is called once per consume attempt with the
	// resolved event type, the handler's wall-clock elapsed time, and
	// the outcome.
	ConsumeObserved(eventType string, elapsed time.Duration, err error)
}
