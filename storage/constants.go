package storage

const (
	// MetadataKeyOriginalFilename is the metadata key for storing the original filename
	// Note: MinIO canonicalizes metadata keys to Title-Case format (HTTP header standard).
	MetadataKeyOriginalFilename = "Original-Filename"

	// TempPrefix is the prefix for temporary object storage.
	// Files uploaded to temp/ should be promoted to permanent storage after business logic commits.
	TempPrefix = "temp/"
)
