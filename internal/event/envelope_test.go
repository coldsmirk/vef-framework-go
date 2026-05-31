package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

type EnvEvent struct {
	Value string `json:"value"`
}

func (*EnvEvent) EventType() string { return "envelope.test" }

func TestEncodeFrameSerializesPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	env := event.Envelope{
		ID:            "id-1",
		Type:          "envelope.test",
		Source:        "svc-A",
		OccurredAt:    now,
		PublishedAt:   now,
		TraceID:       "trace-1",
		CorrelationID: "corr-1",
		Headers:       map[string]string{"k": "v"},
		Payload:       &EnvEvent{Value: "round"},
	}

	frame, err := encodeFrame(env)
	require.NoError(t, err, "EncodeFrame should serialize the envelope payload")

	require.Equal(t, env.ID, frame.ID, "Encoded frame should preserve the envelope ID")
	require.Equal(t, env.Type, frame.Type, "Encoded frame should preserve the event type")
	require.Equal(t, env.Source, frame.Source, "Encoded frame should preserve the source")
	require.Equal(t, env.OccurredAt, frame.OccurredAt, "Encoded frame should preserve the occurrence time")
	require.Equal(t, env.TraceID, frame.TraceID, "Encoded frame should preserve the trace ID")
	require.Equal(t, env.CorrelationID, frame.CorrelationID, "Encoded frame should preserve the correlation ID")
	require.Equal(t, env.Headers, frame.Headers, "Encoded frame should preserve headers")

	var back EnvEvent
	require.NoError(t, json.Unmarshal(frame.Body, &back), "Encoded frame body should unmarshal into the event type")
	require.Equal(t, "round", back.Value, "Frame body must carry the JSON-encoded payload")
}

func TestDecodeFrameYieldsRawPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	body := []byte(`{"value":"trip"}`)

	frame := transport.Frame{
		ID:          "frame-1",
		Type:        "envelope.test",
		Source:      "svc-B",
		OccurredAt:  now,
		PublishedAt: now,
		Body:        body,
	}

	env := decodeFrame(frame)
	require.Equal(t, frame.ID, env.ID, "Decoded envelope should preserve the frame ID")
	require.Equal(t, frame.Type, env.Type, "Decoded envelope should preserve the frame type")

	raw, ok := env.Payload.(event.RawPayload)
	require.True(t, ok, "DecodeFrame must materialize Payload as RawPayload for cross-process delivery")
	require.Equal(t, frame.Type, raw.Type, "RawPayload should carry the frame type")
	require.JSONEq(t, string(body), string(raw.Body), "RawPayload body should preserve the frame body bytes")
}
