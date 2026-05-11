package config

import "time"

// StorageProvider represents supported storage backend types.
type StorageProvider string

// Supported storage providers.
const (
	StorageMinIO      StorageProvider = "minio"
	StorageMemory     StorageProvider = "memory"
	StorageFilesystem StorageProvider = "filesystem"
)

// StorageConfig defines storage provider settings.
//
// Upload-flow tunables (MultipartThreshold, PartSize, ClaimTTL, PresignedTTL,
// PresignedReadTTL) have working defaults applied at read time when their
// zero value is loaded; callers should not assume the parsed config carries
// non-zero values.
type StorageConfig struct {
	Provider    StorageProvider  `config:"provider"`
	AutoMigrate bool             `config:"auto_migrate"`
	MinIO       MinIOConfig      `config:"minio"`
	Filesystem  FilesystemConfig `config:"filesystem"`

	// MultipartThreshold is the file size above which uploads dispatch to the
	// multipart protocol. Default: 5 MiB.
	MultipartThreshold int64 `config:"multipart_threshold"`
	// PartSize is the desired multipart part size in bytes. The runtime
	// applies max(PartSize, Service.Capabilities().MinPartSize). Default: 8 MiB.
	PartSize int64 `config:"part_size"`
	// ClaimTTL is how long an upload claim stays valid before being swept.
	// Default: 24h.
	ClaimTTL time.Duration `config:"claim_ttl"`
	// PresignedTTL is the validity window of presigned PUT URLs (direct
	// upload and per-part). Default: 15m.
	PresignedTTL time.Duration `config:"presigned_ttl"`
	// PresignedReadTTL is the validity window of presigned GET URLs.
	// Default: 1h.
	PresignedReadTTL time.Duration `config:"presigned_read_ttl"`
}

// Default upload-flow tunables. Mirror the values documented in the upload
// API design; configuration overrides apply iff non-zero.
const (
	DefaultMultipartThreshold int64         = 5 * 1024 * 1024
	DefaultPartSize           int64         = 8 * 1024 * 1024
	DefaultClaimTTL           time.Duration = 24 * time.Hour
	DefaultPresignedTTL       time.Duration = 15 * time.Minute
	DefaultPresignedReadTTL   time.Duration = 1 * time.Hour
)

// EffectiveMultipartThreshold returns MultipartThreshold or its default.
func (c *StorageConfig) EffectiveMultipartThreshold() int64 {
	if c.MultipartThreshold > 0 {
		return c.MultipartThreshold
	}

	return DefaultMultipartThreshold
}

// EffectivePartSize returns PartSize or its default.
func (c *StorageConfig) EffectivePartSize() int64 {
	if c.PartSize > 0 {
		return c.PartSize
	}

	return DefaultPartSize
}

// EffectiveClaimTTL returns ClaimTTL or its default.
func (c *StorageConfig) EffectiveClaimTTL() time.Duration {
	if c.ClaimTTL > 0 {
		return c.ClaimTTL
	}

	return DefaultClaimTTL
}

// EffectivePresignedTTL returns PresignedTTL or its default.
func (c *StorageConfig) EffectivePresignedTTL() time.Duration {
	if c.PresignedTTL > 0 {
		return c.PresignedTTL
	}

	return DefaultPresignedTTL
}

// EffectivePresignedReadTTL returns PresignedReadTTL or its default.
func (c *StorageConfig) EffectivePresignedReadTTL() time.Duration {
	if c.PresignedReadTTL > 0 {
		return c.PresignedReadTTL
	}

	return DefaultPresignedReadTTL
}

// MinIOConfig defines MinIO storage settings.
type MinIOConfig struct {
	Endpoint  string `config:"endpoint"`
	AccessKey string `config:"access_key"`
	SecretKey string `config:"secret_key"`
	Bucket    string `config:"bucket"`
	Region    string `config:"region"`
	UseSSL    bool   `config:"use_ssl"`
}

// FilesystemConfig defines filesystem storage settings.
type FilesystemConfig struct {
	Root string `config:"root"`
}
