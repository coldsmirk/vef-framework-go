package null

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// Date is a nullable timex.Date. It supports SQL and JSON serialization.
// It will marshal to null if null.
type Date struct {
	sql.Null[timex.Date]
}

// NewDate creates a new Date.
func NewDate(d timex.Date, valid bool) Date {
	return Date{
		Null: sql.Null[timex.Date]{
			V:     d,
			Valid: valid,
		},
	}
}

// DateFrom creates a new Date that will always be valid.
func DateFrom(d timex.Date) Date {
	return NewDate(d, true)
}

// DateFromPtr creates a new Date that will be null if d is nil.
func DateFromPtr(d *timex.Date) Date {
	if d == nil {
		return NewDate(timex.Date{}, false)
	}

	return NewDate(*d, true)
}

// ValueOrZero returns the inner value if valid, otherwise zero.
func (d Date) ValueOrZero() timex.Date {
	if !d.Valid {
		return timex.Date{}
	}

	return d.V
}

// ValueOr returns the inner value if valid, otherwise v.
func (d Date) ValueOr(v timex.Date) timex.Date {
	if !d.Valid {
		return v
	}

	return d.V
}

// MarshalJSON implements json.Marshaler.
// It will encode null if this date is null.
func (d Date) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return jsonNullBytes, nil
	}

	return d.V.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports string and null input.
func (d *Date) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == 'n' {
		d.Valid = false

		return nil
	}

	if err := json.Unmarshal(data, &d.V); err != nil {
		return fmt.Errorf("null: couldn't unmarshal JSON: %w", err)
	}

	d.Valid = true

	return nil
}

// MarshalText implements encoding.TextMarshaler.
// It returns an empty string if invalid, otherwise timex.Date's MarshalText.
func (d Date) MarshalText() ([]byte, error) {
	if !d.Valid {
		return []byte{}, nil
	}

	return d.V.MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It has backwards compatibility with v3 in that the string "null" is considered equivalent to an empty string
// and unmarshaling will succeed. This may be removed in a future version.
func (d *Date) UnmarshalText(text []byte) error {
	str := string(text)
	// allowing "null" is for backwards compatibility with v3
	if str == "" || str == jsonNull {
		d.Valid = false

		return nil
	}

	if err := d.V.UnmarshalText(text); err != nil {
		return fmt.Errorf("null: couldn't unmarshal text: %w", err)
	}

	d.Valid = true

	return nil
}

// SetValid changes this Date's value and sets it to be non-null.
func (d *Date) SetValid(v timex.Date) {
	d.V = v
	d.Valid = true
}

// Ptr returns a pointer to this Date's value, or a nil pointer if this Date is null.
func (d Date) Ptr() *timex.Date {
	if !d.Valid {
		return nil
	}

	return &d.V
}

// IsZero returns true for invalid Dates, hopefully for future omitempty support.
// A non-null Date with a zero value will not be considered zero.
func (d Date) IsZero() bool {
	return !d.Valid
}

// Equal returns true if both Date objects encode the same date or are both null.
// Two dates can be equal even if they are in different locations.
// For example, 2023-01-01 +0200 CEST and 2023-01-01 UTC are Equal.
func (d Date) Equal(other Date) bool {
	return d.Valid == other.Valid && (!d.Valid || d.V.Equal(other.V))
}

// ExactEqual returns true if both Date objects are equal or both null.
// ExactEqual returns false for dates that are in different locations or
// have a different monotonic clock reading.
func (d Date) ExactEqual(other Date) bool {
	return d.Valid == other.Valid && (!d.Valid || d.V == other.V)
}
