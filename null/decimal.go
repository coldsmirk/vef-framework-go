package null

import (
	dec "github.com/shopspring/decimal"

	"github.com/coldsmirk/vef-framework-go/decimal"
)

// Decimal is a nullable decimal.Decimal. It supports SQL and JSON serialization.
// It will marshal to null if null.
type Decimal struct {
	dec.NullDecimal
}

// NewDecimal creates a new Decimal.
func NewDecimal(d decimal.Decimal, valid bool) Decimal {
	return Decimal{
		dec.NullDecimal{
			Decimal: d,
			Valid:   valid,
		},
	}
}

// DecimalFrom creates a new Decimal that will always be valid.
func DecimalFrom(d decimal.Decimal) Decimal {
	return NewDecimal(d, true)
}

// DecimalFromPtr creates a new Decimal that will be null if d is nil.
func DecimalFromPtr(d *decimal.Decimal) Decimal {
	if d == nil {
		return NewDecimal(decimal.Zero, false)
	}

	return NewDecimal(*d, true)
}

// ValueOrZero returns the inner value if valid, otherwise zero.
func (d Decimal) ValueOrZero() decimal.Decimal {
	if !d.Valid {
		return decimal.Zero
	}

	return d.Decimal
}

// ValueOr returns the inner value if valid, otherwise v.
func (d Decimal) ValueOr(v decimal.Decimal) decimal.Decimal {
	if !d.Valid {
		return v
	}

	return d.Decimal
}

// SetValid changes this Decimal's value and sets it to be non-null.
func (d *Decimal) SetValid(v decimal.Decimal) {
	d.Decimal = v
	d.Valid = true
}

// Ptr returns a pointer to this Decimal's value, or a nil pointer if this Decimal is null.
func (d Decimal) Ptr() *decimal.Decimal {
	if !d.Valid {
		return nil
	}

	return &d.Decimal
}

// IsZero returns true for invalid Decimals, for future omitempty support.
// A non-null Decimal with a zero value will not be considered zero.
func (d Decimal) IsZero() bool {
	return !d.Valid
}

// Equal returns true if both decimals have the same value or are both null.
func (d Decimal) Equal(other Decimal) bool {
	return d.Valid == other.Valid && (!d.Valid || d.Decimal.Equal(other.Decimal))
}

// ExactEqual returns true if both Decimal objects are exactly equal or both null.
// Unlike Equal, this requires the underlying representation to be identical.
func (d Decimal) ExactEqual(other Decimal) bool {
	return d.Valid == other.Valid && (!d.Valid || d.Decimal == other.Decimal)
}
