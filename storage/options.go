package storage

import (
	"io"
	"time"
)

// PutObjectOptions contains parameters for uploading an object.
type PutObjectOptions struct {
	// Key is the unique identifier for the object
	Key string
	// Reader provides the object data to upload
	Reader io.Reader
	// Size is the size of the data in bytes (-1 if unknown)
	Size int64
	// ContentType specifies the MIME type of the object
	ContentType string
	// Metadata contains custom key-value pairs to store with the object
	Metadata map[string]string
}

// GetObjectOptions contains parameters for retrieving an object.
type GetObjectOptions struct {
	// Key is the unique identifier of the object
	Key string
}

// DeleteObjectOptions contains parameters for deleting a single object.
type DeleteObjectOptions struct {
	// Key is the unique identifier of the object to delete
	Key string
}

// DeleteObjectsOptions contains parameters for batch deleting objects.
type DeleteObjectsOptions struct {
	// Keys is the list of object identifiers to delete
	Keys []string
}

// ListObjectsOptions contains parameters for listing objects.
type ListObjectsOptions struct {
	// Prefix filters objects by key prefix
	Prefix string
	// Recursive determines whether to list objects recursively
	Recursive bool
	// MaxKeys limits the maximum number of objects to return
	MaxKeys int
}

// PresignedURLOptions contains parameters for generating presigned URLs.
type PresignedURLOptions struct {
	// Key is the unique identifier of the object
	Key string
	// Expires specifies how long the presigned URL remains valid
	Expires time.Duration
	// Method specifies the HTTP method (GET for download, PUT for upload)
	Method string
}

// CopyObjectOptions contains parameters for copying an object.
type CopyObjectOptions struct {
	// SourceKey is the identifier of the source object
	SourceKey string
	// DestKey is the identifier for the copied object
	DestKey string
}

// StatObjectOptions contains parameters for retrieving object metadata.
type StatObjectOptions struct {
	// Key is the unique identifier of the object
	Key string
}

// PresignPutOptions contains parameters for generating a presigned URL that
// the client can use to PUT a single object directly to the storage backend.
// Used by the "direct" upload protocol for small files.
type PresignPutOptions struct {
	// Key is the unique identifier the uploaded object will receive.
	Key string
	// ContentType is the MIME type the client will send. Backends MAY
	// include it in the URL signature or in PresignedURL.Headers.
	ContentType string
	// Expires is the URL's validity window.
	Expires time.Duration
}

// InitMultipartOptions contains parameters for opening a multipart upload session.
type InitMultipartOptions struct {
	// Key is the unique identifier the final assembled object will receive.
	Key string
	// ContentType is the MIME type recorded with the final object.
	ContentType string
	// Metadata is custom key-value pairs stored on the final object.
	Metadata map[string]string
}

// PresignPartOptions contains parameters for generating a presigned URL that
// the client can use to PUT a single part of an in-progress multipart upload.
type PresignPartOptions struct {
	// Key is the object key the multipart session is targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
	// PartNumber is the 1-indexed part position within the assembled object.
	PartNumber int
	// Expires is the URL's validity window.
	Expires time.Duration
}

// CompletedPart describes one finished part in a multipart upload, supplied
// by the client to CompleteMultipart so the backend can assemble parts in
// order. ETag is the opaque per-part identifier the backend returned to the
// client when the part PUT succeeded; the client passes it back unchanged.
type CompletedPart struct {
	// PartNumber is the 1-indexed part position.
	PartNumber int
	// ETag is the opaque token (RFC 7232 entity-tag) the backend returned
	// in the PUT response when the part was uploaded.
	ETag string
}

// CompleteMultipartOptions contains parameters for finalizing a multipart upload.
type CompleteMultipartOptions struct {
	// Key is the object key the multipart session is targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
	// Parts is the ordered list of completed parts. Backends will validate
	// part numbers form a contiguous 1-indexed sequence.
	Parts []CompletedPart
}

// AbortMultipartOptions contains parameters for cancelling a multipart upload session.
// Calling Abort releases any partially uploaded parts on the backend.
type AbortMultipartOptions struct {
	// Key is the object key the multipart session was targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
}
