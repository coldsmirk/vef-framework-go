package middleware

import "context"

type (
	traceIDKey         struct{}
	incomingTraceIDKey struct{}
)

// WithTraceID stores the active trace ID on ctx. The Tracing consume
// middleware sets this; handlers read it via TraceIDFromContext.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// WithIncomingTraceID parks the producer-supplied (untrusted) trace ID on
// ctx. Strict-mode tracing uses this so audit pipelines can distinguish a
// forged incoming ID from the framework-generated active ID.
func WithIncomingTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, incomingTraceIDKey{}, traceID)
}

// TraceIDFromContext returns the active trace ID stamped onto ctx by the
// Tracing consume middleware. In the default (trusting) mode this is the
// value the producer sent; in strict mode it is a freshly generated,
// non-spoofable ID.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(traceIDKey{}).(string)

	return v
}

// IncomingTraceIDFromContext returns the trace ID supplied by the
// cross-process producer when strict mode is in effect; in the default
// (trusting) mode it returns "" because the producer-supplied value is
// already used as the active TraceIDFromContext.
func IncomingTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v, _ := ctx.Value(incomingTraceIDKey{}).(string)

	return v
}
