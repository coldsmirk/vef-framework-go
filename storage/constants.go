package storage

const (
	// MetadataKeyOriginalFilename is the metadata key for storing the original filename
	// Note: MinIO canonicalizes metadata keys to Title-Case format (HTTP header standard).
	MetadataKeyOriginalFilename = "Original-Filename"
)
