package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	istorage "github.com/coldsmirk/vef-framework-go/internal/storage"
	"github.com/coldsmirk/vef-framework-go/internal/storage/memory"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestNew tests new functionality.
func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		storageConfig *config.StorageConfig
		expectError   bool
		errorContains string
		validateType  func(*testing.T, storage.Service)
	}{
		{
			name: "MemoryService",
			storageConfig: &config.StorageConfig{
				Provider: config.StorageMemory,
			},
			expectError: false,
			validateType: func(t *testing.T, service storage.Service) {
				_, ok := service.(*memory.Service)
				assert.True(t, ok, "Service should be MemoryService")
			},
		},
		{
			name: "UnsupportedService",
			storageConfig: &config.StorageConfig{
				Provider: "unsupported",
			},
			expectError:   true,
			errorContains: "unsupported storage provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := istorage.NewService(tt.storageConfig, &config.AppConfig{})

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid configuration")
				assert.Nil(t, service, "Service should be nil on error")

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error message should contain expected text")
				}
			} else {
				require.NoError(t, err, "Should create service without error")
				require.NotNil(t, service, "Service should not be nil")

				if tt.validateType != nil {
					tt.validateType(t, service)
				}
			}
		})
	}
}
