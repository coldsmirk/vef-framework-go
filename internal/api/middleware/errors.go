package middleware

import "errors"

var (
	// ErrOperationNotFound indicates the operation was not found in context.
	ErrOperationNotFound = errors.New("operation not found in request context")

	// ErrPrincipalNotFound indicates the principal was not found in context.
	ErrPrincipalNotFound = errors.New("principal not found in request context")

	// ErrRequestNotFound indicates the request was not found in context.
	ErrRequestNotFound = errors.New("request not found in request context")

	// ErrAuthStrategyNotFound indicates the auth strategy was not registered.
	ErrAuthStrategyNotFound = errors.New("authentication strategy not found")

	// ErrPermissionCheckerNotProvided indicates no permission checker was configured.
	ErrPermissionCheckerNotProvided = errors.New("permission checker not provided")

	// ErrDataPermissionResolverNotProvided indicates no data permission resolver was configured.
	ErrDataPermissionResolverNotProvided = errors.New("data permission resolver not provided")

	// ErrPermissionDenied indicates the principal does not have the required permission.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrPermissionCheckFailed indicates an error occurred during permission check.
	ErrPermissionCheckFailed = errors.New("permission check failed")

	// ErrDataScopeResolutionFailed indicates an error occurred during data scope resolution.
	ErrDataScopeResolutionFailed = errors.New("data scope resolution failed")

	// ErrAuditEventBuildFailed indicates an error occurred while building audit event.
	ErrAuditEventBuildFailed = errors.New("failed to build audit event")

	// ErrResponseDecodeFailed indicates an error occurred while decoding response body.
	ErrResponseDecodeFailed = errors.New("failed to decode response body")
)
