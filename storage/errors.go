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
	// ErrClaimNotFound indicates the requested upload claim does not exist
	// (already consumed by a business transaction, expired and swept,
	// or never existed).
	ErrClaimNotFound = errors.New("upload claim not found")
	// ErrUploadSessionNotFound indicates the multipart UploadID does not
	// reference a live session. Returned when calling PutPart /
	// CompleteMultipart against an unknown, completed, or aborted
	// session. AbortMultipart is exempt and returns nil for the same
	// condition (idempotent abort).
	ErrUploadSessionNotFound = errors.New("upload session not found")
	// ErrPartETagMismatch indicates one of the (PartNumber, ETag) pairs
	// supplied to CompleteMultipart does not match the ETag the backend
	// recorded for that PartNumber. Typically caused by a PartNumber
	// being silently re-uploaded after the caller persisted the old
	// ETag, or by ETag corruption on the caller side.
	ErrPartETagMismatch = errors.New("part etag mismatch")
	// ErrPartTooSmall indicates a non-final PutPart was smaller than the
	// backend's PartSize. The final part of a session is exempt from
	// the minimum; backends should only return this error for parts
	// that turn out to have a successor.
	ErrPartTooSmall = errors.New("non-final part smaller than backend minimum")
	// ErrPartNumberOutOfRange indicates a CompleteMultipart request
	// supplied a PartNumber outside the contiguous 1..N range, or the
	// supplied Parts list has gaps or duplicates.
	ErrPartNumberOutOfRange = errors.New("part number out of range")
)
