// Package decimal provides a wrapper around github.com/shopspring/decimal
// for arbitrary-precision fixed-point decimal numbers.
package decimal

import "github.com/shopspring/decimal"

// Decimal is an alias for decimal.Decimal.
type Decimal = decimal.Decimal

// Common decimal constants.
var (
	Zero  = decimal.Zero
	One   = decimal.NewFromInt(1)
	Two   = decimal.NewFromInt(2)
	Three = decimal.NewFromInt(3)
	Four  = decimal.NewFromInt(4)
	Five  = decimal.NewFromInt(5)
	Six   = decimal.NewFromInt(6)
	Seven = decimal.NewFromInt(7)
	Eight = decimal.NewFromInt(8)
	Nine  = decimal.NewFromInt(9)
	Ten   = decimal.NewFromInt(10)
)

// Constructors for creating Decimal values.
var (
	New                      = decimal.New
	NewFromFloat             = decimal.NewFromFloat
	NewFromFloat32           = decimal.NewFromFloat32
	NewFromFloatWithExponent = decimal.NewFromFloatWithExponent
	NewFromInt               = decimal.NewFromInt
	NewFromInt32             = decimal.NewFromInt32
	NewFromUint64            = decimal.NewFromUint64
	NewFromBigInt            = decimal.NewFromBigInt
	NewFromBigRat            = decimal.NewFromBigRat
	NewFromString            = decimal.NewFromString
	NewFromFormattedString   = decimal.NewFromFormattedString
	RequireFromString        = decimal.RequireFromString
)

// Utility functions for decimal operations.
var (
	Max         = decimal.Max
	Min         = decimal.Min
	Sum         = decimal.Sum
	Avg         = decimal.Avg
	RescalePair = decimal.RescalePair
)
