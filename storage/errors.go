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
)
