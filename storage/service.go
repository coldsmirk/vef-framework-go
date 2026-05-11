package storage

import (
	"context"
	"io"
	"time"
)

// Service defines the core interface for object storage operations.
//
// The interface is provider-neutral: signatures, return types, and error
// vocabulary are kept independent of any specific SDK or vendor (S3, OSS,
// COS, Azure Blob, etc.). All vendor-specific behaviour lives in
// internal/storage/<provider>/ implementations.
//
// Optional capabilities (multipart upload, presigned URLs) are reported via
// Capabilities() and may return ErrCapabilityNotSupported on backends that
// do not implement them. Callers should consult Capabilities first and
// dispatch accordingly.
type Service interface {
	// Capabilities reports which optional features this backend supports.
	// It is safe to call any time after construction; the returned value
	// must not change for the lifetime of the Service instance.
	Capabilities() ServiceCapabilities

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
	// GetPresignedURL generates a presigned URL for temporary access to an object.
	GetPresignedURL(ctx context.Context, opts PresignedURLOptions) (string, error)
	// CopyObject copies an object from source to destination.
	CopyObject(ctx context.Context, opts CopyObjectOptions) (*ObjectInfo, error)
	// StatObject retrieves metadata information about an object.
	StatObject(ctx context.Context, opts StatObjectOptions) (*ObjectInfo, error)

	// PresignPutObject returns a presigned URL the client can use to PUT a
	// single object directly to storage. Returns ErrCapabilityNotSupported
	// if Capabilities().PresignedPut is false.
	PresignPutObject(ctx context.Context, opts PresignPutOptions) (*PresignedURL, error)

	// InitMultipart opens a multipart upload session in the backend.
	// Returns ErrCapabilityNotSupported if Capabilities().Multipart is false.
	// The returned UploadID is opaque and must be passed back unchanged to
	// PresignPart, CompleteMultipart, and AbortMultipart.
	InitMultipart(ctx context.Context, opts InitMultipartOptions) (*MultipartSession, error)

	// PresignPart returns a presigned URL the client can use to PUT a single
	// part of an in-progress multipart upload. Returns ErrCapabilityNotSupported
	// if Capabilities().PresignedPart is false.
	PresignPart(ctx context.Context, opts PresignPartOptions) (*PresignedURL, error)

	// CompleteMultipart finalizes a multipart upload by assembling all parts
	// into a single object. Parts must be supplied in PartNumber order with
	// the ETags returned to the client when each part was PUT.
	CompleteMultipart(ctx context.Context, opts CompleteMultipartOptions) (*ObjectInfo, error)

	// AbortMultipart cancels an in-progress multipart session and releases
	// any uploaded parts. Safe to call when the session has already been
	// completed or aborted (treated as a no-op).
	AbortMultipart(ctx context.Context, opts AbortMultipartOptions) error
}

// ServiceCapabilities reports which optional features a Service implementation
// supports. Returned by Service.Capabilities; the value is constant for the
// Service's lifetime.
type ServiceCapabilities struct {
	// Multipart indicates the backend supports InitMultipart /
	// CompleteMultipart / AbortMultipart for chunked uploads.
	Multipart bool
	// PresignedPut indicates PresignPutObject can return a usable URL.
	PresignedPut bool
	// PresignedGet indicates GetPresignedURL can return a usable GET URL.
	PresignedGet bool
	// PresignedPart indicates PresignPart can return a usable per-part URL.
	// Implies Multipart.
	PresignedPart bool
	// MaxObjectSize is the maximum supported single-object size in bytes,
	// or 0 for unlimited.
	MaxObjectSize int64
	// MinPartSize is the minimum required size in bytes for any non-final
	// part of a multipart upload, or 0 if not enforced.
	MinPartSize int64
	// MaxPartCount is the maximum number of parts in a single multipart
	// upload, or 0 for unlimited.
	MaxPartCount int
}

// PresignedURL is a backend-issued URL the client can use to access an
// object directly, bypassing the application server. Headers lists any HTTP
// headers the client MUST include with the request; the client should
// forward them verbatim without interpreting their semantics.
type PresignedURL struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

// MultipartSession identifies an opaque multipart upload session in the
// backend. UploadID is provider-defined and must be passed back unchanged
// to subsequent multipart calls.
type MultipartSession struct {
	Key      string `json:"key"`
	UploadID string `json:"uploadId"`
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
