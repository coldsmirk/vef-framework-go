package security

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// i18n message-IDs that callers reference directly (template arguments,
// factory-based result.Err construction, Fiber error mapping, etc.).
// Pure sentinel-internal message IDs live inline at the sentinel
// definition and never need a named constant.
const (
	ErrMessageUnauthenticated                 = "security_unauthenticated"
	ErrMessageExternalAppLoaderNotImplemented = "security_external_app_loader_not_implemented"
	ErrMessageCredentialsFormatInvalid        = "security_credentials_format_invalid"
	ErrMessageUnsupportedAuthenticationType   = "security_unsupported_authentication_type"
	ErrMessageUserLoaderNotImplemented        = "security_user_loader_not_implemented"
	ErrMessageUserInfoLoaderNotImplemented    = "security_user_info_loader_not_implemented"
	ErrMessageChallengeResolveFailed          = "security_challenge_resolve_failed"
)

// Response codes for security-domain API errors.
// 1000-1029: authentication; 1030-1039: challenge.
const (
	ErrCodeUnauthenticated               = 1000
	ErrCodeUnsupportedAuthenticationType = 1001
	ErrCodeTokenExpired                  = 1002
	ErrCodeTokenInvalid                  = 1003
	ErrCodeTokenNotValidYet              = 1004
	ErrCodeTokenInvalidIssuer            = 1005
	ErrCodeTokenInvalidAudience          = 1007
	ErrCodeTokenMissingSubject           = 1008
	ErrCodeTokenMissingTokenType         = 1009
	ErrCodePrincipalInvalid              = 1010
	ErrCodeCredentialsInvalid            = 1011
	ErrCodeAppIDRequired                 = 1012
	ErrCodeTimestampRequired             = 1013
	ErrCodeSignatureRequired             = 1014
	ErrCodeTimestampInvalid              = 1015
	ErrCodeSignatureExpired              = 1016
	ErrCodeExternalAppNotFound           = 1017
	ErrCodeExternalAppDisabled           = 1018
	ErrCodeIPNotAllowed                  = 1019
	ErrCodeSignatureInvalid              = 1020
	ErrCodeNonceRequired                 = 1021
	ErrCodeNonceInvalid                  = 1022
	ErrCodeNonceAlreadyUsed              = 1023
	ErrCodeAuthHeaderMissing             = 1024
	ErrCodeAuthHeaderInvalid             = 1025

	// Challenge errors (1030-1039).
	ErrCodeChallengeRequired      = 1030
	ErrCodeChallengeTokenInvalid  = 1031
	ErrCodeChallengeTokenExpired  = 1032
	ErrCodeChallengeTypeInvalid   = 1033
	ErrCodeChallengeResolveFailed = 1034
	ErrCodeOTPCodeRequired        = 1035
	ErrCodeOTPCodeInvalid         = 1036
	ErrCodeNewPasswordRequired    = 1037
	ErrCodeDepartmentRequired     = 1038
)

// Predefined authentication errors (HTTP 401).
var (
	ErrUnauthenticated = result.Err(
		i18n.T(ErrMessageUnauthenticated),
		result.WithCode(ErrCodeUnauthenticated),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenExpired = result.Err(
		i18n.T("security_token_expired"),
		result.WithCode(ErrCodeTokenExpired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalid = result.Err(
		i18n.T("security_token_invalid"),
		result.WithCode(ErrCodeTokenInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenNotValidYet = result.Err(
		i18n.T("security_token_not_valid_yet"),
		result.WithCode(ErrCodeTokenNotValidYet),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalidIssuer = result.Err(
		i18n.T("security_token_invalid_issuer"),
		result.WithCode(ErrCodeTokenInvalidIssuer),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenInvalidAudience = result.Err(
		i18n.T("security_token_invalid_audience"),
		result.WithCode(ErrCodeTokenInvalidAudience),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenMissingSubject = result.Err(
		i18n.T("security_token_missing_subject"),
		result.WithCode(ErrCodeTokenMissingSubject),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTokenMissingTokenType = result.Err(
		i18n.T("security_token_missing_token_type"),
		result.WithCode(ErrCodeTokenMissingTokenType),
		result.WithStatus(fiber.StatusUnauthorized),
	)
)

// Predefined external app authentication errors (HTTP 401).
var (
	ErrAppIDRequired = result.Err(
		i18n.T("security_app_id_required"),
		result.WithCode(ErrCodeAppIDRequired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTimestampRequired = result.Err(
		i18n.T("security_timestamp_required"),
		result.WithCode(ErrCodeTimestampRequired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureRequired = result.Err(
		i18n.T("security_signature_required"),
		result.WithCode(ErrCodeSignatureRequired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrTimestampInvalid = result.Err(
		i18n.T("security_timestamp_invalid"),
		result.WithCode(ErrCodeTimestampInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureExpired = result.Err(
		i18n.T("security_signature_expired"),
		result.WithCode(ErrCodeSignatureExpired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrSignatureInvalid = result.Err(
		i18n.T("security_signature_invalid"),
		result.WithCode(ErrCodeSignatureInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrExternalAppNotFound = result.Err(
		i18n.T("security_external_app_not_found"),
		result.WithCode(ErrCodeExternalAppNotFound),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrExternalAppDisabled = result.Err(
		i18n.T("security_external_app_disabled"),
		result.WithCode(ErrCodeExternalAppDisabled),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrIPNotAllowed = result.Err(
		i18n.T("security_ip_not_allowed"),
		result.WithCode(ErrCodeIPNotAllowed),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceRequired = result.Err(
		i18n.T("security_nonce_required"),
		result.WithCode(ErrCodeNonceRequired),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceInvalid = result.Err(
		i18n.T("security_nonce_invalid"),
		result.WithCode(ErrCodeNonceInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrNonceAlreadyUsed = result.Err(
		i18n.T("security_nonce_already_used"),
		result.WithCode(ErrCodeNonceAlreadyUsed),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrAuthHeaderMissing = result.Err(
		i18n.T("security_auth_header_missing"),
		result.WithCode(ErrCodeAuthHeaderMissing),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrAuthHeaderInvalid = result.Err(
		i18n.T("security_auth_header_invalid"),
		result.WithCode(ErrCodeAuthHeaderInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
)

// Predefined challenge errors.
var (
	ErrChallengeTokenInvalid = result.Err(
		i18n.T("security_challenge_token_invalid"),
		result.WithCode(ErrCodeChallengeTokenInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrChallengeTypeInvalid = result.Err(
		i18n.T("security_challenge_type_invalid"),
		result.WithCode(ErrCodeChallengeTypeInvalid),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrOTPCodeRequired = result.Err(
		i18n.T("security_otp_code_required"),
		result.WithCode(ErrCodeOTPCodeRequired),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrOTPCodeInvalid = result.Err(
		i18n.T("security_otp_code_invalid"),
		result.WithCode(ErrCodeOTPCodeInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrNewPasswordRequired = result.Err(
		i18n.T("security_new_password_required"),
		result.WithCode(ErrCodeNewPasswordRequired),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrDepartmentRequired = result.Err(
		i18n.T("security_department_required"),
		result.WithCode(ErrCodeDepartmentRequired),
		result.WithStatus(fiber.StatusBadRequest),
	)
)

// ErrCredentialsInvalid creates a credentials invalid error with custom message (HTTP 401).
func ErrCredentialsInvalid(message string) result.Error {
	return result.Err(
		message,
		result.WithCode(ErrCodeCredentialsInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
}

// ErrPrincipalInvalid creates a principal invalid error with custom message (HTTP 401).
func ErrPrincipalInvalid(message string) result.Error {
	return result.Err(
		message,
		result.WithCode(ErrCodePrincipalInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
}
