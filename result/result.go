package result

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
)

// Result represents an API response with code, message, and optional data.
type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Response sends the result as JSON with optional HTTP status (defaults to 200).
func (r Result) Response(ctx fiber.Ctx, status ...int) error {
	statusCode := fiber.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}

	return ctx.Status(statusCode).JSON(r)
}

// IsOk returns true if the result code indicates success.
func (r Result) IsOk() bool {
	return r.Code == OkCode
}

// Ok creates a success Result with optional data and options.
// Usage: Ok(), Ok(data), Ok(WithMessage(...)), Ok(data, WithMessage(...)).
func Ok(dataOrOptions ...any) Result {
	var (
		data             any
		options          []OkOption
		firstOptionIndex = -1
		dataCount        int
	)

	for i, v := range dataOrOptions {
		if opt, ok := v.(OkOption); ok {
			if firstOptionIndex == -1 {
				firstOptionIndex = i
			}

			options = append(options, opt)
		} else {
			if firstOptionIndex != -1 {
				panic("result.Ok: data must come before options")
			}

			dataCount++
			if dataCount > 1 {
				panic("result.Ok: only one data argument is allowed")
			}

			data = v
		}
	}

	r := Result{
		Code:    OkCode,
		Message: i18n.T(OkMessage),
		Data:    data,
	}

	for _, opt := range options {
		opt(&r)
	}

	return r
}
