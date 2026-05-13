package config

import (
	"cmp"
	"time"
)

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
// Upload-flow and worker tunables have working defaults applied at read time
// when their zero value is loaded; callers should not assume the parsed
// config carries non-zero values. Use the Effective* accessors instead of
// reading struct fields directly.
type StorageConfig struct {
	Provider    StorageProvider  `config:"provider"`
	AutoMigrate bool             `config:"auto_migrate"`
	MinIO       MinIOConfig      `config:"minio"`
	Filesystem  FilesystemConfig `config:"filesystem"`

	// MaxUploadSize is the hard upper bound (in bytes) on a single object
	// uploaded through the storage RPC, regardless of declared mode.
	// Default: 1 GiB.
	MaxUploadSize int64 `config:"max_upload_size"`

	// ClaimTTL is how long an upload claim stays valid before being swept.
	// Default: 24h.
	ClaimTTL time.Duration `config:"claim_ttl"`

	// MaxPendingClaims caps the number of in-flight (status='pending')
	// upload claims a single principal may hold simultaneously. Prevents
	// a single user from exhausting backend resources by opening
	// thousands of multipart sessions. Default: 100.
	MaxPendingClaims int `config:"max_pending_claims"`

	// AllowPublicUploads controls whether clients may set public=true on
	// init_upload. When false (default), all uploads land under priv/
	// regardless of the client's request. Set to true only when the
	// business explicitly needs user-initiated public uploads.
	AllowPublicUploads bool `config:"allow_public_uploads"`

	// SweepInterval is how often the claim sweeper polls for expired
	// upload claims. Default: 5m.
	SweepInterval time.Duration `config:"sweep_interval"`
	// SweepBatchSize bounds how many expired claims one sweep tick processes.
	// Default: 200.
	SweepBatchSize int `config:"sweep_batch_size"`

	// DeleteWorkerInterval is how often the delete worker polls for due
	// pending-delete rows. Default: 5m.
	DeleteWorkerInterval time.Duration `config:"delete_worker_interval"`
	// DeleteBatchSize bounds how many rows the delete worker leases per tick.
	// Default: 100.
	DeleteBatchSize int `config:"delete_batch_size"`
	// DeleteConcurrency caps the number of in-flight object deletions
	// per worker tick. Default: 8.
	DeleteConcurrency int `config:"delete_concurrency"`
	// DeleteMaxAttempts is the retry budget after which a pending-delete
	// row is parked as dead-letter. Default: 12.
	DeleteMaxAttempts int `config:"delete_max_attempts"`
	// DeleteLeaseWindow is the visibility timeout applied when leasing
	// rows. Should comfortably exceed expected per-item processing time.
	// Default: 5m.
	DeleteLeaseWindow time.Duration `config:"delete_lease_window"`
}

// Default upload-flow and worker tunables. Mirror the values documented in
// the upload API design; configuration overrides apply iff strictly
// positive (a zero or negative value re-selects the default).
const (
	DefaultMaxUploadSize int64 = 1024 * 1024 * 1024 // 1 GiB

	DefaultClaimTTL         time.Duration = 24 * time.Hour
	DefaultMaxPendingClaims int           = 100

	DefaultSweepInterval  time.Duration = 5 * time.Minute
	DefaultSweepBatchSize int           = 200

	DefaultDeleteWorkerInterval time.Duration = 5 * time.Minute
	DefaultDeleteBatchSize      int           = 100
	DefaultDeleteConcurrency    int           = 8
	DefaultDeleteMaxAttempts    int           = 12
	DefaultDeleteLeaseWindow    time.Duration = 5 * time.Minute
)

// coalescePositive returns v when it is strictly greater than the zero
// value of T, otherwise def. Negative inputs are treated as misconfigured
// and fall back to the default — every Effective* accessor below models
// a size, count, or duration where negative values are nonsensical.
func coalescePositive[T cmp.Ordered](v, def T) T {
	var zero T
	if v <= zero {
		return def
	}

	return v
}

// EffectiveMaxUploadSize returns MaxUploadSize or its default.
func (c *StorageConfig) EffectiveMaxUploadSize() int64 {
	return coalescePositive(c.MaxUploadSize, DefaultMaxUploadSize)
}

// EffectiveClaimTTL returns ClaimTTL or its default.
func (c *StorageConfig) EffectiveClaimTTL() time.Duration {
	return coalescePositive(c.ClaimTTL, DefaultClaimTTL)
}

// EffectiveMaxPendingClaims returns MaxPendingClaims or its default.
func (c *StorageConfig) EffectiveMaxPendingClaims() int {
	return coalescePositive(c.MaxPendingClaims, DefaultMaxPendingClaims)
}

// EffectiveSweepInterval returns SweepInterval or its default.
func (c *StorageConfig) EffectiveSweepInterval() time.Duration {
	return coalescePositive(c.SweepInterval, DefaultSweepInterval)
}

// EffectiveSweepBatchSize returns SweepBatchSize or its default.
func (c *StorageConfig) EffectiveSweepBatchSize() int {
	return coalescePositive(c.SweepBatchSize, DefaultSweepBatchSize)
}

// EffectiveDeleteWorkerInterval returns DeleteWorkerInterval or its default.
func (c *StorageConfig) EffectiveDeleteWorkerInterval() time.Duration {
	return coalescePositive(c.DeleteWorkerInterval, DefaultDeleteWorkerInterval)
}

// EffectiveDeleteBatchSize returns DeleteBatchSize or its default.
func (c *StorageConfig) EffectiveDeleteBatchSize() int {
	return coalescePositive(c.DeleteBatchSize, DefaultDeleteBatchSize)
}

// EffectiveDeleteConcurrency returns DeleteConcurrency or its default.
func (c *StorageConfig) EffectiveDeleteConcurrency() int {
	return coalescePositive(c.DeleteConcurrency, DefaultDeleteConcurrency)
}

// EffectiveDeleteMaxAttempts returns DeleteMaxAttempts or its default.
func (c *StorageConfig) EffectiveDeleteMaxAttempts() int {
	return coalescePositive(c.DeleteMaxAttempts, DefaultDeleteMaxAttempts)
}

// EffectiveDeleteLeaseWindow returns DeleteLeaseWindow or its default.
func (c *StorageConfig) EffectiveDeleteLeaseWindow() time.Duration {
	return coalescePositive(c.DeleteLeaseWindow, DefaultDeleteLeaseWindow)
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
