package adapter

import (
	"errors"
)

var (
	// ErrHandlerFactoryInvalidReturn indicates handler factory invalid return count.
	ErrHandlerFactoryInvalidReturn = errors.New("handler factory method should return 1 or 2 values")
	// ErrHandlerFactoryReturnNotError indicates handler factory second return value is not an error.
	ErrHandlerFactoryReturnNotError = errors.New("handler factory second return value is not an error")
	// ErrHandlerReturnNotError indicates handler return value is not an error.
	ErrHandlerReturnNotError = errors.New("handler return value is not an error")
)
