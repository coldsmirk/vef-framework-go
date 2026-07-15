package jshttp

import (
	"net/http"
	"time"
)

// libConfig collects the settings resolved from Options. The zero value
// applies no restrictions: no timeout, no body size cap, every host and
// network reachable. Each restriction activates only through its option.
type libConfig struct {
	client       *http.Client
	timeout      time.Duration
	maxBodySize  int64
	allowedHosts []string
	publicOnly   bool
}

// Option customizes the http library.
type Option func(*libConfig)

// WithClient supplies a custom http.Client, replacing the built-in client
// entirely. URL-level checks (scheme, host allowlist, literal public-only
// addresses) still apply to every request, but the dial-time address guard
// and the per-request redirect modes ride on the built-in client only — a
// custom client brings its own transport and redirect policy.
func WithClient(client *http.Client) Option {
	return func(c *libConfig) {
		c.client = client
	}
}

// WithTimeout restricts every request to d. It is both the default and the
// upper bound: a script-supplied timeout may only shorten it. Without this
// option requests are bounded only by the runtime's execution context.
func WithTimeout(d time.Duration) Option {
	return func(c *libConfig) {
		c.timeout = d
	}
}

// WithMaxBodySize caps the response body size in bytes; larger responses fail
// the request instead of being silently truncated. Without this option the
// body size is unbounded.
func WithMaxBodySize(n int64) Option {
	return func(c *libConfig) {
		c.maxBodySize = n
	}
}

// WithAllowedHosts restricts requests (including every redirect hop) to the
// given hostnames, matched case-insensitively and without ports.
func WithAllowedHosts(hosts ...string) Option {
	return func(c *libConfig) {
		c.allowedHosts = append(c.allowedHosts, hosts...)
	}
}

// WithPublicNetworkOnly enables the SSRF guard: requests to loopback, private
// (RFC 1918 / ULA), link-local (including the cloud metadata endpoint),
// multicast, and unspecified addresses are rejected. The guard validates the
// resolved address at dial time, so DNS rebinding cannot evade it.
func WithPublicNetworkOnly() Option {
	return func(c *libConfig) {
		c.publicOnly = true
	}
}
