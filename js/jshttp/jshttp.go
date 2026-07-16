package jshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/js"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "http"

// Redirect modes mirroring the fetch RequestInit "redirect" member.
const (
	redirectFollow = "follow"
	redirectError  = "error"
	redirectManual = "manual"
)

// lib exposes outbound HTTP requests to scripts as the global "http" object.
// The API is a synchronous take on the fetch standard:
//
//	http.fetch(url, { method, headers, query, body, redirect, timeout })
//	http.get(url, options?)      // sugar over fetch, likewise post/put/patch
//	http.post(url, body, options?)
//	http.delete(url, options?)
//
// Every call returns { status, statusText, ok, url, redirected, headers,
// body, text(), json(), arrayBuffer() }; failures are thrown as catchable
// exceptions. Response header names are lower-cased and multi-values joined
// with ", ", matching fetch Headers semantics. Beyond fetch: "query" appends
// URL parameters and "timeout" (milliseconds) replaces AbortSignal;
// "redirect" supports follow / error / manual. By default nothing is
// restricted — timeouts, body size caps, host allowlists, and the private
// network guard all activate only through Options.
type lib struct {
	client       *http.Client
	timeout      time.Duration
	maxBodySize  int64
	allowedHosts collections.Set[string]
	publicOnly   bool
}

// New builds the http library. Without options every reachable target is
// allowed and no resource limits apply; see Option for the opt-in
// restrictions.
func New(opts ...Option) js.Lib {
	var cfg libConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	l := &lib{
		client:      cfg.client,
		timeout:     cfg.timeout,
		maxBodySize: cfg.maxBodySize,
		publicOnly:  cfg.publicOnly,
	}

	if len(cfg.allowedHosts) > 0 {
		hosts := make([]string, 0, len(cfg.allowedHosts))
		for _, host := range cfg.allowedHosts {
			hosts = append(hosts, strings.ToLower(host))
		}

		l.allowedHosts = collections.NewHashSetFrom(hosts...)
	}

	if l.client == nil {
		l.client = l.newClient()
	}

	return l
}

// requestOptions mirrors the JS-side options object of the verb helpers.
type requestOptions struct {
	Headers  map[string]string `json:"headers"`
	Query    map[string]string `json:"query"`
	Redirect string            `json:"redirect"`
	Timeout  int64             `json:"timeout"`
}

// fetchInit mirrors the JS-side init object of http.fetch.
type fetchInit struct {
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	Query    map[string]string `json:"query"`
	Body     any               `json:"body"`
	Redirect string            `json:"redirect"`
	Timeout  int64             `json:"timeout"`
}

func (*lib) Name() string {
	return Name
}

func (l *lib) Install(rt *js.Runtime) error {
	fetch := func(rawURL string, init *fetchInit) (map[string]any, error) {
		if init == nil {
			init = new(fetchInit)
		}

		method := strings.ToUpper(init.Method)
		if method == "" {
			method = http.MethodGet
		}

		opts := &requestOptions{Headers: init.Headers, Query: init.Query, Redirect: init.Redirect, Timeout: init.Timeout}

		return l.do(rt, method, rawURL, init.Body, opts)
	}

	return rt.Set(Name, map[string]any{
		"fetch": fetch,
		"get": func(rawURL string, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodGet, rawURL, nil, opts)
		},
		"post": func(rawURL string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPost, rawURL, body, opts)
		},
		"put": func(rawURL string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPut, rawURL, body, opts)
		},
		"patch": func(rawURL string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPatch, rawURL, body, opts)
		},
		"delete": func(rawURL string, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodDelete, rawURL, nil, opts)
		},
	})
}

// do validates, executes, and packages one HTTP request on behalf of the
// script, deriving the request context from the runtime's execution context.
func (l *lib) do(rt *js.Runtime, method, rawURL string, body any, opts *requestOptions) (map[string]any, error) {
	if opts == nil {
		opts = new(requestOptions)
	}

	mode, err := resolveRedirectMode(opts.Redirect)
	if err != nil {
		return nil, err
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if err := l.validateURL(target); err != nil {
		return nil, err
	}

	ctx, cancel := l.requestContext(rt, opts.Timeout)
	defer cancel()

	if mode != redirectFollow {
		ctx = context.WithValue(ctx, redirectModeKey{}, mode)
	}

	reader, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, err
	}

	if len(opts.Query) > 0 {
		query := req.URL.Query()
		for key, value := range opts.Query {
			query.Set(key, value)
		}

		req.URL.RawQuery = query.Encode()
	}

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := l.readBody(resp.Body)
	if err != nil {
		return nil, err
	}

	return buildResponse(rt, resp, data), nil
}

// resolveRedirectMode normalizes the fetch redirect member; an empty value
// means follow.
func resolveRedirectMode(mode string) (string, error) {
	switch mode {
	case "", redirectFollow:
		return redirectFollow, nil
	case redirectError, redirectManual:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: unknown redirect mode %q", ErrInvalidRequest, mode)
	}
}

// requestContext derives the per-request context from the runtime's execution
// context. The configured timeout acts as default and upper bound for the
// script-supplied one; without either, the request is bounded only by the
// execution context.
func (l *lib) requestContext(rt *js.Runtime, timeoutMs int64) (context.Context, context.CancelFunc) {
	timeout := l.timeout

	if timeoutMs > 0 {
		if d := time.Duration(timeoutMs) * time.Millisecond; timeout == 0 || d < timeout {
			timeout = d
		}
	}

	if timeout > 0 {
		return context.WithTimeout(rt.Context(), timeout)
	}

	return context.WithCancel(rt.Context())
}

// readBody drains the response body, enforcing the size cap when one is
// configured.
func (l *lib) readBody(body io.Reader) ([]byte, error) {
	if l.maxBodySize <= 0 {
		return io.ReadAll(body)
	}

	data, err := io.ReadAll(io.LimitReader(body, l.maxBodySize+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > l.maxBodySize {
		return nil, fmt.Errorf("%w: limit %d bytes", ErrBodyTooLarge, l.maxBodySize)
	}

	return data, nil
}

// encodeBody converts the script-supplied body into a request reader.
// Strings and byte slices pass through verbatim; any other non-nil value is
// JSON-encoded with an implied application/json content type.
func encodeBody(body any) (io.Reader, string, error) {
	switch b := body.(type) {
	case nil:
		return nil, "", nil
	case string:
		return strings.NewReader(b), "", nil
	case []byte:
		return bytes.NewReader(b), "", nil
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return nil, "", err
		}

		return bytes.NewReader(data), "application/json", nil
	}
}

// buildResponse packages the response for the script in the fetch Response
// shape. resp.Request reflects the final request of the redirect chain, so
// "url" is the final URL and a non-nil Request.Response marks a followed
// redirect.
func buildResponse(rt *js.Runtime, resp *http.Response, body []byte) map[string]any {
	headers := make(map[string]string, len(resp.Header))
	for name, values := range resp.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}

	return map[string]any{
		"status":     resp.StatusCode,
		"statusText": http.StatusText(resp.StatusCode),
		"ok":         resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		"url":        resp.Request.URL.String(),
		"redirected": resp.Request.Response != nil,
		"headers":    headers,
		"body":       string(body),
		"text": func() string {
			return string(body)
		},
		"json": func() (any, error) {
			var value any
			if err := json.Unmarshal(body, &value); err != nil {
				return nil, err
			}

			return value, nil
		},
		"arrayBuffer": func() any {
			return rt.VM().NewArrayBuffer(body)
		},
	}
}
