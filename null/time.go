package null

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// Time is a nullable timex.Time. It supports SQL and JSON serialization.
// It will marshal to null if null.
type Time struct {
	sql.Null[timex.Time]
}

// NewTime creates a new Time.
func NewTime(t timex.Time, valid bool) Time {
	return Time{
		Null: sql.Null[timex.Time]{
			V:     t,
			Valid: valid,
		},
	}
}

// TimeFrom creates a new Time that will always be valid.
func TimeFrom(t timex.Time) Time {
	return NewTime(t, true)
}

// TimeFromPtr creates a new Time that will be null if t is nil.
func TimeFromPtr(t *timex.Time) Time {
	if t == nil {
		return NewTime(timex.Time{}, false)
	}

	return NewTime(*t, true)
}

// ValueOrZero returns the inner value if valid, otherwise zero.
func (t Time) ValueOrZero() timex.Time {
	if !t.Valid {
		return timex.Time{}
	}

	return t.V
}

// ValueOr returns the inner value if valid, otherwise v.
func (t Time) ValueOr(v timex.Time) timex.Time {
	if !t.Valid {
		return v
	}

	return t.V
}

// MarshalJSON implements json.Marshaler.
// It will encode null if this time is null.
func (t Time) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return jsonNullBytes, nil
	}

	return t.V.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports string and null input.
func (t *Time) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == 'n' {
		t.Valid = false

		return nil
	}

	if err := json.Unmarshal(data, &t.V); err != nil {
		return fmt.Errorf("null: couldn't unmarshal JSON: %w", err)
	}

	t.Valid = true

	return nil
}

// MarshalText implements encoding.TextMarshaler.
// It returns an empty string if invalid, otherwise timex.Time's MarshalText.
func (t Time) MarshalText() ([]byte, error) {
	if !t.Valid {
		return []byte{}, nil
	}

	return t.V.MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It has backwards compatibility with v3 in that the string "null" is considered equivalent to an empty string
// and unmarshaling will succeed. This may be removed in a future version.
func (t *Time) UnmarshalText(text []byte) error {
	str := string(text)
	// allowing "null" is for backwards compatibility with v3
	if str == "" || str == jsonNull {
		t.Valid = false

		return nil
	}

	if err := t.V.UnmarshalText(text); err != nil {
		return fmt.Errorf("null: couldn't unmarshal text: %w", err)
	}

	t.Valid = true

	return nil
}

// SetValid changes this Time's value and sets it to be non-null.
func (t *Time) SetValid(v timex.Time) {
	t.V = v
	t.Valid = true
}

// Ptr returns a pointer to this Time's value, or a nil pointer if this Time is null.
func (t Time) Ptr() *timex.Time {
	if !t.Valid {
		return nil
	}

	return &t.V
}

// IsZero returns true for invalid Times, hopefully for future omitempty support.
// A non-null Time with a zero value will not be considered zero.
func (t Time) IsZero() bool {
	return !t.Valid
}

// Equal returns true if both Time objects encode the same time or are both null.
// Two times can be equal even if they are in different locations.
// For example, 6:00 +0200 CEST and 4:00 UTC are Equal.
func (t Time) Equal(other Time) bool {
	return t.Valid == other.Valid && (!t.Valid || t.V.Equal(other.V))
}

// ExactEqual returns true if both Time objects are equal or both null.
// ExactEqual returns false for times that are in different locations or
// have a different monotonic clock reading.
func (t Time) ExactEqual(other Time) bool {
	return t.Valid == other.Valid && (!t.Valid || t.V == other.V)
}
