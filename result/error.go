package result

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
)

// Error represents a business-level error for API responses.
// It separates transport-level concerns (HTTP Status) from business logic (Code, Message).
// HTTP Status typically remains 200 to indicate successful communication,
// while Code indicates the actual business result.
type Error struct {
	Code    int
	Message string
	Status  int
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Message
}

// Err creates a new Error with optional message and options.
// Usage: Err(), Err("message"), Err("message", WithCode(...)), Err(WithCode(...)).
func Err(messageOrOptions ...any) Error {
	var (
		message string
		options []ErrOption
	)

	for i, v := range messageOrOptions {
		switch opt := v.(type) {
		case string:
			if i != 0 {
				panic("result.Err: message string must be the first argument")
			}

			message = opt

		case ErrOption:
			options = append(options, opt)
		default:
			panic(fmt.Sprintf("result.Err: invalid argument type %T at position %d", v, i))
		}
	}

	if message == "" {
		message = i18n.T(ErrMessage)
	}

	err := Error{
		Code:    ErrCodeDefault,
		Message: message,
		Status:  fiber.StatusOK,
	}

	for _, opt := range options {
		opt(&err)
	}

	return err
}

// Errf creates a new Error with a formatted message.
// Usage: Errf("user %s not found", username), Errf("error %d", code, WithCode(...)).
func Errf(format string, args ...any) Error {
	if len(args) == 0 {
		panic("result.Errf: at least one format argument is required")
	}

	var (
		formatArgs       []any
		options          []ErrOption
		firstOptionIndex = -1
	)

	for i, v := range args {
		if opt, ok := v.(ErrOption); ok {
			if firstOptionIndex == -1 {
				firstOptionIndex = i
			}

			options = append(options, opt)
		} else {
			if firstOptionIndex != -1 {
				panic("result.Errf: format arguments must come before options")
			}

			formatArgs = append(formatArgs, v)
		}
	}

	err := Error{
		Code:    ErrCodeDefault,
		Message: fmt.Sprintf(format, formatArgs...),
		Status:  fiber.StatusOK,
	}

	for _, opt := range options {
		opt(&err)
	}

	return err
}

// AsErr extracts an Error from err if present.
func AsErr(err error) (Error, bool) {
	var target Error
	if errors.As(err, &target) {
		return target, true
	}

	return Error{}, false
}

// IsRecordNotFound checks if the error is a record not found error.
func IsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}
