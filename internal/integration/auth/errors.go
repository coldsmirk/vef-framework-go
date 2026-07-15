package auth

import "errors"

// ErrMissingParam indicates a required auth parameter is absent or empty.
var ErrMissingParam = errors.New("integration auth: missing parameter")

// ErrSigningScriptReturn rejects a signing script whose return value is not a
// header object.
var ErrSigningScriptReturn = errors.New("integration auth: signing script must return a header object")
