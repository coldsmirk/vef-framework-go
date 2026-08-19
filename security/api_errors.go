package security

import (
	"math"
	"time"

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
	ErrMessageAccountLocked                   = "security_account_locked"
	ErrMessagePasswordTooShort                = "security_password_too_short"
	ErrMessagePasswordTooLong                 = "security_password_too_long"
	ErrMessagePasswordTooFewCharClasses       = "security_password_too_few_char_classes"
)

// Response codes for security-domain API errors.
// 1000-1029: authentication; 1030-1039: challenge; 1050: password policy;
// 1060-1063: trust login.
const (
	ErrCodeUnauthenticated               = 1000
	ErrCodeUnsupportedAuthenticationType = 1001
	ErrCodeTokenExpired                  = 1002
	ErrCodeTokenInvalid                  = 1003
	ErrCodeTokenNotValidYet              = 1004
	ErrCodeTokenInvalidIssuer            = 1005
	ErrCodeTokenInvalidAudience          = 1006
	ErrCodePrincipalInvalid              = 1007
	ErrCodeCredentialsInvalid            = 1008
	ErrCodeAppIDRequired                 = 1009
	ErrCodeTimestampRequired             = 1010
	ErrCodeSignatureRequired             = 1011
	ErrCodeTimestampInvalid              = 1012
	ErrCodeSignatureExpired              = 1013
	ErrCodeExternalAppNotFound           = 1014
	ErrCodeExternalAppDisabled           = 1015
	ErrCodeIPNotAllowed                  = 1016
	ErrCodeSignatureInvalid              = 1017
	ErrCodeNonceRequired                 = 1018
	ErrCodeNonceInvalid                  = 1019
	ErrCodeNonceAlreadyUsed              = 1020
	ErrCodeAuthHeaderMissing             = 1021
	ErrCodeAuthHeaderInvalid             = 1022
	ErrCodeAccountLocked                 = 1023
	ErrCodeTooManyConcurrentSessions     = 1024
	ErrCodeAPIKeyInvalid                 = 1025
	ErrCodeBasicCredentialsInvalid       = 1026

	// Challenge errors (1030-1039). 1030 and 1032 are absent: they were never wired to a sentinel.
	ErrCodeChallengeTokenInvalid  = 1031
	ErrCodeChallengeTypeInvalid   = 1033
	ErrCodeChallengeResolveFailed = 1034
	ErrCodeOTPCodeRequired        = 1035
	ErrCodeOTPCodeInvalid         = 1036
	ErrCodeNewPasswordRequired    = 1037
	ErrCodeDepartmentRequired     = 1038

	// Password policy errors (1050). Every policy violation shares one code; the
	// i18n message identifies which rule was broken.
	ErrCodePasswordPolicyViolation = 1050

	// Trust login errors (1060-1063).
	ErrCodeTrustAuthFailed         = 1060
	ErrCodeTrustRedirectNotAllowed = 1061
	ErrCodeTrustUserNotResolved    = 1062
	ErrCodeTrustCodeInvalid        = 1063
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

	// ErrReservedPrincipal rejects a framework-internal identity at every entry
	// point — authentication, challenge resolution, token issuance. See
	// Principal.IsReserved.
	ErrReservedPrincipal = result.Err(
		i18n.T("security_reserved_principal_forbidden"),
		result.WithCode(ErrCodePrincipalInvalid),
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
	ErrAPIKeyInvalid = result.Err(
		i18n.T("security_api_key_invalid"),
		result.WithCode(ErrCodeAPIKeyInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	ErrBasicCredentialsInvalid = result.Err(
		i18n.T("security_basic_credentials_invalid"),
		result.WithCode(ErrCodeBasicCredentialsInvalid),
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
	ErrTooManyConcurrentSessions = result.Err(
		i18n.T("security_too_many_concurrent_sessions"),
		result.WithCode(ErrCodeTooManyConcurrentSessions),
		result.WithStatus(fiber.StatusForbidden),
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
	ErrChallengeResolveFailed = result.Err(
		i18n.T(ErrMessageChallengeResolveFailed),
		result.WithCode(ErrCodeChallengeResolveFailed),
		result.WithStatus(fiber.StatusUnauthorized),
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

// Predefined password-policy errors (HTTP 400). All carry
// ErrCodePasswordPolicyViolation; the message states which rule was broken.
var (
	ErrPasswordMissingUppercase = result.Err(
		i18n.T("security_password_missing_uppercase"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordMissingLowercase = result.Err(
		i18n.T("security_password_missing_lowercase"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordMissingDigit = result.Err(
		i18n.T("security_password_missing_digit"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordMissingSymbol = result.Err(
		i18n.T("security_password_missing_symbol"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordContainsIdentity = result.Err(
		i18n.T("security_password_contains_identity"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordBlocked = result.Err(
		i18n.T("security_password_blocked"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrPasswordReused = result.Err(
		i18n.T("security_password_reused"),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
)

// Trust login errors. Every one of them means the handoff must be started
// again from the external system.
var (
	// ErrTrustAuthFailed is the single verdict the trust-login gateway returns
	// for every way a handoff can fail verification: a bad signature, an expired
	// or replayed one, an unknown external app, a disabled one, or a source
	// address outside its whitelist. Distinguishing them would let a caller
	// enumerate which app IDs exist, and the caller has the same fix in every
	// case — re-sign the handoff correctly (HTTP 401).
	ErrTrustAuthFailed = result.Err(
		i18n.T("security_trust_auth_failed"),
		result.WithCode(ErrCodeTrustAuthFailed),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	// ErrTrustRedirectNotAllowed rejects a trust-login redirect target that no
	// entry of the initiating app's allowlist covers. It is deliberately
	// distinct from ErrTrustAuthFailed: the handoff authenticated, and the fix
	// is a configuration change rather than a re-signed request (HTTP 400).
	ErrTrustRedirectNotAllowed = result.Err(
		i18n.T("security_trust_redirect_not_allowed"),
		result.WithCode(ErrCodeTrustRedirectNotAllowed),
		result.WithStatus(fiber.StatusBadRequest),
	)
	// ErrTrustUserNotResolved reports that no local user corresponds to the
	// external identifier the handoff carried (HTTP 401).
	ErrTrustUserNotResolved = result.Err(
		i18n.T("security_trust_user_not_resolved"),
		result.WithCode(ErrCodeTrustUserNotResolved),
		result.WithStatus(fiber.StatusUnauthorized),
	)
	// ErrTrustCodeInvalid rejects a trust-login code that is unknown, already
	// redeemed, expired, or presented by a browser other than the one the
	// gateway redirected. The cases are deliberately indistinguishable: every
	// one of them means "start the handoff again" (HTTP 401).
	ErrTrustCodeInvalid = result.Err(
		i18n.T("security_trust_code_invalid"),
		result.WithCode(ErrCodeTrustCodeInvalid),
		result.WithStatus(fiber.StatusUnauthorized),
	)
)

// ErrPasswordTooShort reports a password below the minimum length (HTTP 400).
func ErrPasswordTooShort(minLength int) result.Error {
	return result.Err(
		i18n.T(ErrMessagePasswordTooShort, map[string]any{"min": minLength}),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
}

// ErrPasswordTooLong reports a password above the maximum length (HTTP 400).
func ErrPasswordTooLong(maxLength int) result.Error {
	return result.Err(
		i18n.T(ErrMessagePasswordTooLong, map[string]any{"max": maxLength}),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
}

// ErrPasswordTooFewCharClasses reports too few distinct character classes (HTTP 400).
func ErrPasswordTooFewCharClasses(minClasses int) result.Error {
	return result.Err(
		i18n.T(ErrMessagePasswordTooFewCharClasses, map[string]any{"count": minClasses}),
		result.WithCode(ErrCodePasswordPolicyViolation),
		result.WithStatus(fiber.StatusBadRequest),
	)
}

// ErrAccountLocked reports that brute-force protection has blocked further login
// attempts for the identity (HTTP 429). retryAfter is surfaced to the caller,
// rounded up to whole minutes (never below one).
func ErrAccountLocked(retryAfter time.Duration) result.Error {
	minutes := max(int(math.Ceil(retryAfter.Minutes())), 1)

	return result.Err(
		i18n.T(ErrMessageAccountLocked, map[string]any{"minutes": minutes}),
		result.WithCode(ErrCodeAccountLocked),
		result.WithStatus(fiber.StatusTooManyRequests),
	)
}

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
