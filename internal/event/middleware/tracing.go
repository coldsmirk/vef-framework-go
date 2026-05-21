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
//
// Two modes are supported:
//
//   - Default (NewTracing): incoming TraceIDs are trusted and propagated
//     end-to-end, matching W3C / OpenTelemetry conventions. This is the
//     correct choice for receiving events inside a trust boundary
//     (intra-cluster RPC, internal pub/sub).
//   - Strict (NewTracingStrict): incoming TraceIDs are treated as
//     untrusted. A fresh ID is generated for log correlation and the
//     declared incoming value is parked under a separate ctx key
//     accessible via IncomingTraceIDFromContext. Use when accepting
//     events from less-trusted producers (multi-tenant ingress, public
//     webhook → outbox bridge).
type Tracing struct {
	strict bool
}

// NewTracing constructs a Tracing middleware that trusts incoming
// TraceIDs (W3C-compatible propagation).
func NewTracing() *Tracing { return new(Tracing) }

// NewTracingStrict constructs a Tracing middleware that does not trust
// incoming TraceIDs. A fresh trace ID is generated per consume and the
// declared incoming value is exposed only through IncomingTraceIDFromContext
// so audit pipelines can distinguish forged from framework-generated IDs.
func NewTracingStrict() *Tracing { return &Tracing{strict: true} }

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

// WrapConsume restores the trace ID into the context. Behavior depends
// on the strict flag — see the Tracing type documentation.
func (m *Tracing) WrapConsume(next pubmw.ConsumeHandler) pubmw.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		incoming := env.TraceID
		if incoming == "" {
			incoming = env.Headers[traceHeaderKey]
		}

		if m.strict {
			if incoming != "" {
				ctx = context.WithValue(ctx, incomingTraceIDKey{}, incoming)
			}

			ctx = context.WithValue(ctx, traceIDKey{}, id.GenerateUUID())
		} else {
			trace := incoming
			if trace == "" {
				trace = id.GenerateUUID()
			}

			ctx = context.WithValue(ctx, traceIDKey{}, trace)
		}

		return next(ctx, d, env)
	}
}

// TraceIDFromContext returns the trace ID stamped onto ctx by the
// Tracing consume middleware. In the default (trusting) mode this is
// the same value the producer sent; in strict mode it is a freshly
// generated, non-spoofable ID.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(traceIDKey{}).(string)

	return v
}

// IncomingTraceIDFromContext returns the trace ID supplied by the
// cross-process producer when strict mode is in effect; in the default
// (trusting) mode this returns "" because the producer-supplied value
// is already used as the active TraceIDFromContext.
func IncomingTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(incomingTraceIDKey{}).(string)

	return v
}
