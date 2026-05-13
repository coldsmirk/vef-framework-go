package storage

import (
	"io"
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

// InitMultipartOptions contains parameters for opening a multipart upload
// session. The session is owned by the backend; callers thread the
// returned UploadID back through PutPart, CompleteMultipart, and
// AbortMultipart without interpreting it.
type InitMultipartOptions struct {
	// Key is the unique identifier the final assembled object will
	// receive.
	Key string
	// ContentType is the MIME type recorded with the final object.
	ContentType string
	// Metadata is custom key-value pairs stored on the final object.
	// Programmatic channel only — the HTTP API does not expose it.
	Metadata map[string]string
}

// PutPartOptions contains parameters for uploading a single part of an
// in-progress multipart upload. The reader MUST yield exactly Size
// bytes; backends use Size to validate the part against Multipart.PartSize()
// and to plan storage layout.
type PutPartOptions struct {
	// Key is the object key the multipart session is targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
	// PartNumber is the 1-indexed part position within the assembled
	// object. Re-uploading the same PartNumber overwrites the previous
	// content and yields a new ETag.
	PartNumber int
	// Reader is the part payload; the backend reads exactly Size bytes.
	Reader io.Reader
	// Size is the exact byte length of the part. Backends MAY reject
	// requests where Size is smaller than ServiceCapabilities.PartSize
	// (except for the final part of a session) with ErrPartTooSmall.
	Size int64
}

// PartInfo describes a successfully uploaded part as reported by the
// backend. ETag is the opaque token the caller MUST persist and replay
// back through CompleteMultipart.
type PartInfo struct {
	// PartNumber is the 1-indexed part position.
	PartNumber int
	// ETag is the opaque per-part identifier the backend assigned. The
	// format is backend-specific (MD5 hex on filesystem / memory, S3
	// entity-tag on MinIO); callers MUST treat it as an opaque string.
	ETag string
	// Size is the byte length the backend recorded for this part.
	Size int64
}

// CompletedPart references one finished part in a multipart upload
// (PartNumber + ETag), forwarded back to CompleteMultipart so the
// backend can verify and assemble parts in order.
type CompletedPart struct {
	// PartNumber is the 1-indexed part position.
	PartNumber int
	// ETag is the opaque token the backend returned from PutPart for
	// this part.
	ETag string
}

// CompleteMultipartOptions contains parameters for finalizing a
// multipart upload. Parts MUST be sorted ascending by PartNumber and
// MUST form a contiguous 1..N sequence; missing or duplicate part
// numbers cause CompleteMultipart to return ErrPartNumberOutOfRange.
type CompleteMultipartOptions struct {
	// Key is the object key the multipart session is targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
	// Parts is the ordered list of (PartNumber, ETag) pairs the backend
	// uses to verify and assemble the final object.
	Parts []CompletedPart
}

// AbortMultipartOptions contains parameters for canceling a multipart
// upload session. Backends discard any uploaded parts and release the
// session; calling Abort on an unknown or already-closed session is a
// no-op (idempotent).
type AbortMultipartOptions struct {
	// Key is the object key the multipart session was targeting.
	Key string
	// UploadID is the opaque session token returned by InitMultipart.
	UploadID string
}
