package result

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
)

// Predefined authorization and request errors.
var (
	ErrAccessDenied = Err(
		i18n.T(ErrMessageAccessDenied),
		WithCode(ErrCodeAccessDenied),
		WithStatus(fiber.StatusForbidden),
	)
	ErrTooManyRequests = Err(
		i18n.T(ErrMessageTooManyRequests),
		WithCode(ErrCodeTooManyRequests),
		WithStatus(fiber.StatusTooManyRequests),
	)
	ErrRequestTimeout = Err(
		i18n.T(ErrMessageRequestTimeout),
		WithCode(ErrCodeRequestTimeout),
		WithStatus(fiber.StatusRequestTimeout),
	)
	ErrUnknown = Err(
		i18n.T(ErrMessageUnknown),
		WithCode(ErrCodeUnknown),
		WithStatus(fiber.StatusInternalServerError),
	)
)

// Predefined ORM/persistence errors (HTTP 200 with error code) that
// any module may surface.
var (
	ErrRecordNotFound = Err(
		i18n.T(ErrMessageRecordNotFound),
		WithCode(ErrCodeRecordNotFound),
	)
	ErrRecordAlreadyExists = Err(
		i18n.T(ErrMessageRecordAlreadyExists),
		WithCode(ErrCodeRecordAlreadyExists),
	)
	ErrForeignKeyViolation = Err(
		i18n.T(ErrMessageForeignKeyViolation),
		WithCode(ErrCodeForeignKeyViolation),
	)
	ErrDangerousSQL = Err(
		i18n.T(ErrMessageDangerousSQL),
		WithCode(ErrCodeDangerousSQL),
	)
)

// ErrNotImplemented creates a not implemented error with custom message (HTTP 501).
func ErrNotImplemented(message string) Error {
	return Err(
		message,
		WithCode(ErrCodeNotImplemented),
		WithStatus(fiber.StatusNotImplemented),
	)
}
