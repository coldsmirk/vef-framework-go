package jshttp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// maxRedirects bounds every followed redirect chain of the built-in client.
const maxRedirects = 10

// newClient builds the built-in http.Client: transport defaults matching
// net/http, the dial-time SSRF guard when WithPublicNetworkOnly is set, and
// redirect handling (per-request modes, hop limit, policy re-validation).
func (l *lib) newClient() *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	if l.publicOnly {
		dialer.Control = guardDialAddress
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         dialer.DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: l.checkRedirect,
	}
}

// redirectModeKey carries the per-request redirect mode through the request
// context to checkRedirect.
type redirectModeKey struct{}

// checkRedirect implements the fetch redirect modes and re-validates every
// followed hop, so a permitted origin cannot bounce the script to a forbidden
// target.
func (l *lib) checkRedirect(req *http.Request, via []*http.Request) error {
	if mode, ok := req.Context().Value(redirectModeKey{}).(string); ok {
		switch mode {
		case redirectError:
			return ErrRedirectBlocked
		case redirectManual:
			return http.ErrUseLastResponse
		}
	}

	if len(via) >= maxRedirects {
		return ErrTooManyRedirects
	}

	return l.validateURL(req.URL)
}

// validateURL applies the request-level policy: the http/https scheme
// whitelist always, the host allowlist when configured, and — when the host
// is a literal IP under WithPublicNetworkOnly — the address guard.
// DNS-resolved addresses are checked at dial time by guardDialAddress.
func (l *lib) validateURL(u *url.URL) error {
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: %q", ErrSchemeNotAllowed, u.Scheme)
	}

	host := strings.ToLower(u.Hostname())
	if l.allowedHosts != nil && !l.allowedHosts.Contains(host) {
		return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}

	if l.publicOnly {
		if ip := net.ParseIP(host); ip != nil && forbiddenIP(ip) {
			return fmt.Errorf("%w: %s", ErrAddressNotAllowed, host)
		}
	}

	return nil
}

// guardDialAddress validates the resolved address right before the socket
// connects, so DNS rebinding and redirect tricks cannot smuggle a request
// into a forbidden network.
func guardDialAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	ip := net.ParseIP(host)
	if ip == nil || forbiddenIP(ip) {
		return fmt.Errorf("%w: %s", ErrAddressNotAllowed, host)
	}

	return nil
}

// forbiddenIP reports whether ip belongs to a range WithPublicNetworkOnly
// blocks: loopback, private (RFC 1918 / ULA), link-local (including the
// cloud metadata endpoint 169.254.169.254), multicast, and unspecified
// addresses.
func forbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}
