package jshttp

import "errors"

var (
	// ErrInvalidRequest is thrown into the script for a malformed call, e.g.
	// an unknown redirect mode.
	ErrInvalidRequest = errors.New("jshttp: invalid request")
	// ErrSchemeNotAllowed is thrown into the script for URL schemes other
	// than http and https.
	ErrSchemeNotAllowed = errors.New("jshttp: scheme not allowed")
	// ErrHostNotAllowed is thrown into the script when the target host is
	// outside the allowlist configured with WithAllowedHosts.
	ErrHostNotAllowed = errors.New("jshttp: host not allowed")
	// ErrAddressNotAllowed is thrown into the script when WithPublicNetworkOnly
	// is set and the target resolves to a loopback, private, link-local, or
	// unspecified address.
	ErrAddressNotAllowed = errors.New("jshttp: address not allowed")
	// ErrBodyTooLarge is thrown into the script when the response body
	// exceeds the limit configured with WithMaxBodySize.
	ErrBodyTooLarge = errors.New("jshttp: response body too large")
	// ErrRedirectBlocked is thrown into the script when a redirect arrives
	// under redirect mode "error".
	ErrRedirectBlocked = errors.New("jshttp: redirect blocked")
	// ErrTooManyRedirects is thrown into the script when a redirect chain
	// exceeds the redirect limit.
	ErrTooManyRedirects = errors.New("jshttp: too many redirects")
)
