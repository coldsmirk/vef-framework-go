package mapx

import "errors"

var (
	// ErrInvalidToMapValue indicates the value passed to ToMap is not a struct.
	ErrInvalidToMapValue = errors.New("the value of ToMap function must be a struct")
	// ErrInvalidFromMapType indicates the type parameter of FromMap is not a struct.
	ErrInvalidFromMapType = errors.New("the type parameter of FromMap function must be a struct")
	// ErrValueOrZeroMethodNotFound indicates ValueOrZero method is missing on null.Value type.
	ErrValueOrZeroMethodNotFound = errors.New("ValueOrZero method not found on null.Value type")
)
