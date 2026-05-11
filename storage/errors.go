package storage

import "errors"

var (
	// ErrBucketNotFound indicates the specified bucket does not exist.
	ErrBucketNotFound = errors.New("bucket not found")
	// ErrObjectNotFound indicates the specified object does not exist.
	ErrObjectNotFound = errors.New("object not found")
	// ErrInvalidBucketName indicates the bucket name is invalid.
	ErrInvalidBucketName = errors.New("invalid bucket name")
	// ErrInvalidObjectKey indicates the object key is invalid.
	ErrInvalidObjectKey = errors.New("invalid object key")
	// ErrAccessDenied indicates permission is denied for the operation.
	ErrAccessDenied = errors.New("access denied")
	// ErrProviderNotConfigured indicates no storage provider is configured.
	ErrProviderNotConfigured = errors.New("storage provider not configured")
	// ErrCapabilityNotSupported indicates the backend does not support the
	// requested optional capability (e.g. multipart, presigned PUT).
	// Callers should consult Service.Capabilities() and pick another mode.
	ErrCapabilityNotSupported = errors.New("capability not supported by backend")
	// ErrClaimNotFound indicates the requested upload claim does not exist
	// (already consumed by a business transaction, expired and swept,
	// or never existed).
	ErrClaimNotFound = errors.New("upload claim not found")
)
