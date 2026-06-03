package decimal

import "github.com/shopspring/decimal"

// Decimal is an alias for decimal.Decimal.
type Decimal = decimal.Decimal

// Common decimal constants.
var (
	Zero = decimal.Zero
	One  = decimal.NewFromInt(1)
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
