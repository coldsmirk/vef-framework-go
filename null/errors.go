package null

import "errors"

// ErrInvalidBoolText indicates invalid text for null.Bool UnmarshalText.
var ErrInvalidBoolText = errors.New("null: invalid input for UnmarshalText")
