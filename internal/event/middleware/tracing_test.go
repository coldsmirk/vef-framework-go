package middleware

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

func TestTracing(t *testing.T) {
	t.Run("Order", func(t *testing.T) {
		require.Equal(t, pubmw.OrderTracing, NewTracing().Order(), "tracing Order must equal OrderTracing")
	})

	t.Run("PublishStampsValidW3CContext", func(t *testing.T) {
		var captured *event.Envelope

		h := NewTracing().WrapPublish(func(_ context.Context, env *event.Envelope) error {
			captured = env

			return nil
		})

		require.NoError(t, h(context.Background(), &event.Envelope{Type: "test.created"}), "publish wrapper should succeed")

		require.Len(t, captured.TraceID, traceIDHexLen, "trace ID should be 32 hex chars")
		require.Len(t, captured.SpanID, spanIDHexLen, "span ID should be 16 hex chars")
		assertHex(t, captured.TraceID)
		assertHex(t, captured.SpanID)

		require.Equal(t, "00-"+captured.TraceID+"-"+captured.SpanID+"-01", captured.Headers[traceHeaderKey],
			"traceparent header must be a valid W3C value")
	})

	t.Run("PublishKeepsExistingTraceIDButFreshSpan", func(t *testing.T) {
		existing := newTraceID()

		var captured *event.Envelope

		h := NewTracing().WrapPublish(func(_ context.Context, env *event.Envelope) error {
			captured = env

			return nil
		})

		require.NoError(t, h(context.Background(), &event.Envelope{TraceID: existing}), "publish should succeed")
		require.Equal(t, existing, captured.TraceID, "existing trace ID must be preserved")
		require.Len(t, captured.SpanID, spanIDHexLen, "a fresh span ID must be set")
	})

	t.Run("ConsumeDefaultTrustsIncoming", func(t *testing.T) {
		incoming := newTraceID()

		var (
			seen       string
			handlerEnv event.Envelope
		)

		h := NewTracing().WrapConsume(func(ctx context.Context, _ transport.Delivery, env event.Envelope) error {
			seen = pubmw.TraceIDFromContext(ctx)
			handlerEnv = env

			return nil
		})

		require.NoError(t, h(context.Background(), nil, event.Envelope{TraceID: incoming}), "consume should succeed")
		require.Equal(t, incoming, seen, "default mode must trust the incoming trace ID")
		require.Equal(t, incoming, handlerEnv.TraceID, "default mode leaves the trusted trace on the envelope")
	})

	t.Run("ConsumeDefaultParsesTraceparentHeader", func(t *testing.T) {
		traceID := newTraceID()

		var seen string

		h := NewTracing().WrapConsume(func(ctx context.Context, _ transport.Delivery, _ event.Envelope) error {
			seen = pubmw.TraceIDFromContext(ctx)

			return nil
		})

		env := event.Envelope{Headers: map[string]string{traceHeaderKey: formatTraceparent(traceID, newSpanID())}}
		require.NoError(t, h(context.Background(), nil, env), "consume should succeed")
		require.Equal(t, traceID, seen, "trace ID must be parsed from the traceparent header")
	})

	t.Run("ConsumeDefaultGeneratesWhenAbsent", func(t *testing.T) {
		var seen string

		h := NewTracing().WrapConsume(func(ctx context.Context, _ transport.Delivery, _ event.Envelope) error {
			seen = pubmw.TraceIDFromContext(ctx)

			return nil
		})

		require.NoError(t, h(context.Background(), nil, event.Envelope{}), "consume should succeed")
		require.Len(t, seen, traceIDHexLen, "a fresh trace ID must be generated when none is supplied")
	})

	t.Run("StrictGeneratesFreshAndParksIncoming", func(t *testing.T) {
		incoming := newTraceID()

		var (
			active     string
			parked     string
			handlerEnv event.Envelope
		)

		h := NewTracingStrict().WrapConsume(func(ctx context.Context, _ transport.Delivery, env event.Envelope) error {
			active = pubmw.TraceIDFromContext(ctx)
			parked = pubmw.IncomingTraceIDFromContext(ctx)
			handlerEnv = env

			return nil
		})

		require.NoError(t, h(context.Background(), nil, event.Envelope{TraceID: incoming}), "consume should succeed")
		require.Len(t, active, traceIDHexLen, "strict mode must generate a fresh active trace ID")
		require.NotEqual(t, incoming, active, "strict active trace must differ from the untrusted incoming")
		require.Equal(t, incoming, parked, "strict mode parks the incoming trace ID")
		require.Equal(t, active, handlerEnv.TraceID, "strict mode hands the handler the fresh trace, never the untrusted one")
	})
}

func TestParseTraceparent(t *testing.T) {
	valid := formatTraceparent(newTraceID(), newSpanID())

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"Valid", valid, valid[3 : 3+traceIDHexLen]},
		{"Empty", "", ""},
		{"WrongPartCount", "00-" + strings.Repeat("a", traceIDHexLen), ""},
		{"ShortTraceID", "00-abc-" + strings.Repeat("b", spanIDHexLen) + "-01", ""},
		{"NonHexTraceID", "00-" + strings.Repeat("z", traceIDHexLen) + "-" + strings.Repeat("b", spanIDHexLen) + "-01", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equalf(t, tc.want, parseTraceparent(tc.in), "parseTraceparent(%q)", tc.in)
		})
	}
}

func assertHex(t *testing.T, s string) {
	t.Helper()

	_, err := hex.DecodeString(s)
	require.NoErrorf(t, err, "value %q should be valid hex", s)
}
