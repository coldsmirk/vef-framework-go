package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestRedisStreamConfigIncludesClaimOverrides(t *testing.T) {
	cfg := &config.EventConfig{
		Transports: config.EventTransportsConfig{
			RedisStream: config.EventRedisStreamTransportConfig{
				StreamPrefix:   "custom",
				MaxLenApprox:   123,
				BlockTimeout:   2 * time.Second,
				ClaimIdle:      3 * time.Second,
				ClaimInterval:  4 * time.Second,
				ClaimBatchSize: 17,
				ConsumerID:     "consumer-1",
				StartID:        "42-0",
			},
		},
	}

	got := redisStreamConfig(cfg)

	require.Equal(t, "custom", got.StreamPrefix, "Stream prefix should be copied")
	require.EqualValues(t, 123, got.MaxLenApprox, "Max len should be copied")
	require.Equal(t, 2*time.Second, got.BlockTimeout, "Block timeout should be copied")
	require.Equal(t, 3*time.Second, got.ClaimIdle, "Claim idle should be copied")
	require.Equal(t, 4*time.Second, got.ClaimInterval, "Claim interval should be copied")
	require.EqualValues(t, 17, got.ClaimBatchSize, "Claim batch size should be copied")
	require.Equal(t, "consumer-1", got.ConsumerID, "Consumer id should be copied")
	require.Equal(t, "42-0", got.StartID, "Start id should be copied")
}
