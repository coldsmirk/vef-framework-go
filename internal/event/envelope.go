package event

import (
	"encoding/json"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// encodeFrame serializes an Envelope into a Frame. The Payload is
// JSON-encoded so cross-process transports can ship the wire-compatible
// form; in-process transports may keep the original payload reference
// alongside via the Headers map for zero-copy delivery, but that
// optimisation is delegated to the transport implementation.
func encodeFrame(env event.Envelope) (transport.Frame, error) {
	body, err := json.Marshal(env.Payload)
	if err != nil {
		return transport.Frame{}, fmt.Errorf("encode payload %s: %w", env.Type, err)
	}

	return transport.Frame{
		ID:            env.ID,
		Type:          env.Type,
		Source:        env.Source,
		OccurredAt:    env.OccurredAt,
		PublishedAt:   env.PublishedAt,
		TraceID:       env.TraceID,
		SpanID:        env.SpanID,
		CorrelationID: env.CorrelationID,
		Headers:       env.Headers,
		Body:          body,
	}, nil
}

// decodeFrame reconstructs an Envelope from a Frame. The Payload is
// returned as RawPayload — consumers using SubscribeTyped[T] decode it
// further; untyped Handler consumers receive the raw bytes.
func decodeFrame(frame transport.Frame) event.Envelope {
	return event.Envelope{
		ID:            frame.ID,
		Type:          frame.Type,
		Source:        frame.Source,
		OccurredAt:    frame.OccurredAt,
		PublishedAt:   frame.PublishedAt,
		TraceID:       frame.TraceID,
		SpanID:        frame.SpanID,
		CorrelationID: frame.CorrelationID,
		Headers:       frame.Headers,
		Payload: event.RawPayload{
			Type: frame.Type,
			Body: frame.Body,
		},
	}
}
