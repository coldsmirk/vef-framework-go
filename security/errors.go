package security

import "errors"

var (
	ErrDecodeJWTSecretFailed   = errors.New("failed to decode jwt secret")
	ErrGenerateJWTSecretFailed = errors.New("failed to generate jwt secret")

	ErrDecodeSignatureSecretFailed = errors.New("failed to decode signature secret")
	ErrSignatureSecretRequired     = errors.New("signature secret is required")

	ErrUserDetailsNotStruct        = errors.New("user details type must be a struct or struct pointer")
	ErrExternalAppDetailsNotStruct = errors.New("external app details type must be a struct or struct pointer")

	ErrQueryNotQueryBuilder = errors.New("query does not implement QueryBuilder interface")
	ErrQueryModelNotSet     = errors.New("query must call Model() before applying data permission")
)
