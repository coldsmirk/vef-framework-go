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

// MoveObjectOptions contains parameters for moving an object.
type MoveObjectOptions struct {
	CopyObjectOptions
}

// StatObjectOptions contains parameters for retrieving object metadata.
type StatObjectOptions struct {
	// Key is the unique identifier of the object
	Key string
}
