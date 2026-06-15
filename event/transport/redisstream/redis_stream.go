package redisstream

import "time"

// Name is the stable identifier used in routing configuration.
const Name = "redis_stream"

// Config configures a redis_stream Transport instance.
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
	// ReaperConcurrency bounds how many subscriptions the reaper reclaims
	// in parallel per cycle. Reclaim invokes the user handler inline, so
	// a single slow handler must not serialize failover across unrelated
	// streams; fanning out with a bounded pool isolates the blast radius.
	// Defaults to 4.
	ReaperConcurrency int
	// HandlerTimeout bounds a single handler invocation (both fresh
	// XREADGROUP deliveries and reaper redeliveries). A hung handler is
	// canceled via context once the deadline elapses, freeing the worker
	// instead of pinning it indefinitely; the message stays pending for a
	// later retry. Zero disables the deadline (handlers run under the
	// transport lifecycle context only).
	HandlerTimeout time.Duration
	// SetupTimeout bounds the XGROUP CREATE issued by Subscribe so a
	// stalled Redis fails the subscription loudly instead of wedging the
	// caller (and fx startup) on the deadline-less transport lifecycle
	// context. Defaults to 5s.
	SetupTimeout time.Duration
	// ConsumerID is an optional human-readable prefix for the consumer
	// name within a group (useful for observability via XINFO CONSUMERS).
	// A unique per-process suffix is always appended so replicas sharing
	// the same config never collide within a consumer group. When empty,
	// "vef" is used as the prefix.
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

// EffectiveReaperConcurrency applies the default when unset.
func (c Config) EffectiveReaperConcurrency() int {
	if c.ReaperConcurrency > 0 {
		return c.ReaperConcurrency
	}

	return 4
}

// EffectiveSetupTimeout applies the default when unset.
func (c Config) EffectiveSetupTimeout() time.Duration {
	if c.SetupTimeout > 0 {
		return c.SetupTimeout
	}

	return 5 * time.Second
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
