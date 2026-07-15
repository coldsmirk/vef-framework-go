package auth

import "errors"

// ErrMissingParam indicates a required auth parameter is absent or empty.
var ErrMissingParam = errors.New("integration auth: missing parameter")
