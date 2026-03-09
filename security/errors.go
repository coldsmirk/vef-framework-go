package security

import "errors"

var (
	ErrDecodeJWTSecretFailed = errors.New("failed to decode jwt secret")

	ErrDecodeSignatureSecretFailed = errors.New("failed to decode signature secret")
	ErrSignatureSecretRequired     = errors.New("signature secret is required")
	ErrSignatureAppIDRequired      = errors.New("signature appID is required")
	ErrSignatureNonceRequired      = errors.New("signature nonce is required")
	ErrSignatureRequired           = errors.New("signature is required")
	ErrSignatureInvalid            = errors.New("signature is invalid")
	ErrSignatureExpired            = errors.New("signature has expired")
	ErrSignatureNonceUsed          = errors.New("signature nonce has already been used")

	ErrUserDetailsNotStruct        = errors.New("user details type must be a struct or struct pointer")
	ErrExternalAppDetailsNotStruct = errors.New("external app details type must be a struct or struct pointer")

	ErrQueryNotQueryBuilder = errors.New("query does not implement QueryBuilder interface")
	ErrQueryModelNotSet     = errors.New("query must call Model() before applying data permission")
)
