package app

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// fiberErrorMapping defines the mapping from Fiber HTTP status codes to application error codes and messages.
type fiberErrorMapping struct {
	code    int
	message string
}

// fiberErrorMappings maps Fiber HTTP status codes to error codes and message keys.
var fiberErrorMappings = map[int]fiberErrorMapping{
	fiber.StatusNotFound: {
		code:    result.ErrCodeNotFound,
		message: result.ErrMessageNotFound,
	},
	fiber.StatusUnauthorized: {
		code:    result.ErrCodeUnauthenticated,
		message: result.ErrMessageUnauthenticated,
	},
	fiber.StatusForbidden: {
		code:    result.ErrCodeAccessDenied,
		message: result.ErrMessageAccessDenied,
	},
	fiber.StatusUnsupportedMediaType: {
		code:    result.ErrCodeUnsupportedMediaType,
		message: result.ErrMessageUnsupportedMediaType,
	},
	fiber.StatusRequestTimeout: {
		code:    result.ErrCodeRequestTimeout,
		message: result.ErrMessageRequestTimeout,
	},
}

// handleError handles the error and returns the response.
func handleError(ctx fiber.Ctx, err error) error {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		// Look up the error mapping for this status code
		mapping, exists := fiberErrorMappings[fiberErr.Code]

		var r result.Result
		if exists {
			r = result.Result{
				Code:    mapping.code,
				Message: i18n.T(mapping.message),
			}
		} else {
			contextx.Logger(ctx).Errorf(
				"Unmapped Fiber error: status=%d, message=%s",
				fiberErr.Code, fiberErr.Message,
			)

			r = result.Result{
				Code:    result.ErrCodeUnknown,
				Message: i18n.T(result.ErrMessageUnknown),
			}
		}

		return r.Response(ctx, fiberErr.Code)
	}

	if resultErr, ok := result.AsErr(err); ok {
		return responseError(resultErr, ctx)
	}

	contextx.Logger(ctx).Errorf(
		"Unhandled error: type=%T, error=%v",
		err, err,
	)

	return responseError(result.ErrUnknown, ctx)
}

// responseError sends an error response to the client.
func responseError(e result.Error, ctx fiber.Ctx) error {
	return result.Result{
		Code:    e.Code,
		Message: e.Message,
	}.Response(ctx, e.Status)
}

// MapFiberError maps a Fiber HTTP status code to a business error code and i18n message key.
// Returns ErrCodeUnknown and ErrMessageUnknown if no mapping exists.
func MapFiberError(statusCode int) (code int, messageKey string, ok bool) {
	mapping, ok := fiberErrorMappings[statusCode]
	if !ok {
		return result.ErrCodeUnknown, result.ErrMessageUnknown, false
	}

	return mapping.code, mapping.message, true
}
