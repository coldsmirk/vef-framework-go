package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCronStoreConfigEffectiveDefaults(t *testing.T) {
	var cfg CronStoreConfig

	assert.Equal(t, DefaultCronStorePollInterval, cfg.EffectivePollInterval(), "Zero poll interval must default")
	assert.Equal(t, DefaultCronStoreBatchSize, cfg.EffectiveBatchSize(), "Zero batch size must default")
	assert.Equal(t, DefaultCronStoreMaxConcurrent, cfg.EffectiveMaxConcurrent(), "Zero max concurrent must default")
	assert.Equal(t, DefaultCronStoreMisfireThreshold, cfg.EffectiveMisfireThreshold(), "Zero misfire threshold must default")
	assert.Equal(t, DefaultCronStoreHeartbeatInterval, cfg.EffectiveHeartbeatInterval(), "Zero heartbeat interval must default")
	assert.Equal(t, DefaultCronStoreAbandonedAfter, cfg.EffectiveAbandonedAfter(), "Zero abandoned window must default")
}

func TestCronStoreConfigEffectiveOverrides(t *testing.T) {
	cfg := CronStoreConfig{
		PollInterval:      time.Second,
		BatchSize:         5,
		MaxConcurrent:     2,
		MisfireThreshold:  30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		AbandonedAfter:    20 * time.Second,
	}

	assert.Equal(t, time.Second, cfg.EffectivePollInterval(), "Explicit poll interval must win")
	assert.Equal(t, 5, cfg.EffectiveBatchSize(), "Explicit batch size must win")
	assert.Equal(t, 2, cfg.EffectiveMaxConcurrent(), "Explicit max concurrent must win")
	assert.Equal(t, 30*time.Second, cfg.EffectiveMisfireThreshold(), "Explicit misfire threshold must win")
	assert.Equal(t, 3*time.Second, cfg.EffectiveHeartbeatInterval(), "Explicit heartbeat interval must win")
	assert.Equal(t, 20*time.Second, cfg.EffectiveAbandonedAfter(), "Explicit abandoned window must win")
}

func TestCronConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  CronConfig
		wantErr error
	}{
		{name: "ZeroConfig", config: CronConfig{}},
		{
			name: "CoherentExplicitSettings",
			config: CronConfig{Store: CronStoreConfig{
				HeartbeatInterval: 5 * time.Second,
				AbandonedAfter:    10 * time.Second,
				RunRetention:      30 * 24 * time.Hour,
			}},
		},
		{
			name:    "NegativeDuration",
			config:  CronConfig{Store: CronStoreConfig{RunRetention: -time.Hour}},
			wantErr: ErrInvalidCronStoreDuration,
		},
		{
			name:    "NegativeRunTimeout",
			config:  CronConfig{Store: CronStoreConfig{RunTimeout: -time.Second}},
			wantErr: ErrInvalidCronStoreDuration,
		},
		{
			name: "AbandonedWindowTighterThanHeartbeatCadence",
			config: CronConfig{Store: CronStoreConfig{
				HeartbeatInterval: 30 * time.Second,
				AbandonedAfter:    45 * time.Second,
			}},
			wantErr: ErrCronStoreAbandonedTooSoon,
		},
		{
			name: "AbandonedWindowAgainstDefaultHeartbeat",
			config: CronConfig{Store: CronStoreConfig{
				AbandonedAfter: 15 * time.Second,
			}},
			wantErr: ErrCronStoreAbandonedTooSoon,
		},
		{
			name: "HeartbeatComparisonDoesNotOverflow",
			config: CronConfig{Store: CronStoreConfig{
				HeartbeatInterval: time.Duration(1<<63 - 1),
				AbandonedAfter:    time.Duration(1<<63 - 1),
			}},
			wantErr: ErrCronStoreAbandonedTooSoon,
		},
		{
			name:    "NegativeBatchSize",
			config:  CronConfig{Store: CronStoreConfig{BatchSize: -1}},
			wantErr: ErrInvalidCronStoreCount,
		},
		{
			name:    "NegativeMaxConcurrent",
			config:  CronConfig{Store: CronStoreConfig{MaxConcurrent: -8}},
			wantErr: ErrInvalidCronStoreCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err, "Config must validate")
			} else {
				assert.ErrorIs(t, err, tt.wantErr, "Validation must fail with the expected sentinel")
			}
		})
	}
}
