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

func TestApprovalFormDataMaxBytes(t *testing.T) {
	t.Run("UnsetDefaultsTo64KiB", func(t *testing.T) {
		cfg := config.ApprovalConfig{}

		require.Equal(t, 64*1024, cfg.EffectiveFormDataMaxBytes(),
			"Unset form_data_max_bytes should default to 64 KiB")
		require.Equal(t, config.DefaultFormDataMaxBytes, cfg.EffectiveFormDataMaxBytes(),
			"The accessor default and the exported constant must agree")
	})

	t.Run("OverrideWins", func(t *testing.T) {
		cfg := config.ApprovalConfig{FormDataMaxBytes: 4 * 1024 * 1024}

		require.Equal(t, 4*1024*1024, cfg.EffectiveFormDataMaxBytes(),
			"A configured cap should be returned verbatim")
		require.NoError(t, cfg.Validate(), "A positive cap should be valid")
	})

	t.Run("NegativeIsRejected", func(t *testing.T) {
		// Zero means "use the default"; a negative value is a typo that would
		// otherwise reject every submission, so it must fail at boot.
		cfg := config.ApprovalConfig{FormDataMaxBytes: -1}

		err := cfg.Validate()
		require.Error(t, err, "A negative cap should fail configuration validation")
		require.ErrorIs(t, err, config.ErrInvalidApprovalFormDataMaxBytes,
			"Error should wrap ErrInvalidApprovalFormDataMaxBytes so operators can match it")
	})
}
