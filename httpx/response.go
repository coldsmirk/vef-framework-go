package httpx

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"time"
)

// Response is the fully buffered outcome of a call: the body has been read
// and the connection released, so a Response is inert data that is safe to
// keep and share.
type Response struct {
	request    *Request
	statusCode int
	status     string
	header     http.Header
	cookies    []*http.Cookie
	body       []byte
	duration   time.Duration
	attempts   int
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int {
	return r.statusCode
}

// Status returns the full status line, e.g. "200 OK".
func (r *Response) Status() string {
	return r.status
}

// IsSuccess reports whether the status code is in the 2xx range. A non-2xx
// response is not an error — inspect it through this and StatusCode.
func (r *Response) IsSuccess() bool {
	return r.statusCode >= http.StatusOK && r.statusCode < http.StatusMultipleChoices
}

// Header returns the first value of the named response header.
func (r *Response) Header(key string) string {
	return r.header.Get(key)
}

// Headers returns all response headers.
func (r *Response) Headers() http.Header {
	return r.header
}

// Cookies returns the cookies the response sets.
func (r *Response) Cookies() []*http.Cookie {
	return r.cookies
}

// Body returns the buffered response body.
func (r *Response) Body() []byte {
	return r.body
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.body)
}

// JSON unmarshals the response body as JSON into v.
func (r *Response) JSON(v any) error {
	return json.Unmarshal(r.body, v)
}

// XML unmarshals the response body as XML into v.
func (r *Response) XML(v any) error {
	return xml.Unmarshal(r.body, v)
}

// Duration returns the wall time of the whole call, retries included.
func (r *Response) Duration() time.Duration {
	return r.duration
}

// Attempts returns how many attempts the call made; 1 unless retries ran.
func (r *Response) Attempts() int {
	return r.attempts
}

// Request returns the request that produced this response.
func (r *Response) Request() *Request {
	return r.request
}
