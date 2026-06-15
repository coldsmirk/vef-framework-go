package event

import (
	"encoding/json"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// encodeFrame serializes an Envelope into a Frame. The Payload is always
// JSON-encoded into Frame.Body so every transport ships the same
// wire-compatible form; decodeFrame reconstructs it as a RawPayload. The
// live Event reference is intentionally not carried through the Frame, so
// in-process delivery pays the same marshal/unmarshal round trip as the
// cross-process path.
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
