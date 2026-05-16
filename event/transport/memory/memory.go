// Package memory declares the configuration surface for the in-process
// Transport. It is the default for single-node deployments and the
// canonical sink for outbox-style transports that need a downstream
// dispatcher.
package memory

import "time"

// Name is the stable identifier used in routing configuration.
const Name = "memory"

// FullPolicy controls what happens when a subscription queue is full.
type FullPolicy string

const (
	// FullPolicyError rejects new publishes with ErrQueueFull. The
	// caller decides whether to retry or fail the upstream operation.
	// This is the default — it makes back-pressure explicit.
	FullPolicyError FullPolicy = "error"
	// FullPolicyBlock blocks the publisher until the queue drains.
	// Use only when the publisher can tolerate latency spikes.
	FullPolicyBlock FullPolicy = "block"
	// FullPolicyDropOldest evicts the oldest pending frame to make
	// room for the new one. Use for throwaway telemetry where loss
	// is preferable to either error or blocking.
	FullPolicyDropOldest FullPolicy = "drop_oldest"
)

// Config configures a memory Transport instance.
type Config struct {
	// QueueSize is the per-subscription channel capacity. Defaults
	// to 1024 when zero or negative.
	QueueSize int
	// FullPolicy controls the action when QueueSize is reached.
	// Defaults to FullPolicyError when empty.
	FullPolicy FullPolicy
	// PublishTimeout caps Publish in the block-policy case. Zero
	// means no timeout.
	PublishTimeout time.Duration
}

// EffectiveQueueSize applies the default when QueueSize is unset.
func (c Config) EffectiveQueueSize() int {
	if c.QueueSize > 0 {
		return c.QueueSize
	}

	return 1024
}

// EffectiveFullPolicy applies the default when FullPolicy is unset.
func (c Config) EffectiveFullPolicy() FullPolicy {
	if c.FullPolicy != "" {
		return c.FullPolicy
	}

	return FullPolicyError
}
