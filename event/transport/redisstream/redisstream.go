// Package redisstream declares the cross-process Transport contract
// backed by Redis Streams.
//
// Each event type maps to one stream: StreamPrefix + EventType. Consumer
// groups are configured per Subscribe call; consumer IDs default to the
// framework-wide VEF_NODE_ID so that XCLAIM can adopt orphaned messages
// after a crash.
package redisstream

import "time"

// Name is the stable identifier used in routing configuration.
const Name = "redisstream"

// Config configures a redisstream Transport instance.
type Config struct {
	// StreamPrefix is prepended to event types to form the Redis
	// Stream key. Defaults to "vef:events:".
	StreamPrefix string
	// MaxLenApprox caps each stream length using XADD MAXLEN ~ N.
	// Zero disables trimming.
	MaxLenApprox int64
	// BlockTimeout bounds a single XREADGROUP call.
	BlockTimeout time.Duration
	// ClaimIdle is the idle threshold beyond which the reaper
	// XCLAIMs a pending message from another consumer.
	ClaimIdle time.Duration
	// ClaimInterval is the period of the reaper loop.
	ClaimInterval time.Duration
	// ClaimBatchSize bounds the number of pending entries the reaper
	// inspects per cycle. Defaults to 64.
	ClaimBatchSize int64
	// ConsumerID overrides the consumer name within a group. When
	// empty the transport derives one from VEF_NODE_ID + a random
	// suffix.
	ConsumerID string
	// StartID is the Redis Streams ID a newly created consumer group
	// resumes from. Defaults to "0" so messages produced before the
	// group existed are still delivered (at-least-once safety). Set
	// to "$" for fire-and-forget topics where backlog should be
	// dropped on first subscribe.
	StartID string
}

// EffectiveStreamPrefix applies the default when unset.
func (c Config) EffectiveStreamPrefix() string {
	if c.StreamPrefix != "" {
		return c.StreamPrefix
	}

	return "vef:events:"
}

// EffectiveBlockTimeout applies the default when unset.
func (c Config) EffectiveBlockTimeout() time.Duration {
	if c.BlockTimeout > 0 {
		return c.BlockTimeout
	}

	return 5 * time.Second
}

// EffectiveClaimIdle applies the default when unset.
func (c Config) EffectiveClaimIdle() time.Duration {
	if c.ClaimIdle > 0 {
		return c.ClaimIdle
	}

	return 60 * time.Second
}

// EffectiveClaimInterval applies the default when unset.
func (c Config) EffectiveClaimInterval() time.Duration {
	if c.ClaimInterval > 0 {
		return c.ClaimInterval
	}

	return 30 * time.Second
}

// EffectiveClaimBatchSize applies the default when unset.
func (c Config) EffectiveClaimBatchSize() int64 {
	if c.ClaimBatchSize > 0 {
		return c.ClaimBatchSize
	}

	return 64
}

// EffectiveStartID applies the default when unset.
func (c Config) EffectiveStartID() string {
	if c.StartID != "" {
		return c.StartID
	}

	return "0"
}

// StreamKey composes the Redis key for an event type.
func (c Config) StreamKey(eventType string) string {
	return c.EffectiveStreamPrefix() + eventType
}
