package httpx

import "errors"

var (
	// ErrInvalidOption reports a client option carrying an invalid value,
	// such as a malformed base or proxy URL.
	ErrInvalidOption = errors.New("httpx: invalid option")

	// ErrConflictingOptions reports mutually exclusive client options, such
	// as WithHTTPClient combined with transport-level options.
	ErrConflictingOptions = errors.New("httpx: conflicting options")

	// ErrInvalidRequestURL reports a request URL that cannot be parsed, or a
	// relative URL used on a client that has no base URL.
	ErrInvalidRequestURL = errors.New("httpx: invalid request URL")

	// ErrMissingPathParam reports a ":name" path segment left unresolved
	// because no value was supplied via SetPathParam.
	ErrMissingPathParam = errors.New("httpx: missing path parameter")

	// ErrRequestReused reports a second execution of a single-use Request.
	ErrRequestReused = errors.New("httpx: request already executed")

	// ErrTooManyRedirects reports a call exceeding the redirect cap set via
	// WithMaxRedirects.
	ErrTooManyRedirects = errors.New("httpx: too many redirects")

	// ErrResponseTooLarge reports a response body exceeding the cap set via
	// WithMaxResponseBody.
	ErrResponseTooLarge = errors.New("httpx: response body too large")
)
