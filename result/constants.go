package result

// i18n message keys for API responses.
const (
	OkMessage                                 = "ok"
	ErrMessage                                = "error"
	ErrMessageRecordNotFound                  = "record_not_found"
	ErrMessageRecordAlreadyExists             = "record_already_exists"
	ErrMessageForeignKeyViolation             = "foreign_key_violation"
	ErrMessageUnknown                         = "unknown_error"
	ErrMessageNotFound                        = "not_found"
	ErrMessageTooManyRequests                 = "too_many_requests"
	ErrMessageUnauthenticated                 = "unauthenticated"
	ErrMessageTokenExpired                    = "token_expired"
	ErrMessageTokenInvalid                    = "token_invalid"
	ErrMessageTokenNotValidYet                = "token_not_valid_yet"
	ErrMessageTokenInvalidIssuer              = "token_invalid_issuer"
	ErrMessageTokenInvalidAudience            = "token_invalid_audience"
	ErrMessageTokenMissingSubject             = "token_missing_subject"
	ErrMessageTokenMissingTokenType           = "token_missing_token_type"
	ErrMessageAppIDRequired                   = "app_id_required"
	ErrMessageTimestampRequired               = "timestamp_required"
	ErrMessageSignatureRequired               = "signature_required"
	ErrMessageTimestampInvalid                = "timestamp_invalid"
	ErrMessageSignatureExpired                = "signature_expired"
	ErrMessageExternalAppNotFound             = "external_app_not_found"
	ErrMessageExternalAppDisabled             = "external_app_disabled"
	ErrMessageIPNotAllowed                    = "ip_not_allowed"
	ErrMessageSignatureInvalid                = "signature_invalid"
	ErrMessageAccessDenied                    = "access_denied"
	ErrMessageUnsupportedMediaType            = "unsupported_media_type"
	ErrMessageRequestTimeout                  = "request_timeout"
	ErrMessageMonitorNotReady                 = "monitor_not_ready"
	ErrMessageInvalidFileKey                  = "invalid_file_key"
	ErrMessageFileNotFound                    = "file_not_found"
	ErrMessageFailedToGetFile                 = "failed_to_get_file"
	ErrMessageAPIRequestParamsInvalidJSON     = "api_request_params_invalid_json"
	ErrMessageAPIRequestMetaInvalidJSON       = "api_request_meta_invalid_json"
	ErrMessageDangerousSQL                    = "dangerous_sql"
	ErrMessageExternalAppLoaderNotImplemented = "external_app_loader_not_implemented"
	ErrMessageCredentialsFormatInvalid        = "credentials_format_invalid"
	ErrMessageCredentialsFieldsRequired       = "credentials_fields_required"
	ErrMessageSignatureDecodeFailed           = "signature_decode_failed"
	ErrMessageNonceRequired                   = "nonce_required"
	ErrMessageNonceInvalid                    = "nonce_invalid"
	ErrMessageNonceAlreadyUsed                = "nonce_already_used"
	ErrMessageAuthHeaderMissing               = "auth_header_missing"
	ErrMessageAuthHeaderInvalid               = "auth_header_invalid"
	ErrMessageUnsupportedAuthenticationType   = "unsupported_authentication_type"
	ErrMessageUserLoaderNotImplemented        = "user_loader_not_implemented"
	ErrMessageUserInfoLoaderNotImplemented    = "user_info_loader_not_implemented"
	ErrMessageChallengeRequired               = "challenge_required"
	ErrMessageChallengeTokenInvalid           = "challenge_token_invalid"
	ErrMessageChallengeTokenExpired           = "challenge_token_expired"
	ErrMessageChallengeTypeInvalid            = "challenge_type_invalid"
	ErrMessageChallengeResolveFailed          = "challenge_resolve_failed"
	ErrMessageOTPCodeRequired                 = "otp_code_required"
	ErrMessageOTPCodeInvalid                  = "otp_code_invalid"
	ErrMessageNewPasswordRequired             = "new_password_required"
	ErrMessageDepartmentRequired              = "department_required"
)

// Response codes for API results.
// Code 0 indicates success; codes 1000+ indicate authentication/authorization errors;
// codes 2000+ indicate business logic errors.
const (
	OkCode = 0

	// Authentication errors (1000-1099).
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

	// Authorization errors (1100-1199).
	ErrCodeAccessDenied = 1100

	// Resource errors (1200-1299).
	ErrCodeNotFound = 1200

	// Media type errors (1300-1399).
	ErrCodeUnsupportedMediaType = 1300

	// Request errors (1400-1499).
	ErrCodeBadRequest      = 1400
	ErrCodeTooManyRequests = 1401
	ErrCodeRequestTimeout  = 1402

	// Not implemented (1500-1599).
	ErrCodeNotImplemented = 1500

	// SQL errors (1600-1699).
	ErrCodeDangerousSQL = 1600

	// Unknown errors (1900-1999).
	ErrCodeUnknown = 1900

	// Business errors (2000+).
	ErrCodeDefault             = 2000
	ErrCodeRecordNotFound      = 2001
	ErrCodeRecordAlreadyExists = 2002
	ErrCodeForeignKeyViolation = 2003
	ErrCodeMonitorNotReady     = 2100
	ErrCodeInvalidFileKey      = 2200
	ErrCodeFileNotFound        = 2201
	ErrCodeSchemaTableNotFound = 2300
)
