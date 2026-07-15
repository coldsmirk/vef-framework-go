package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coldsmirk/vef-framework-go/version"
)

// defaultUserAgent identifies the framework client unless the application
// sets its own User-Agent header.
const defaultUserAgent = "vef/" + version.VEFVersion

// Client is an HTTP client for calling one upstream service — construct one
// per third-party system. A Client is immutable after New and safe for
// concurrent use; per-call state lives in the Request created by NewRequest.
type Client struct {
	httpClient      *http.Client
	baseURL         *url.URL
	timeout         time.Duration
	header          http.Header
	query           url.Values
	authorization   string
	retry           *RetryConfig
	maxResponseBody int64
	requestHooks    []RequestHook
	responseHooks   []ResponseHook
}

// New builds a Client, validating options eagerly: a malformed base or proxy
// URL and conflicting transport-level options fail construction. The
// zero-option client is ready to use — no base URL, a 30s call timeout, no
// retries.
func New(opts ...Option) (*Client, error) {
	cfg := clientConfig{timeout: defaultTimeout, maxRedirects: defaultMaxRedirects}
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var base *url.URL

	if cfg.baseURL != "" {
		parsed, err := url.Parse(cfg.baseURL)
		if err != nil || !parsed.IsAbs() {
			return nil, fmt.Errorf("%w: base URL %q must be absolute", ErrInvalidOption, cfg.baseURL)
		}

		base = parsed
	}

	httpClient, err := cfg.buildHTTPClient()
	if err != nil {
		return nil, err
	}

	var retry *RetryConfig

	if cfg.retry != nil {
		normalized := cfg.retry.withDefaults()
		retry = &normalized
	}

	return &Client{
		httpClient:      httpClient,
		baseURL:         base,
		timeout:         cfg.timeout,
		header:          cfg.header,
		query:           cfg.query,
		authorization:   cfg.authorization,
		retry:           retry,
		maxResponseBody: cfg.maxResponseBody,
		requestHooks:    cfg.requestHooks,
		responseHooks:   cfg.responseHooks,
	}, nil
}

// NewRequest starts a single-use request builder bound to this client.
func (c *Client) NewRequest() *Request {
	return &Request{
		client: c,
		header: make(http.Header),
		query:  make(url.Values),
	}
}

// validate rejects invalid option values and mutually exclusive options.
func (c *clientConfig) validate() error {
	if c.httpClient != nil && (c.transport != nil || c.proxyURL != "" || c.tlsConfig != nil || c.cookieJar != nil || c.maxRedirectsSet) {
		return fmt.Errorf("%w: WithHTTPClient takes over transport, redirects, and cookies", ErrConflictingOptions)
	}

	if c.transport != nil && (c.proxyURL != "" || c.tlsConfig != nil) {
		return fmt.Errorf("%w: WithTransport owns proxy and TLS settings", ErrConflictingOptions)
	}

	if c.maxRedirects < 0 {
		return fmt.Errorf("%w: negative redirect cap %d", ErrInvalidOption, c.maxRedirects)
	}

	return nil
}

// buildHTTPClient assembles the underlying *http.Client from the
// transport-level options, or returns the application-supplied one verbatim.
func (c *clientConfig) buildHTTPClient() (*http.Client, error) {
	if c.httpClient != nil {
		return c.httpClient, nil
	}

	transport := c.transport
	if transport == nil {
		base := http.DefaultTransport.(*http.Transport).Clone()

		if c.proxyURL != "" {
			proxy, err := url.Parse(c.proxyURL)
			if err != nil || !proxy.IsAbs() {
				return nil, fmt.Errorf("%w: proxy URL %q must be absolute", ErrInvalidOption, c.proxyURL)
			}

			base.Proxy = http.ProxyURL(proxy)
		}

		if c.tlsConfig != nil {
			base.TLSClientConfig = c.tlsConfig
		}

		transport = base
	}

	return &http.Client{
		Transport:     transport,
		Jar:           c.cookieJar,
		CheckRedirect: redirectPolicy(c.maxRedirects),
	}, nil
}

// redirectPolicy caps redirect following at limit; a zero limit returns the
// redirect response itself instead of following it.
func redirectPolicy(limit int) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if limit == 0 {
			return http.ErrUseLastResponse
		}

		if len(via) >= limit {
			return fmt.Errorf("%w: stopped after %d redirects", ErrTooManyRedirects, limit)
		}

		return nil
	}
}
