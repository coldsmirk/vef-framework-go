package stream

import "errors"

var (
	ErrSourceRequired = errors.New("message source is required")
	ErrSourceClosed   = errors.New("message source is closed")
)
