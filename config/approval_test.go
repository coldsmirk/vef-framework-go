package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestApprovalBusinessBindingConfigDefaults(t *testing.T) {
	cfg := config.ApprovalBusinessBindingConfig{}

	require.Equal(t, config.ApprovalBindingSynchronous, cfg.EffectiveConsistency(),
		"Unset consistency should default to synchronous rollback semantics")
	require.Equal(t, 10*time.Second, cfg.EffectiveScanInterval(),
		"Unset scan interval should default to ten seconds")
	require.Equal(t, 100, cfg.EffectiveBatchSize(),
		"Unset batch size should default to one hundred")
	require.NoError(t, cfg.Validate(), "Default business binding config should be valid")
}

func TestApprovalBusinessBindingConfigOverrides(t *testing.T) {
	cfg := config.ApprovalBusinessBindingConfig{
		Consistency:  config.ApprovalBindingEventual,
		ScanInterval: time.Minute,
		BatchSize:    25,
	}

	require.Equal(t, config.ApprovalBindingEventual, cfg.EffectiveConsistency(),
		"Configured eventual consistency should pass through")
	require.Equal(t, time.Minute, cfg.EffectiveScanInterval(),
		"Configured scan interval should pass through")
	require.Equal(t, 25, cfg.EffectiveBatchSize(),
		"Configured batch size should pass through")
	require.NoError(t, cfg.Validate(), "Supported overrides should be valid")
}

func TestApprovalBusinessBindingConfigValidation(t *testing.T) {
	t.Run("RejectsUnknownConsistency", func(t *testing.T) {
		cfg := config.ApprovalBusinessBindingConfig{Consistency: "async"}
		require.ErrorIs(t, cfg.Validate(), config.ErrInvalidApprovalBindingConsistency,
			"Unknown consistency should fail startup validation")
	})

	t.Run("RejectsNegativeScanInterval", func(t *testing.T) {
		cfg := config.ApprovalBusinessBindingConfig{ScanInterval: -time.Second}
		require.ErrorIs(t, cfg.Validate(), config.ErrInvalidApprovalBusinessBindingWorkerConfig,
			"Negative scan interval should fail startup validation")
	})

	t.Run("RejectsNegativeBatchSize", func(t *testing.T) {
		cfg := config.ApprovalBusinessBindingConfig{BatchSize: -1}
		require.ErrorIs(t, cfg.Validate(), config.ErrInvalidApprovalBusinessBindingWorkerConfig,
			"Negative batch size should fail startup validation")
	})
}
