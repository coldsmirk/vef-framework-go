package exec

import "errors"

var (
	// ErrAbsoluteURLNotAllowed rejects a script request carrying an absolute
	// URL: adapter scripts reach only their own system's base URL.
	ErrAbsoluteURLNotAllowed = errors.New("integration: absolute URLs are not allowed; use a path relative to the system base URL")

	// ErrRedirectModeUnsupported rejects the fetch redirect modes the scoped
	// client does not implement ("error" and "manual").
	ErrRedirectModeUnsupported = errors.New("integration: only the default 'follow' redirect mode is supported")

	// ErrOutputNotSerializable rejects a script return value that cannot be
	// represented as JSON (functions, cycles).
	ErrOutputNotSerializable = errors.New("integration: script output is not JSON-serializable")

	// ErrEnvelopeRequestNotObject rejects a request envelope script that did
	// not return the request object.
	ErrEnvelopeRequestNotObject = errors.New("integration: request envelope script must return the request object")
)

// upstreamError marks a failure the adapter script attributed to the
// external system via errors.upstream(message). It crosses the goja boundary
// wrapped in the script exception and is recovered with errors.As.
type upstreamError struct {
	message string
}

func (e *upstreamError) Error() string {
	return e.message
}

// ErrInvalidCodesOption rejects a codes.* options argument: more than one
// object, an empty object, several keys, a non-true flag, or an unknown key.
var ErrInvalidCodesOption = errors.New("codes: invalid options")

// codeMapError marks a code map fault raised by the codes library — a missing
// or disabled map, an unmapped value under the reject policy, or a stored map
// that no longer builds. It classifies as a configuration failure: the fix is
// a code map edit, not a script change.
type codeMapError struct {
	apiErr error
}

func (e *codeMapError) Error() string {
	return e.apiErr.Error()
}

// transportError marks a wire call that never completed. The scoped http
// library wraps client transport failures in it so the invoker can classify
// them apart from script bugs.
type transportError struct {
	err error
}

func (e *transportError) Error() string {
	return e.err.Error()
}

func (e *transportError) Unwrap() error {
	return e.err
}
