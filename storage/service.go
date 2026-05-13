package storage

import (
	"context"
	"io"
	"time"
)

// Service is the provider-neutral storage interface. Every backend MUST
// implement all methods. Vendor-specific behavior lives in
// internal/storage/<provider>/ and is independent of any specific SDK.
type Service interface {
	// PutObject uploads an object to storage.
	PutObject(ctx context.Context, opts PutObjectOptions) (*ObjectInfo, error)
	// GetObject retrieves an object from storage.
	GetObject(ctx context.Context, opts GetObjectOptions) (io.ReadCloser, error)
	// DeleteObject deletes a single object from storage.
	DeleteObject(ctx context.Context, opts DeleteObjectOptions) error
	// DeleteObjects deletes multiple objects from storage in a batch operation.
	DeleteObjects(ctx context.Context, opts DeleteObjectsOptions) error
	// ListObjects lists objects in a bucket with optional filtering.
	ListObjects(ctx context.Context, opts ListObjectsOptions) ([]ObjectInfo, error)
	// CopyObject copies an object from source to destination.
	CopyObject(ctx context.Context, opts CopyObjectOptions) (*ObjectInfo, error)
	// StatObject retrieves metadata information about an object.
	StatObject(ctx context.Context, opts StatObjectOptions) (*ObjectInfo, error)
}

// Multipart is the vendor-neutral chunked-upload primitive. Every
// backend in this framework implements Multipart; callers obtain the
// handle via a type assertion against storage.Service.
//
// The model is S3-inspired but does NOT leak S3 vocabulary into the
// contract:
//
//   - UploadID is opaque; only the issuing backend interprets it.
//   - ETag is an opaque per-part identifier issued by the backend; the
//     caller holds it verbatim and passes it back to Complete.
//   - PartNumber is 1-indexed and contiguous (1..N) at Complete time.
//
// Contract:
//
//  1. PutPart calls for distinct PartNumbers of the same session MAY
//     proceed concurrently. Concurrent calls for the SAME PartNumber
//     have last-writer-wins semantics — the part is overwritten and
//     the previous ETag is invalidated.
//  2. Except the final part, every part MUST be at least
//     Capabilities().PartSize bytes; smaller parts return
//     ErrPartTooSmall.
//  3. CompleteMultipart MUST verify every (PartNumber, ETag) pair in
//     opts.Parts matches a recorded part; mismatches return
//     ErrPartETagMismatch.
//  4. CompleteMultipart MUST verify Parts cover the contiguous range
//     1..N with no gaps or duplicates; otherwise returns
//     ErrPartNumberOutOfRange.
//  5. After CompleteMultipart (success) or AbortMultipart the session
//     is closed; further PutPart / CompleteMultipart / AbortMultipart
//     against the same UploadID return ErrUploadSessionNotFound, with
//     the exception that AbortMultipart is idempotent — re-aborting an
//     unknown session returns nil.
//  6. Session TTL is NOT part of the contract: long-running sessions
//     are valid. Cleanup of abandoned sessions is driven by upper-layer
//     sweepers (see internal/storage/worker) through AbortMultipart.
//     Implementations MAY garbage-collect internally for resource
//     hygiene, but that is a quality concern, not a contract concern.
type Multipart interface {
	// PartSize returns the backend's authoritative part size in bytes.
	// Callers MUST split the object into chunks of exactly this size
	// (except the final chunk, which may be smaller). The value is
	// constant for the lifetime of the backend instance.
	PartSize() int64

	// MaxPartCount returns the maximum number of parts in a single
	// multipart upload, or 0 for unlimited. The value is constant for
	// the lifetime of the backend instance.
	MaxPartCount() int

	// InitMultipart opens a new upload session and returns an opaque
	// UploadID.
	InitMultipart(ctx context.Context, opts InitMultipartOptions) (*MultipartSession, error)

	// PutPart uploads a single part to an open session and returns the
	// ETag the backend assigned. Re-uploading the same PartNumber
	// overwrites the previous content and yields a new ETag.
	PutPart(ctx context.Context, opts PutPartOptions) (*PartInfo, error)

	// CompleteMultipart finalizes a session by assembling its parts in
	// PartNumber order. See the contract notes on the interface
	// documentation for the verification rules and error mapping.
	CompleteMultipart(ctx context.Context, opts CompleteMultipartOptions) (*ObjectInfo, error)

	// AbortMultipart cancels an open session, discarding any uploaded
	// parts. Idempotent: calling Abort on an unknown / already-closed
	// session returns nil.
	AbortMultipart(ctx context.Context, opts AbortMultipartOptions) error
}

// MultipartSession identifies an opaque multipart upload session in the
// backend. UploadID is provider-defined and must be passed back
// unchanged to subsequent multipart calls.
type MultipartSession struct {
	Key      string
	UploadID string
}

// ObjectInfo represents metadata information about a stored object.
type ObjectInfo struct {
	// Bucket is the name of the storage bucket
	Bucket string `json:"bucket"`
	// Key is the unique identifier of the object within the bucket
	Key string `json:"key"`
	// ETag is the entity tag, typically an MD5 hash used for versioning and cache validation
	ETag string `json:"eTag"`
	// Size is the object size in bytes
	Size int64 `json:"size"`
	// ContentType is the MIME type of the object
	ContentType string `json:"contentType"`
	// LastModified is the timestamp when the object was last modified
	LastModified time.Time `json:"lastModified"`
	// Metadata contains custom key-value pairs associated with the object
	Metadata map[string]string `json:"metadata,omitempty"`
}
