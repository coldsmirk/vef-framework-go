package config

// StorageProvider represents supported storage backend types.
type StorageProvider string

// Supported storage providers.
const (
	StorageMinIO      StorageProvider = "minio"
	StorageMemory     StorageProvider = "memory"
	StorageFilesystem StorageProvider = "filesystem"
)

// StorageConfig defines storage provider settings.
type StorageConfig struct {
	Provider   StorageProvider  `config:"provider"`
	MinIO      MinIOConfig      `config:"minio"`
	Filesystem FilesystemConfig `config:"filesystem"`
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
