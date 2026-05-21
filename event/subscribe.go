package event

// SubscribeOption mutates subscription-time configuration. Options
// compose left-to-right.
type SubscribeOption func(*SubscribeConfig)

// SubscribeConfig carries the resolved per-subscription settings.
type SubscribeConfig struct {
	// Group is the consumer group name. Two subscriptions sharing a
	// group on a SupportsGroups transport receive at most one delivery
	// per message between them (load balancing).
	Group string
	// Concurrency is the desired worker count for parallel handler
	// dispatch within this subscription. Defaults to 1.
	Concurrency int
}

// ApplySubscribeOptions returns a fully resolved SubscribeConfig.
func ApplySubscribeOptions(opts []SubscribeOption) SubscribeConfig {
	var cfg SubscribeConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// WithGroup sets the consumer group name. Required for any subscription
// whose resolved route touches an at-least-once transport (outbox,
// Redis Streams, …) — the group is the Inbox dedupe scope and the
// Redis Streams XGROUP identifier, both of which must remain stable
// across process restarts. Bus.Subscribe returns ErrGroupRequired if
// the group is missing on such a route.
func WithGroup(name string) SubscribeOption {
	return func(c *SubscribeConfig) { c.Group = name }
}

// WithConcurrency sets the per-subscription worker count.
func WithConcurrency(n int) SubscribeOption {
	return func(c *SubscribeConfig) {
		if n > 0 {
			c.Concurrency = n
		}
	}
}
