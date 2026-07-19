package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPushConfigEffectiveDefaults(t *testing.T) {
	cfg := new(PushConfig)

	assert.Equal(t, "/ws", cfg.EffectivePath(), "Zero path should default to /ws")
	assert.Equal(t, 30*time.Second, cfg.EffectivePingInterval(), "Zero ping interval should default to 30s")
	assert.Equal(t, 10*time.Second, cfg.EffectiveWriteTimeout(), "Zero write timeout should default to 10s")
	assert.Equal(t, 32, cfg.EffectiveSendBuffer(), "Zero send buffer should default to 32")
	assert.Equal(t, 60*time.Second, cfg.EffectiveSessionRecheckInterval(), "Zero recheck interval should default to 60s")
}

func TestPushConfigEffectiveOverrides(t *testing.T) {
	cfg := &PushConfig{
		Path:                   "/realtime",
		PingInterval:           5 * time.Second,
		WriteTimeout:           time.Second,
		SendBuffer:             8,
		SessionRecheckInterval: 10 * time.Second,
	}

	assert.Equal(t, "/realtime", cfg.EffectivePath(), "Configured path should pass through")
	assert.Equal(t, 5*time.Second, cfg.EffectivePingInterval(), "Configured ping interval should pass through")
	assert.Equal(t, time.Second, cfg.EffectiveWriteTimeout(), "Configured write timeout should pass through")
	assert.Equal(t, 8, cfg.EffectiveSendBuffer(), "Configured send buffer should pass through")
	assert.Equal(t, 10*time.Second, cfg.EffectiveSessionRecheckInterval(), "Configured recheck interval should pass through")
}
