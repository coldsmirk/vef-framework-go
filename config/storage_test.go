package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestStorageConfigEffectiveDefaults(t *testing.T) {
	// A zero-valued config must yield every documented default. This pins the
	// coalescePositive zero branch for all accessors at once.
	cfg := config.StorageConfig{}

	require.Equal(t, config.DefaultMaxUploadSize, cfg.EffectiveMaxUploadSize(),
		"Zero max upload size should fall back to the default")
	require.Equal(t, config.DefaultClaimTTL, cfg.EffectiveClaimTTL(),
		"Zero claim TTL should fall back to the default")
	require.Equal(t, config.DefaultMaxPendingClaims, cfg.EffectiveMaxPendingClaims(),
		"Zero max pending claims should fall back to the default")
	require.Equal(t, config.DefaultSweepInterval, cfg.EffectiveSweepInterval(),
		"Zero sweep interval should fall back to the default")
	require.Equal(t, config.DefaultSweepBatchSize, cfg.EffectiveSweepBatchSize(),
		"Zero sweep batch size should fall back to the default")
	require.Equal(t, config.DefaultDeleteWorkerInterval, cfg.EffectiveDeleteWorkerInterval(),
		"Zero delete worker interval should fall back to the default")
	require.Equal(t, config.DefaultDeleteBatchSize, cfg.EffectiveDeleteBatchSize(),
		"Zero delete batch size should fall back to the default")
	require.Equal(t, config.DefaultDeleteConcurrency, cfg.EffectiveDeleteConcurrency(),
		"Zero delete concurrency should fall back to the default")
	require.Equal(t, config.DefaultDeleteMaxAttempts, cfg.EffectiveDeleteMaxAttempts(),
		"Zero delete max attempts should fall back to the default")
	require.Equal(t, config.DefaultDeleteLeaseWindow, cfg.EffectiveDeleteLeaseWindow(),
		"Zero delete lease window should fall back to the default")
}

func TestStorageConfigEffectiveNegativeFallsBack(t *testing.T) {
	// coalescePositive treats negatives as misconfigured (v <= zero) and
	// re-selects the default. Exercise the negative branch across types.
	cfg := config.StorageConfig{
		MaxUploadSize:        -1,
		ClaimTTL:             -time.Second,
		MaxPendingClaims:     -5,
		DeleteConcurrency:    -8,
		DeleteLeaseWindow:    -time.Minute,
		DeleteMaxAttempts:    -1,
		SweepBatchSize:       -10,
		DeleteWorkerInterval: -time.Hour,
	}

	require.Equal(t, config.DefaultMaxUploadSize, cfg.EffectiveMaxUploadSize(),
		"Negative max upload size should re-select the default")
	require.Equal(t, config.DefaultClaimTTL, cfg.EffectiveClaimTTL(),
		"Negative claim TTL should re-select the default")
	require.Equal(t, config.DefaultMaxPendingClaims, cfg.EffectiveMaxPendingClaims(),
		"Negative max pending claims should re-select the default")
	require.Equal(t, config.DefaultDeleteConcurrency, cfg.EffectiveDeleteConcurrency(),
		"Negative delete concurrency should re-select the default")
	require.Equal(t, config.DefaultDeleteLeaseWindow, cfg.EffectiveDeleteLeaseWindow(),
		"Negative delete lease window should re-select the default")
	require.Equal(t, config.DefaultDeleteMaxAttempts, cfg.EffectiveDeleteMaxAttempts(),
		"Negative delete max attempts should re-select the default")
	require.Equal(t, config.DefaultSweepBatchSize, cfg.EffectiveSweepBatchSize(),
		"Negative sweep batch size should re-select the default")
	require.Equal(t, config.DefaultDeleteWorkerInterval, cfg.EffectiveDeleteWorkerInterval(),
		"Negative delete worker interval should re-select the default")
}

func TestStorageConfigEffectivePositivePassthrough(t *testing.T) {
	// The positive branch must return the configured value verbatim.
	cfg := config.StorageConfig{
		MaxUploadSize:        2048,
		ClaimTTL:             48 * time.Hour,
		MaxPendingClaims:     7,
		SweepInterval:        time.Minute,
		SweepBatchSize:       50,
		DeleteWorkerInterval: 2 * time.Minute,
		DeleteBatchSize:      33,
		DeleteConcurrency:    4,
		DeleteMaxAttempts:    3,
		DeleteLeaseWindow:    90 * time.Second,
	}

	require.Equal(t, int64(2048), cfg.EffectiveMaxUploadSize(),
		"Positive max upload size should pass through unchanged")
	require.Equal(t, 48*time.Hour, cfg.EffectiveClaimTTL(),
		"Positive claim TTL should pass through unchanged")
	require.Equal(t, 7, cfg.EffectiveMaxPendingClaims(),
		"Positive max pending claims should pass through unchanged")
	require.Equal(t, time.Minute, cfg.EffectiveSweepInterval(),
		"Positive sweep interval should pass through unchanged")
	require.Equal(t, 50, cfg.EffectiveSweepBatchSize(),
		"Positive sweep batch size should pass through unchanged")
	require.Equal(t, 2*time.Minute, cfg.EffectiveDeleteWorkerInterval(),
		"Positive delete worker interval should pass through unchanged")
	require.Equal(t, 33, cfg.EffectiveDeleteBatchSize(),
		"Positive delete batch size should pass through unchanged")
	require.Equal(t, 4, cfg.EffectiveDeleteConcurrency(),
		"Positive delete concurrency should pass through unchanged")
	require.Equal(t, 3, cfg.EffectiveDeleteMaxAttempts(),
		"Positive delete max attempts should pass through unchanged")
	require.Equal(t, 90*time.Second, cfg.EffectiveDeleteLeaseWindow(),
		"Positive delete lease window should pass through unchanged")
}
