package httpx

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/url"
	"time"
)

const (
	// defaultTimeout bounds a whole call, retries included, when the
	// application does not choose its own bound via WithTimeout.
	defaultTimeout = 30 * time.Second

	// defaultMaxRedirects mirrors the net/http redirect cap.
	defaultMaxRedirects = 10
)

// RequestHook runs after a request is fully built — URL resolved, body
// marshaled — and before it is sent, so it can read the final shape and still
// adjust headers: the hook point for signing, audit, and logging. A returned
// error aborts the call.
type RequestHook func(req *Request) error

// ResponseHook runs after a response arrives and its body is buffered. A
// returned error fails the call.
type ResponseHook func(resp *Response) error

// clientConfig collects the settings resolved from Options.
type clientConfig struct {
	baseURL         string
	timeout         time.Duration
	header          http.Header
	query           url.Values
	authorization   string
	retry           *RetryConfig
	proxyURL        string
	tlsConfig       *tls.Config
	cookieJar       http.CookieJar
	maxRedirects    int
	maxRedirectsSet bool
	maxResponseBody int64
	requestHooks    []RequestHook
	responseHooks   []ResponseHook
	transport       http.RoundTripper
	httpClient      *http.Client
}

// Option customizes client construction.
type Option func(*clientConfig)

// WithBaseURL sets the absolute URL every relative request URL joins onto; a
// request supplying an absolute URL bypasses it.
func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) {
		c.baseURL = baseURL
	}
}

// WithTimeout bounds each whole call, retries included; an earlier context
// deadline still wins, and Request.SetTimeout overrides the bound per
// request. Zero removes it. The default is 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = d
	}
}

// WithHeader adds a default header sent with every request; a request-level
// header of the same name replaces it.
func WithHeader(key, value string) Option {
	return func(c *clientConfig) {
		if c.header == nil {
			c.header = make(http.Header)
		}

		c.header.Add(key, value)
	}
}

// WithQuery adds a default query parameter sent with every request, such as
// an API key; a request-level parameter of the same name replaces it.
func WithQuery(key, value string) Option {
	return func(c *clientConfig) {
		if c.query == nil {
			c.query = make(url.Values)
		}

		c.query.Add(key, value)
	}
}

// WithBasicAuth authenticates every request with HTTP Basic credentials. The
// last authentication option wins, and request-level authentication
// overrides it.
func WithBasicAuth(username, password string) Option {
	return func(c *clientConfig) {
		c.authorization = basicAuthorization(username, password)
	}
}

// WithBearerToken authenticates every request with a bearer token. The last
// authentication option wins, and request-level authentication overrides it.
func WithBearerToken(token string) Option {
	return func(c *clientConfig) {
		c.authorization = bearerAuthorization(token)
	}
}

// WithRetry enables automatic retries under the given policy; without this
// option every call makes a single attempt.
func WithRetry(cfg RetryConfig) Option {
	return func(c *clientConfig) {
		c.retry = &cfg
	}
}

// WithProxy routes every request through the given proxy URL. Incompatible
// with WithTransport and WithHTTPClient.
func WithProxy(proxyURL string) Option {
	return func(c *clientConfig) {
		c.proxyURL = proxyURL
	}
}

// WithTLSConfig sets the TLS client configuration. Incompatible with
// WithTransport and WithHTTPClient.
func WithTLSConfig(cfg *tls.Config) Option {
	return func(c *clientConfig) {
		c.tlsConfig = cfg
	}
}

// WithCookieJar stores and replays cookies across the client's requests.
// Incompatible with WithHTTPClient.
func WithCookieJar(jar http.CookieJar) Option {
	return func(c *clientConfig) {
		c.cookieJar = jar
	}
}

// WithMaxRedirects caps how many redirects a call follows (default 10); zero
// disables following so a 3xx response is returned as-is. Incompatible with
// WithHTTPClient.
func WithMaxRedirects(n int) Option {
	return func(c *clientConfig) {
		c.maxRedirects = n
		c.maxRedirectsSet = true
	}
}

// WithMaxResponseBody caps the buffered response body size in bytes; a
// larger body fails the call with ErrResponseTooLarge. The default is
// unbounded.
func WithMaxResponseBody(n int64) Option {
	return func(c *clientConfig) {
		c.maxResponseBody = n
	}
}

// WithRequestHook appends hooks run before each request is sent, in
// registration order.
func WithRequestHook(hooks ...RequestHook) Option {
	return func(c *clientConfig) {
		c.requestHooks = append(c.requestHooks, hooks...)
	}
}

// WithResponseHook appends hooks run after each response body is buffered,
// in registration order.
func WithResponseHook(hooks ...ResponseHook) Option {
	return func(c *clientConfig) {
		c.responseHooks = append(c.responseHooks, hooks...)
	}
}

// WithTransport sets the wire-level transport — the seam for tracing
// round-trippers, custom dialing, and test doubles. Incompatible with
// WithProxy, WithTLSConfig, and WithHTTPClient.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *clientConfig) {
		c.transport = rt
	}
}

// WithHTTPClient replaces the underlying *http.Client entirely, taking
// ownership of transport, redirects, and cookies. Incompatible with
// WithTransport, WithProxy, WithTLSConfig, WithCookieJar, and
// WithMaxRedirects.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = hc
	}
}

// basicAuthorization renders the Authorization header value for HTTP Basic
// credentials.
func basicAuthorization(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

// bearerAuthorization renders the Authorization header value for a bearer
// token.
func bearerAuthorization(token string) string {
	return "Bearer " + token
}
