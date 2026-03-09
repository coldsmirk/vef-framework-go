package result

import "fmt"

// ErrOption configures an Error.
type ErrOption func(*Error)

// WithCode sets the business error code.
func WithCode(code int) ErrOption {
	return func(e *Error) { e.Code = code }
}

// WithStatus sets the HTTP status code.
func WithStatus(status int) ErrOption {
	return func(e *Error) { e.Status = status }
}

// OkOption configures a Result.
type OkOption func(*Result)

// WithMessage sets a custom message for the result.
func WithMessage(message string) OkOption {
	return func(r *Result) { r.Message = message }
}

// WithMessagef sets a formatted message for the result.
func WithMessagef(format string, args ...any) OkOption {
	return func(r *Result) { r.Message = fmt.Sprintf(format, args...) }
}
