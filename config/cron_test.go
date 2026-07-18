package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCronStoreConfigEffectiveDefaults(t *testing.T) {
	var cfg CronStoreConfig

	assert.Equal(t, DefaultCronStorePollInterval, cfg.EffectivePollInterval(), "zero poll interval must default")
	assert.Equal(t, DefaultCronStoreBatchSize, cfg.EffectiveBatchSize(), "zero batch size must default")
	assert.Equal(t, DefaultCronStoreMaxConcurrent, cfg.EffectiveMaxConcurrent(), "zero max concurrent must default")
	assert.Equal(t, DefaultCronStoreMisfireThreshold, cfg.EffectiveMisfireThreshold(), "zero misfire threshold must default")
	assert.Equal(t, DefaultCronStoreHeartbeatInterval, cfg.EffectiveHeartbeatInterval(), "zero heartbeat interval must default")
	assert.Equal(t, DefaultCronStoreAbandonedAfter, cfg.EffectiveAbandonedAfter(), "zero abandoned window must default")
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

	assert.Equal(t, time.Second, cfg.EffectivePollInterval(), "explicit poll interval must win")
	assert.Equal(t, 5, cfg.EffectiveBatchSize(), "explicit batch size must win")
	assert.Equal(t, 2, cfg.EffectiveMaxConcurrent(), "explicit max concurrent must win")
	assert.Equal(t, 30*time.Second, cfg.EffectiveMisfireThreshold(), "explicit misfire threshold must win")
	assert.Equal(t, 3*time.Second, cfg.EffectiveHeartbeatInterval(), "explicit heartbeat interval must win")
	assert.Equal(t, 20*time.Second, cfg.EffectiveAbandonedAfter(), "explicit abandoned window must win")
}

func TestCronConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  CronConfig
		wantErr error
	}{
		{name: "zero config", config: CronConfig{}},
		{
			name: "coherent explicit settings",
			config: CronConfig{Store: CronStoreConfig{
				HeartbeatInterval: 5 * time.Second,
				AbandonedAfter:    10 * time.Second,
				RunRetention:      30 * 24 * time.Hour,
			}},
		},
		{
			name:    "negative duration",
			config:  CronConfig{Store: CronStoreConfig{RunRetention: -time.Hour}},
			wantErr: ErrInvalidCronStoreDuration,
		},
		{
			name:    "negative run timeout",
			config:  CronConfig{Store: CronStoreConfig{RunTimeout: -time.Second}},
			wantErr: ErrInvalidCronStoreDuration,
		},
		{
			name: "abandoned window tighter than heartbeat cadence",
			config: CronConfig{Store: CronStoreConfig{
				HeartbeatInterval: 30 * time.Second,
				AbandonedAfter:    45 * time.Second,
			}},
			wantErr: ErrCronStoreAbandonedTooSoon,
		},
		{
			name: "abandoned window against default heartbeat",
			config: CronConfig{Store: CronStoreConfig{
				AbandonedAfter: 15 * time.Second,
			}},
			wantErr: ErrCronStoreAbandonedTooSoon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err, "config must validate")
			} else {
				assert.ErrorIs(t, err, tt.wantErr, "validation must fail with the expected sentinel")
			}
		})
	}
}
