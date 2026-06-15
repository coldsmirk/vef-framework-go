package storage

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/storage/filesystem"
	"github.com/coldsmirk/vef-framework-go/internal/storage/memory"
	"github.com/coldsmirk/vef-framework-go/internal/storage/minio"
	"github.com/coldsmirk/vef-framework-go/storage"
)

func NewService(cfg *config.StorageConfig, appCfg *config.AppConfig) (storage.Service, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = config.StorageMemory

		logger.Warnf(
			"storage provider not configured; defaulting to in-memory storage — objects are lost on restart. Set vef.storage.provider (filesystem or minio) for any non-test deployment.",
		)
	}

	switch provider {
	case config.StorageMinIO:
		return minio.New(cfg.MinIO, appCfg)
	case config.StorageMemory:
		return memory.New(), nil
	case config.StorageFilesystem:
		return filesystem.New(cfg.Filesystem)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageProvider, provider)
	}
}
