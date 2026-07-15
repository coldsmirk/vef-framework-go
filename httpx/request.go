package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// bodyKind enumerates how the request body was supplied.
type bodyKind int

const (
	bodyNone bodyKind = iota
	bodyBuffered
	bodyStream
	bodyForm
)

// filePart is one file attached to a multipart form body.
type filePart struct {
	field    string
	filename string
	path     string
	reader   io.Reader
}

// Request is a single-use request builder created by Client.NewRequest: the
// Set/Add methods chain, a terminal method (Get, Post, ..., Do) executes the
// request, and a second execution fails with ErrRequestReused. Builder
// errors such as a failed body marshal are recorded and surface at the
// terminal method, so a chain needs no mid-flight error checks.
type Request struct {
	client *Client

	header        http.Header
	query         url.Values
	pathParams    map[string]string
	cookies       []*http.Cookie
	authorization string
	timeout       time.Duration
	timeoutSet    bool

	kind     bodyKind
	body     []byte
	bodyType string
	stream   io.Reader
	form     url.Values
	files    []filePart

	err      error
	executed bool

	ctx    context.Context
	method string
	url    string
}

// SetHeader sets a header, replacing a client default or previously set
// value of the same name.
func (r *Request) SetHeader(key, value string) *Request {
	r.header.Set(key, value)

	return r
}

// AddHeader appends a header value, keeping previously added ones.
func (r *Request) AddHeader(key, value string) *Request {
	r.header.Add(key, value)

	return r
}

// SetHeaders sets each header in h.
func (r *Request) SetHeaders(h map[string]string) *Request {
	for key, value := range h {
		r.header.Set(key, value)
	}

	return r
}

// SetQuery sets a query parameter, replacing a client default, URL-carried,
// or previously set value of the same name.
func (r *Request) SetQuery(key, value string) *Request {
	r.query.Set(key, value)

	return r
}

// AddQuery appends a query parameter value, keeping previously added ones.
func (r *Request) AddQuery(key, value string) *Request {
	r.query.Add(key, value)

	return r
}

// SetQueries sets each query parameter in m.
func (r *Request) SetQueries(m map[string]string) *Request {
	for key, value := range m {
		r.query.Set(key, value)
	}

	return r
}

// SetPathParam supplies the value for a ":name" segment of the request URL —
// the same template syntax as the server-side routes. The value is
// URL-escaped; a ":name" segment with no value fails the call with
// ErrMissingPathParam.
func (r *Request) SetPathParam(key, value string) *Request {
	if r.pathParams == nil {
		r.pathParams = make(map[string]string)
	}

	r.pathParams[key] = value

	return r
}

// SetPathParams supplies the value for each ":name" segment in m.
func (r *Request) SetPathParams(m map[string]string) *Request {
	for key, value := range m {
		r.SetPathParam(key, value)
	}

	return r
}

// SetCookie sends a cookie with the request, replacing a same-named one.
func (r *Request) SetCookie(name, value string) *Request {
	for _, cookie := range r.cookies {
		if cookie.Name == name {
			cookie.Value = value

			return r
		}
	}

	r.cookies = append(r.cookies, &http.Cookie{Name: name, Value: value})

	return r
}

// SetBasicAuth authenticates this request with HTTP Basic credentials,
// overriding client-level authentication; the last authentication setter
// wins.
func (r *Request) SetBasicAuth(username, password string) *Request {
	r.authorization = basicAuthorization(username, password)

	return r
}

// SetBearerToken authenticates this request with a bearer token, overriding
// client-level authentication; the last authentication setter wins.
func (r *Request) SetBearerToken(token string) *Request {
	r.authorization = bearerAuthorization(token)

	return r
}

// SetJSON marshals v into a JSON request body and sets the Content-Type.
// Body setters overwrite one another: the last one wins.
func (r *Request) SetJSON(v any) *Request {
	data, err := json.Marshal(v)
	if err != nil {
		return r.recordError(fmt.Errorf("httpx: marshal JSON body: %w", err))
	}

	return r.SetBody(data, "application/json")
}

// SetXML marshals v into an XML request body and sets the Content-Type.
func (r *Request) SetXML(v any) *Request {
	data, err := xml.Marshal(v)
	if err != nil {
		return r.recordError(fmt.Errorf("httpx: marshal XML body: %w", err))
	}

	return r.SetBody(data, "application/xml")
}

// SetBody sets a raw request body with an explicit content type; a
// Content-Type header set on the request or client still takes precedence.
func (r *Request) SetBody(body []byte, contentType string) *Request {
	r.resetBody(bodyBuffered)
	r.body = body
	r.bodyType = contentType

	return r
}

// SetBodyReader streams the request body from reader with an explicit
// content type. A streamed body cannot be replayed, so the request is never
// retried.
func (r *Request) SetBodyReader(reader io.Reader, contentType string) *Request {
	r.resetBody(bodyStream)
	r.stream = reader
	r.bodyType = contentType

	return r
}

// SetForm sets each form field in m. Fields alone are sent as
// application/x-www-form-urlencoded; attaching any file upgrades the body to
// multipart/form-data with the fields as parts.
func (r *Request) SetForm(m map[string]string) *Request {
	r.ensureForm()

	for key, value := range m {
		r.form.Set(key, value)
	}

	return r
}

// AddFormField appends a form field value, allowing repeated names.
func (r *Request) AddFormField(key, value string) *Request {
	r.ensureForm()
	r.form.Add(key, value)

	return r
}

// AddFile attaches the file at path as a multipart part named field, using
// the path's base name as the file name; the file is read when the request
// executes.
func (r *Request) AddFile(field, path string) *Request {
	r.ensureForm()
	r.files = append(r.files, filePart{field: field, filename: filepath.Base(path), path: path})

	return r
}

// AddFileReader attaches a multipart file part read from reader; the reader
// is drained into the buffered body when the request executes.
func (r *Request) AddFileReader(field, filename string, reader io.Reader) *Request {
	r.ensureForm()
	r.files = append(r.files, filePart{field: field, filename: filename, reader: reader})

	return r
}

// SetTimeout overrides the client's call timeout for this request; zero
// removes the bound entirely.
func (r *Request) SetTimeout(d time.Duration) *Request {
	r.timeout = d
	r.timeoutSet = true

	return r
}

// Method returns the HTTP method; empty until a terminal method runs.
func (r *Request) Method() string {
	return r.method
}

// URL returns the fully resolved request URL — base joined, path parameters
// substituted, query merged; empty until a terminal method runs.
func (r *Request) URL() string {
	return r.url
}

// Header returns the first value of the named request header.
func (r *Request) Header(key string) string {
	return r.header.Get(key)
}

// Headers returns the request headers. During a RequestHook they are the
// final outgoing headers and may still be mutated.
func (r *Request) Headers() http.Header {
	return r.header
}

// Body returns the buffered request body; nil when there is none or when
// the body streams from a reader.
func (r *Request) Body() []byte {
	return r.body
}

// Context returns the execution context while the request runs;
// context.Background() otherwise.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}

	return context.Background()
}

// Get executes the request as an HTTP GET against url, resolved against the
// client's base URL when relative.
func (r *Request) Get(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodGet, url)
}

// Post executes the request as an HTTP POST against url.
func (r *Request) Post(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodPost, url)
}

// Put executes the request as an HTTP PUT against url.
func (r *Request) Put(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodPut, url)
}

// Patch executes the request as an HTTP PATCH against url.
func (r *Request) Patch(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodPatch, url)
}

// Delete executes the request as an HTTP DELETE against url.
func (r *Request) Delete(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodDelete, url)
}

// Head executes the request as an HTTP HEAD against url.
func (r *Request) Head(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodHead, url)
}

// Options executes the request as an HTTP OPTIONS against url.
func (r *Request) Options(ctx context.Context, url string) (*Response, error) {
	return r.Do(ctx, http.MethodOptions, url)
}

// Do executes the request with an arbitrary method; every terminal verb
// funnels through it.
func (r *Request) Do(ctx context.Context, method, url string) (*Response, error) {
	if r.executed {
		return nil, ErrRequestReused
	}

	r.executed = true

	if r.err != nil {
		return nil, r.err
	}

	r.method = method
	if err := r.finalize(url); err != nil {
		return nil, err
	}

	timeout := r.client.timeout
	if r.timeoutSet {
		timeout = r.timeout
	}

	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	r.ctx = ctx
	defer func() { r.ctx = nil }()

	for _, hook := range r.client.requestHooks {
		if err := hook(r); err != nil {
			return nil, fmt.Errorf("httpx: request hook: %w", err)
		}
	}

	resp, err := r.send(ctx)
	if err != nil {
		return nil, err
	}

	for _, hook := range r.client.responseHooks {
		if err := hook(resp); err != nil {
			return nil, fmt.Errorf("httpx: response hook: %w", err)
		}
	}

	return resp, nil
}

// recordError keeps the first builder error for the terminal method.
func (r *Request) recordError(err error) *Request {
	if r.err == nil {
		r.err = err
	}

	return r
}

// resetBody switches the body representation, dropping any previous one.
func (r *Request) resetBody(kind bodyKind) {
	r.kind = kind
	r.body = nil
	r.bodyType = ""
	r.stream = nil
	r.form = nil
	r.files = nil
}

// ensureForm switches the body to the form representation unless it already
// is one.
func (r *Request) ensureForm() {
	if r.kind == bodyForm {
		return
	}

	r.resetBody(bodyForm)
	r.form = make(url.Values)
}

// finalize resolves the URL and assembles the outgoing body and headers, so
// request hooks observe the exact request that goes on the wire.
func (r *Request) finalize(requestURL string) error {
	if err := r.resolveURL(requestURL); err != nil {
		return err
	}

	if err := r.buildBody(); err != nil {
		return err
	}

	r.mergeHeaders()

	return nil
}

// resolveURL joins requestURL onto the client base URL when relative,
// substitutes ":name" path parameters, and merges the query values: values
// set on the request replace same-named client defaults and URL-carried
// parameters.
func (r *Request) resolveURL(requestURL string) error {
	target, err := r.joinBaseURL(requestURL)
	if err != nil {
		return err
	}

	if err := substitutePathParams(target, r.pathParams); err != nil {
		return err
	}

	query := target.Query()

	for key, values := range r.client.query {
		if _, ok := query[key]; !ok {
			query[key] = slices.Clone(values)
		}
	}

	maps.Copy(query, r.query)
	target.RawQuery = query.Encode()

	r.url = target.String()

	return nil
}

// joinBaseURL parses requestURL, joining it onto the client base URL unless
// it is already absolute.
func (r *Request) joinBaseURL(requestURL string) (*url.URL, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidRequestURL, requestURL, err)
	}

	if parsed.IsAbs() {
		return parsed, nil
	}

	base := r.client.baseURL
	if base == nil {
		return nil, fmt.Errorf("%w: %q is relative and the client has no base URL", ErrInvalidRequestURL, requestURL)
	}

	joined := strings.TrimSuffix(base.String(), "/")
	if requestURL != "" {
		joined += "/" + strings.TrimPrefix(requestURL, "/")
	}

	target, err := url.Parse(joined)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidRequestURL, requestURL, err)
	}

	return target, nil
}

// substitutePathParams replaces every ":name" path segment with its value
// from params, URL-escaped; a segment with no value is an error, a param
// matching no segment is ignored.
func substitutePathParams(target *url.URL, params map[string]string) error {
	if !strings.Contains(target.Path, ":") {
		return nil
	}

	segments := strings.Split(target.EscapedPath(), "/")

	for i, segment := range segments {
		name, ok := strings.CutPrefix(segment, ":")
		if !ok || name == "" {
			continue
		}

		value, ok := params[name]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingPathParam, name)
		}

		segments[i] = url.PathEscape(value)
	}

	escaped := strings.Join(segments, "/")

	unescaped, err := url.PathUnescape(escaped)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRequestURL, err)
	}

	target.Path = unescaped
	target.RawPath = escaped

	return nil
}

// buildBody materializes a form body: fields alone encode as
// x-www-form-urlencoded, fields with files as multipart/form-data. Buffered
// and streamed bodies are already final.
func (r *Request) buildBody() error {
	if r.kind != bodyForm {
		return nil
	}

	if len(r.files) == 0 {
		encoded := []byte(r.form.Encode())
		r.resetBody(bodyBuffered)
		r.body = encoded
		r.bodyType = "application/x-www-form-urlencoded"

		return nil
	}

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	for _, key := range slices.Sorted(maps.Keys(r.form)) {
		for _, value := range r.form[key] {
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("httpx: write form field %s: %w", key, err)
			}
		}
	}

	for _, file := range r.files {
		if err := writeFilePart(writer, file); err != nil {
			return err
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("httpx: finish multipart body: %w", err)
	}

	contentType := writer.FormDataContentType()

	r.resetBody(bodyBuffered)
	r.body = buf.Bytes()
	r.bodyType = contentType

	return nil
}

// writeFilePart streams one attached file into the multipart writer.
func writeFilePart(writer *multipart.Writer, file filePart) error {
	part, err := writer.CreateFormFile(file.field, file.filename)
	if err != nil {
		return fmt.Errorf("httpx: create file part %s: %w", file.field, err)
	}

	source := file.reader

	if file.path != "" {
		opened, err := os.Open(file.path)
		if err != nil {
			return fmt.Errorf("httpx: open file %s: %w", file.path, err)
		}
		defer func() { _ = opened.Close() }()

		source = opened
	}

	if _, err := io.Copy(part, source); err != nil {
		return fmt.Errorf("httpx: read file part %s: %w", file.field, err)
	}

	return nil
}

// mergeHeaders folds the client defaults, authentication, cookies, and body
// content type into the request headers, which become the final outgoing
// set.
func (r *Request) mergeHeaders() {
	if r.header.Get("Content-Type") == "" && r.bodyType != "" {
		r.header.Set("Content-Type", r.bodyType)
	}

	final := r.client.header.Clone()
	if final == nil {
		final = make(http.Header)
	}

	if r.client.authorization != "" {
		final.Set("Authorization", r.client.authorization)
	}

	maps.Copy(final, r.header)

	if r.authorization != "" {
		final.Set("Authorization", r.authorization)
	}

	if final.Get("User-Agent") == "" {
		final.Set("User-Agent", defaultUserAgent)
	}

	r.header = final

	if len(r.cookies) > 0 {
		carrier := &http.Request{Header: r.header}
		for _, cookie := range r.cookies {
			carrier.AddCookie(cookie)
		}
	}
}

// send runs the attempt loop under the client retry policy and stamps the
// final response with its duration and attempt count.
func (r *Request) send(ctx context.Context) (*Response, error) {
	maxAttempts := 1
	if r.client.retry != nil && r.kind != bodyStream {
		maxAttempts = r.client.retry.MaxAttempts
	}

	start := time.Now()

	var (
		resp     *Response
		err      error
		attempts int
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attempts = attempt
		resp, err = r.attempt(ctx)

		if attempt == maxAttempts || ctx.Err() != nil {
			break
		}

		if !r.client.retry.shouldRetry(r.method, resp, err) {
			break
		}

		var hint time.Duration
		if resp != nil {
			hint = retryAfterHint(resp)
		}

		if sleepErr := sleepBackoff(ctx, r.client.retry.backoffDelay(attempt, hint)); sleepErr != nil {
			break
		}
	}

	if err != nil {
		if attempts > 1 {
			return nil, fmt.Errorf("httpx: giving up after %d attempts: %w", attempts, err)
		}

		return nil, err
	}

	resp.duration = time.Since(start)
	resp.attempts = attempts

	return resp, nil
}

// attempt performs one wire exchange and buffers the response.
func (r *Request) attempt(ctx context.Context) (*Response, error) {
	var body io.Reader

	switch r.kind {
	case bodyBuffered:
		if len(r.body) > 0 {
			body = bytes.NewReader(r.body)
		}
	case bodyStream:
		body = r.stream
	case bodyNone, bodyForm:
	}

	req, err := http.NewRequestWithContext(ctx, r.method, r.url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequestURL, err)
	}

	req.Header = r.header.Clone()

	raw, err := r.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = raw.Body.Close() }()

	data, err := readBody(raw.Body, r.client.maxResponseBody)
	if err != nil {
		return nil, err
	}

	return &Response{
		request:    r,
		statusCode: raw.StatusCode,
		status:     raw.Status,
		header:     raw.Header,
		cookies:    raw.Cookies(),
		body:       data,
	}, nil
}

// readBody buffers the response body, enforcing the configured size cap.
func readBody(body io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("httpx: read response body: %w", err)
		}

		return data, nil
	}

	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("httpx: read response body: %w", err)
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: cap %d bytes", ErrResponseTooLarge, limit)
	}

	return data, nil
}
