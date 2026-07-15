package exec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/js"
)

// httpLibName is the global binding of the scoped HTTP client.
const httpLibName = "http"

// httpLib exposes the system-scoped HTTP client to adapter scripts as the
// global "http" object, mirroring the jshttp fetch-style API:
//
//	http.fetch(path, { method, headers, query, body, timeout })
//	http.get(path, options?)     // sugar over fetch, likewise post/put/patch
//	http.post(path, body, options?)
//	http.delete(path, options?)
//
// Paths are always relative — the client is locked to the system's base URL
// and authentication is injected by the host, so scripts never see
// credentials or reach other hosts. Every call returns { status, statusText,
// ok, url, headers, body, text(), json(), arrayBuffer() }; failures are
// thrown as catchable exceptions. A script-supplied timeout (milliseconds)
// may only shorten the system call timeout. The fetch "redirect" member is
// not supported beyond the default follow mode.
type httpLib struct {
	client *httpx.Client
	// callTimeout is the system's per-request bound; script timeouts clamp
	// to it.
	callTimeout time.Duration
}

func newHTTPLib(client *httpx.Client, callTimeout time.Duration) js.Lib {
	return &httpLib{client: client, callTimeout: callTimeout}
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

func (*httpLib) Name() string {
	return httpLibName
}

func (l *httpLib) Install(rt *js.Runtime) error {
	fetch := func(path string, init *fetchInit) (map[string]any, error) {
		if init == nil {
			init = new(fetchInit)
		}

		method := strings.ToUpper(init.Method)
		if method == "" {
			method = http.MethodGet
		}

		opts := &requestOptions{Headers: init.Headers, Query: init.Query, Redirect: init.Redirect, Timeout: init.Timeout}

		return l.do(rt, method, path, init.Body, opts)
	}

	return rt.Set(httpLibName, map[string]any{
		"fetch": fetch,
		"get": func(path string, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodGet, path, nil, opts)
		},
		"post": func(path string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPost, path, body, opts)
		},
		"put": func(path string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPut, path, body, opts)
		},
		"patch": func(path string, body any, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodPatch, path, body, opts)
		},
		"delete": func(path string, opts *requestOptions) (map[string]any, error) {
			return l.do(rt, http.MethodDelete, path, nil, opts)
		},
	})
}

// do validates, executes, and packages one request on behalf of the script,
// recording the wire exchange into the invocation trace.
func (l *httpLib) do(rt *js.Runtime, method, path string, body any, opts *requestOptions) (map[string]any, error) {
	if opts == nil {
		opts = new(requestOptions)
	}

	if err := validatePath(path); err != nil {
		return nil, err
	}

	if opts.Redirect != "" && opts.Redirect != "follow" {
		return nil, fmt.Errorf("%w: %q", ErrRedirectModeUnsupported, opts.Redirect)
	}

	req := l.client.NewRequest()

	if len(opts.Headers) > 0 {
		req.SetHeaders(opts.Headers)
	}

	if len(opts.Query) > 0 {
		req.SetQueries(opts.Query)
	}

	if timeout := l.requestTimeout(opts.Timeout); timeout > 0 {
		req.SetTimeout(timeout)
	}

	if err := setBody(req, body); err != nil {
		return nil, err
	}

	ctx := rt.Context()

	resp, err := req.Do(ctx, method, path)
	if err != nil {
		if tc := traceFrom(ctx); tc != nil {
			tc.record(integration.HTTPExchange{Method: method, URL: path, Error: err.Error()})
		}

		if ctx.Err() != nil {
			return nil, err
		}

		return nil, &transportError{err: err}
	}

	if tc := traceFrom(ctx); tc != nil {
		tc.record(exchangeOf(resp))
	}

	return buildResponse(rt, resp), nil
}

// validatePath rejects absolute and host-carrying URLs: scripts may only
// address paths under the system base URL.
func validatePath(path string) error {
	parsed, err := url.Parse(path)
	if err != nil {
		return err
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return fmt.Errorf("%w: %q", ErrAbsoluteURLNotAllowed, path)
	}

	return nil
}

// requestTimeout clamps the script-supplied timeout to the system call
// timeout; zero keeps the client default.
func (l *httpLib) requestTimeout(timeoutMs int64) time.Duration {
	if timeoutMs <= 0 {
		return 0
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if l.callTimeout > 0 && timeout > l.callTimeout {
		return l.callTimeout
	}

	return timeout
}

// setBody encodes the script-supplied body onto the request. Strings and
// byte slices pass through verbatim; any other non-nil value is JSON-encoded
// with an implied application/json content type.
func setBody(req *httpx.Request, body any) error {
	switch b := body.(type) {
	case nil:
		return nil
	case string:
		req.SetBody([]byte(b), "")
	case []byte:
		req.SetBody(b, "")
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return err
		}

		req.SetBody(data, "application/json")
	}

	return nil
}

// exchangeOf captures the wire exchange of a completed call. Masking and
// truncation happen in the trace collector.
func exchangeOf(resp *httpx.Response) integration.HTTPExchange {
	req := resp.Request()

	return integration.HTTPExchange{
		Method:          req.Method(),
		URL:             req.URL(),
		RequestHeaders:  flattenHeader(req.Headers()),
		RequestBody:     string(req.Body()),
		Status:          resp.StatusCode(),
		ResponseHeaders: flattenHeader(resp.Headers()),
		ResponseBody:    resp.String(),
		DurationMs:      resp.Duration().Milliseconds(),
	}
}

// buildResponse packages the response for the script in the fetch Response
// shape. Header names are lower-cased and multi-values joined with ", ",
// matching fetch Headers semantics.
func buildResponse(rt *js.Runtime, resp *httpx.Response) map[string]any {
	headers := make(map[string]string, len(resp.Headers()))
	for name, values := range resp.Headers() {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}

	body := resp.Body()

	return map[string]any{
		"status":     resp.StatusCode(),
		"statusText": http.StatusText(resp.StatusCode()),
		"ok":         resp.IsSuccess(),
		"url":        resp.Request().URL(),
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
