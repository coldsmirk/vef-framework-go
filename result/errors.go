package result

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
)

// Predefined authentication errors (HTTP 401).
var (
	ErrUnauthenticated = Err(
		i18n.T(ErrMessageUnauthenticated),
		WithCode(ErrCodeUnauthenticated),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenExpired = Err(
		i18n.T(ErrMessageTokenExpired),
		WithCode(ErrCodeTokenExpired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalid = Err(
		i18n.T(ErrMessageTokenInvalid),
		WithCode(ErrCodeTokenInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenNotValidYet = Err(
		i18n.T(ErrMessageTokenNotValidYet),
		WithCode(ErrCodeTokenNotValidYet),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalidIssuer = Err(
		i18n.T(ErrMessageTokenInvalidIssuer),
		WithCode(ErrCodeTokenInvalidIssuer),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalidAudience = Err(
		i18n.T(ErrMessageTokenInvalidAudience),
		WithCode(ErrCodeTokenInvalidAudience),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenMissingSubject = Err(
		i18n.T(ErrMessageTokenMissingSubject),
		WithCode(ErrCodeTokenMissingSubject),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenMissingTokenType = Err(
		i18n.T(ErrMessageTokenMissingTokenType),
		WithCode(ErrCodeTokenMissingTokenType),
		WithStatus(fiber.StatusUnauthorized),
	)
)

// Predefined external app authentication errors (HTTP 401).
var (
	ErrAppIDRequired = Err(
		i18n.T(ErrMessageAppIDRequired),
		WithCode(ErrCodeAppIDRequired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTimestampRequired = Err(
		i18n.T(ErrMessageTimestampRequired),
		WithCode(ErrCodeTimestampRequired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureRequired = Err(
		i18n.T(ErrMessageSignatureRequired),
		WithCode(ErrCodeSignatureRequired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrTimestampInvalid = Err(
		i18n.T(ErrMessageTimestampInvalid),
		WithCode(ErrCodeTimestampInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureExpired = Err(
		i18n.T(ErrMessageSignatureExpired),
		WithCode(ErrCodeSignatureExpired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureInvalid = Err(
		i18n.T(ErrMessageSignatureInvalid),
		WithCode(ErrCodeSignatureInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrExternalAppNotFound = Err(
		i18n.T(ErrMessageExternalAppNotFound),
		WithCode(ErrCodeExternalAppNotFound),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrExternalAppDisabled = Err(
		i18n.T(ErrMessageExternalAppDisabled),
		WithCode(ErrCodeExternalAppDisabled),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrIPNotAllowed = Err(
		i18n.T(ErrMessageIPNotAllowed),
		WithCode(ErrCodeIPNotAllowed),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceRequired = Err(
		i18n.T(ErrMessageNonceRequired),
		WithCode(ErrCodeNonceRequired),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceInvalid = Err(
		i18n.T(ErrMessageNonceInvalid),
		WithCode(ErrCodeNonceInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceAlreadyUsed = Err(
		i18n.T(ErrMessageNonceAlreadyUsed),
		WithCode(ErrCodeNonceAlreadyUsed),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrAuthHeaderMissing = Err(
		i18n.T(ErrMessageAuthHeaderMissing),
		WithCode(ErrCodeAuthHeaderMissing),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrAuthHeaderInvalid = Err(
		i18n.T(ErrMessageAuthHeaderInvalid),
		WithCode(ErrCodeAuthHeaderInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
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

// Predefined challenge errors.
var (
	ErrChallengeTokenInvalid = Err(
		i18n.T(ErrMessageChallengeTokenInvalid),
		WithCode(ErrCodeChallengeTokenInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrChallengeTypeInvalid = Err(
		i18n.T(ErrMessageChallengeTypeInvalid),
		WithCode(ErrCodeChallengeTypeInvalid),
		WithStatus(fiber.StatusBadRequest),
	)
	ErrOTPCodeRequired = Err(
		i18n.T(ErrMessageOTPCodeRequired),
		WithCode(ErrCodeOTPCodeRequired),
		WithStatus(fiber.StatusBadRequest),
	)
	ErrOTPCodeInvalid = Err(
		i18n.T(ErrMessageOTPCodeInvalid),
		WithCode(ErrCodeOTPCodeInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
	ErrNewPasswordRequired = Err(
		i18n.T(ErrMessageNewPasswordRequired),
		WithCode(ErrCodeNewPasswordRequired),
		WithStatus(fiber.StatusBadRequest),
	)
	ErrDepartmentRequired = Err(
		i18n.T(ErrMessageDepartmentRequired),
		WithCode(ErrCodeDepartmentRequired),
		WithStatus(fiber.StatusBadRequest),
	)
)

// Predefined business errors (HTTP 200 with error code).
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

// ErrCredentialsInvalid creates a credentials invalid error with custom message (HTTP 401).
func ErrCredentialsInvalid(message string) Error {
	return Err(
		message,
		WithCode(ErrCodeCredentialsInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
}

// ErrPrincipalInvalid creates a principal invalid error with custom message (HTTP 401).
func ErrPrincipalInvalid(message string) Error {
	return Err(
		message,
		WithCode(ErrCodePrincipalInvalid),
		WithStatus(fiber.StatusUnauthorized),
	)
}
