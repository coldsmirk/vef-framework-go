package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// Tracing propagates W3C trace context across the publish/consume
// boundary. On publish it ensures the envelope carries a 32-hex trace ID
// and a fresh 16-hex span ID, and stamps a valid `traceparent` header
// (version 00). On consume it restores the trace ID onto the context so
// downstream loggers and handlers can correlate every record.
//
// Two modes are supported:
//
//   - Default (NewTracing): incoming trace IDs are trusted and propagated
//     end-to-end, matching W3C / OpenTelemetry conventions. This is the
//     correct choice for receiving events inside a trust boundary
//     (intra-cluster RPC, internal pub/sub).
//   - Strict (NewTracingStrict): incoming trace IDs are treated as
//     untrusted. A fresh ID is generated for log correlation, the declared
//     incoming value is parked under a separate ctx key accessible via
//     IncomingTraceIDFromContext, and the envelope handed to the handler
//     carries the fresh ID — never the untrusted one. Use when accepting
//     events from less-trusted producers (multi-tenant ingress, public
//     webhook → outbox bridge).
type Tracing struct {
	strict bool
}

// NewTracing constructs a Tracing middleware that trusts incoming
// trace IDs (W3C-compatible propagation).
func NewTracing() *Tracing { return new(Tracing) }

// NewTracingStrict constructs a Tracing middleware that does not trust
// incoming trace IDs. A fresh trace ID is generated per consume and the
// declared incoming value is exposed only through IncomingTraceIDFromContext
// so audit pipelines can distinguish forged from framework-generated IDs.
func NewTracingStrict() *Tracing { return &Tracing{strict: true} }

// Name implements both middleware interfaces.
func (*Tracing) Name() string { return "tracing" }

// Order implements both middleware interfaces. Tracing runs early so the
// trace context it installs is visible to logging, metrics, and the
// handler.
func (*Tracing) Order() int { return middleware.OrderTracing }

// Applies always returns true; tracing is cross-cutting.
func (*Tracing) Applies(transport.Capabilities) bool { return true }

const (
	traceHeaderKey    = "traceparent"
	traceVersion      = "00"
	traceFlagsSampled = "01"
	traceIDHexLen     = 32 // 16 bytes
	spanIDHexLen      = 16 // 8 bytes
)

// WrapPublish ensures the envelope carries a valid W3C trace context and
// stamps the traceparent header.
func (*Tracing) WrapPublish(next middleware.PublishHandler) middleware.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		if env.TraceID == "" {
			env.TraceID = newTraceID()
		}

		// Each publish hop opens a new span.
		env.SpanID = newSpanID()

		if env.Headers == nil {
			env.Headers = make(map[string]string, 1)
		}

		env.Headers[traceHeaderKey] = formatTraceparent(env.TraceID, env.SpanID)

		return next(ctx, env)
	}
}

// WrapConsume restores the trace ID into the context. Behavior depends
// on the strict flag — see the Tracing type documentation.
func (m *Tracing) WrapConsume(next middleware.ConsumeHandler) middleware.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		incoming := env.TraceID
		if incoming == "" {
			incoming = parseTraceparent(env.Headers[traceHeaderKey])
		}

		if m.strict {
			if incoming != "" {
				ctx = middleware.WithIncomingTraceID(ctx, incoming)
			}

			// Generate a fresh, non-spoofable ID and overwrite the envelope
			// so neither the context nor the handler ever sees the untrusted
			// incoming value as the active trace.
			fresh := newTraceID()
			env.TraceID = fresh
			ctx = middleware.WithTraceID(ctx, fresh)

			return next(ctx, d, env)
		}

		trace := incoming
		if trace == "" {
			trace = newTraceID()
		}

		env.TraceID = trace
		ctx = middleware.WithTraceID(ctx, trace)

		return next(ctx, d, env)
	}
}

// newTraceID returns a random 16-byte (32 hex) W3C trace ID.
func newTraceID() string { return randomHex(traceIDHexLen / 2) }

// newSpanID returns a random 8-byte (16 hex) W3C span ID.
func newSpanID() string { return randomHex(spanIDHexLen / 2) }

// randomHex returns the hex encoding of n random bytes. crypto/rand.Read
// does not fail on supported platforms; the error is intentionally
// discarded so this cross-cutting path never panics.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

// formatTraceparent builds a W3C traceparent value:
// version "-" trace-id "-" span-id "-" flags.
func formatTraceparent(traceID, spanID string) string {
	return traceVersion + "-" + traceID + "-" + spanID + "-" + traceFlagsSampled
}

// parseTraceparent extracts the trace ID from a W3C traceparent header,
// returning "" when the value is absent or malformed.
func parseTraceparent(v string) string {
	if v == "" {
		return ""
	}

	parts := strings.Split(v, "-")
	if len(parts) != 4 {
		return ""
	}

	version, traceID, spanID, flags := parts[0], parts[1], parts[2], parts[3]
	if len(version) != 2 || len(traceID) != traceIDHexLen || len(spanID) != spanIDHexLen || len(flags) != 2 {
		return ""
	}

	if !isHex(traceID) || !isHex(spanID) {
		return ""
	}

	return traceID
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)

	return err == nil
}
