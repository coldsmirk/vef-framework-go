package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestIntegrationConfigEffectiveSecretAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm config.IntegrationSecretAlgorithm
		want      config.IntegrationSecretAlgorithm
	}{
		{"EmptyDefaultsToAES", "", config.IntegrationSecretAlgorithmAES},
		{"ExplicitAES", config.IntegrationSecretAlgorithmAES, config.IntegrationSecretAlgorithmAES},
		{"ExplicitSM4", config.IntegrationSecretAlgorithmSM4, config.IntegrationSecretAlgorithmSM4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.IntegrationConfig{SecretAlgorithm: tt.algorithm}

			assert.Equal(t, tt.want, cfg.EffectiveSecretAlgorithm(), "Effective algorithm should resolve the default")
		})
	}
}

func TestIntegrationConfigValidateSecretAlgorithm(t *testing.T) {
	t.Run("AcceptsSupportedAlgorithms", func(t *testing.T) {
		for _, algorithm := range []config.IntegrationSecretAlgorithm{"", config.IntegrationSecretAlgorithmAES, config.IntegrationSecretAlgorithmSM4} {
			cfg := &config.IntegrationConfig{SecretAlgorithm: algorithm}

			require.NoError(t, cfg.Validate(), "Supported algorithm %q should validate", algorithm)
		}
	})

	t.Run("RejectsUnknownAlgorithm", func(t *testing.T) {
		cfg := &config.IntegrationConfig{SecretAlgorithm: "des"}

		err := cfg.Validate()
		require.Error(t, err, "Unknown algorithm should fail validation")
		assert.ErrorIs(t, err, config.ErrInvalidIntegrationSecretAlgorithm, "Error should be the invalid-algorithm sentinel")
	})
}
