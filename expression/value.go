package expression

import (
	"encoding/json"
	"fmt"
)

// Value is the backend-agnostic result of an evaluation. It wraps a plain Go
// value and never exposes any backend type, so consumers stay decoupled from
// the underlying engine.
type Value struct {
	raw any
}

// NewValue wraps a raw evaluated value. It is the construction point for
// backend adapters; ordinary callers obtain Values from Engine or Program.
func NewValue(raw any) Value {
	return Value{raw: raw}
}

// Interface returns the underlying value as produced by the backend.
func (v Value) Interface() any {
	return v.raw
}

// IsNil reports whether the evaluation produced a null/absent value.
func (v Value) IsNil() bool {
	return v.raw == nil
}

// Bool returns the value as a bool, or an error if it is not boolean.
func (v Value) Bool() (bool, error) {
	b, ok := v.raw.(bool)
	if !ok {
		return false, fmt.Errorf("%w: expected bool, got %T", ErrUnexpectedType, v.raw)
	}

	return b, nil
}

// Decode unmarshals the value into target, which must be a non-nil pointer.
// Conversion goes through JSON, matching how backends serialize results;
// integers beyond float64 precision may lose accuracy.
func (v Value) Decode(target any) error {
	data, err := json.Marshal(v.raw)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

// DecodeValue decodes v into a new T. It exists because Go interfaces cannot
// declare generic methods.
func DecodeValue[T any](v Value) (T, error) {
	var out T
	if err := v.Decode(&out); err != nil {
		return out, err
	}

	return out, nil
}
