package event

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// TypedHandler is the strongly-typed handler signature accepted by
// SubscribeTyped. The framework decodes the wire payload into T before
// invoking the handler.
type TypedHandler[T Event] func(ctx context.Context, evt T, env Envelope) error

// SubscribeTyped registers a typed handler for events of type T. The
// framework deduces the topic from T's EventType() method and decodes
// cross-process payloads via reflect + json.Unmarshal. No global
// registry is involved — the type information is captured at the call
// site, which keeps tests independent of init() side effects.
//
// T is typically a pointer type (e.g. *FooEvent). The receiver of
// EventType must be safe on a nil value — implementations should
// return a string literal rather than dereference fields. Value types
// (e.g. SubscribeTyped[FooEvent]) are also supported provided
// EventType has a value receiver.
func SubscribeTyped[T Event](
	b Bus,
	h TypedHandler[T],
	opts ...SubscribeOption,
) (Unsubscribe, error) {
	var zero T

	// Discover the runtime type *before* invoking EventType, otherwise
	// instantiating with the bare Event interface would dereference a
	// nil interface value and panic instead of returning the documented
	// ErrNilTypeParameter sentinel.
	declared := reflect.TypeOf(zero)
	if declared == nil {
		return nil, ErrNilTypeParameter
	}

	eventType := zero.EventType()

	return b.Subscribe(eventType, func(ctx context.Context, env Envelope) error {
		evt, err := decodePayload[T](declared, env.Payload)
		if err != nil {
			return fmt.Errorf("event: subscribe %s: %w", eventType, err)
		}

		return h(ctx, evt, env)
	}, opts...)
}

// AsEvents converts a slice of any Event-implementing type into a
// []Event suitable for Bus.PublishBatch. Go's type system does not
// auto-convert []T to []Event even when T satisfies Event, so callers
// that maintain typed slices (e.g. domain-specific event lists) need
// this adapter at the publish boundary.
func AsEvents[T Event](items []T) []Event {
	out := make([]Event, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}

// decodePayload converts the envelope payload into the typed handler
// parameter. Three shapes are accepted:
//
//   - Payload is already a T (in-process delivery).
//   - Payload is a RawPayload carrying canonical JSON (cross-process).
//   - T is a value type whose pointer type also satisfies Event; the
//     decoded *Element is dereferenced to a T.
//
// Anything else is wrapped in ErrUnknownPayload.
func decodePayload[T Event](declared reflect.Type, payload Event) (T, error) {
	var zero T

	if typed, ok := payload.(T); ok {
		return typed, nil
	}

	raw, ok := payload.(RawPayload)
	if !ok {
		return zero, fmt.Errorf("%w: got %T", ErrUnknownPayload, payload)
	}

	// Always allocate the underlying element so json.Unmarshal has a
	// pointer to write into; pick pointer or dereferenced form per T.
	elemType := declared
	if elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}

	ptr := reflect.New(elemType)
	if err := json.Unmarshal(raw.Body, ptr.Interface()); err != nil {
		return zero, fmt.Errorf("decode: %w", err)
	}

	decoded := ptr.Interface()
	if declared.Kind() != reflect.Pointer {
		decoded = ptr.Elem().Interface()
	}

	typed, ok := decoded.(T)
	if !ok {
		return zero, fmt.Errorf("%w: decoded %T cannot satisfy %T", ErrUnknownPayload, decoded, zero)
	}

	return typed, nil
}
