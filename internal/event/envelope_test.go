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
	require.NoError(t, err)

	require.Equal(t, env.ID, frame.ID)
	require.Equal(t, env.Type, frame.Type)
	require.Equal(t, env.Source, frame.Source)
	require.Equal(t, env.OccurredAt, frame.OccurredAt)
	require.Equal(t, env.TraceID, frame.TraceID)
	require.Equal(t, env.CorrelationID, frame.CorrelationID)
	require.Equal(t, env.Headers, frame.Headers)

	var back EnvEvent
	require.NoError(t, json.Unmarshal(frame.Body, &back))
	require.Equal(t, "round", back.Value, "frame body must carry the JSON-encoded payload")
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
	require.Equal(t, frame.ID, env.ID)
	require.Equal(t, frame.Type, env.Type)

	raw, ok := env.Payload.(event.RawPayload)
	require.True(t, ok, "decodeFrame must materialize Payload as RawPayload for cross-process delivery")
	require.Equal(t, frame.Type, raw.Type)
	require.JSONEq(t, string(body), string(raw.Body))
}
