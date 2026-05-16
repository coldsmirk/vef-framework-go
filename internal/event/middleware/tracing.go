package middleware

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/id"
)

// Tracing propagates a trace identifier across the publish/consume
// boundary. On publish, a missing TraceID is filled with a fresh UUID
// and stashed into Headers under the W3C-compatible "traceparent" key.
// On consume, the TraceID is restored onto the context so downstream
// loggers can attach it to every record.
type Tracing struct{}

// NewTracing constructs a Tracing middleware.
func NewTracing() *Tracing { return new(Tracing) }

// Name implements both middleware interfaces.
func (*Tracing) Name() string { return "tracing" }

// Applies always returns true; tracing is cross-cutting.
func (*Tracing) Applies(transport.Capabilities) bool { return true }

const traceHeaderKey = "traceparent"

type (
	traceIDKey         struct{}
	incomingTraceIDKey struct{}
)

// WrapPublish injects a trace ID into the envelope.
func (*Tracing) WrapPublish(next pubmw.PublishHandler) pubmw.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		if env.TraceID == "" {
			env.TraceID = id.GenerateUUID()
		}

		if env.Headers == nil {
			env.Headers = make(map[string]string, 1)
		}

		if _, ok := env.Headers[traceHeaderKey]; !ok {
			env.Headers[traceHeaderKey] = env.TraceID
		}

		return next(ctx, env)
	}
}

// WrapConsume restores the trace ID into the context. Trace IDs
// arriving from cross-process transports are treated as untrusted and
// stored under a separate ctx key (IncomingTraceIDFromContext); the
// active trace context is always freshly generated so log/audit
// correlation cannot be spoofed by a hostile publisher.
func (*Tracing) WrapConsume(next pubmw.ConsumeHandler) pubmw.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		incoming := env.TraceID
		if incoming == "" {
			incoming = env.Headers[traceHeaderKey]
		}

		if incoming != "" {
			ctx = context.WithValue(ctx, incomingTraceIDKey{}, incoming)
		}

		ctx = context.WithValue(ctx, traceIDKey{}, id.GenerateUUID())

		return next(ctx, d, env)
	}
}

// TraceIDFromContext returns the locally-generated trace ID stamped
// onto ctx by the Tracing consume middleware, or "" when absent. It is
// always safe to log; do not rely on it matching any caller-supplied
// header value.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(traceIDKey{}).(string)

	return v
}

// IncomingTraceIDFromContext returns the trace ID supplied by the
// cross-process producer (when present). Treat this value as untrusted
// — log it under a distinct field (e.g. "incoming_trace_id") so audit
// pipelines can differentiate forged versus framework-generated IDs.
func IncomingTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(incomingTraceIDKey{}).(string)

	return v
}
