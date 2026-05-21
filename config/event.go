package config

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// EventConfig governs the framework event bus: transports, routing,
// middleware, and the consume-side Inbox table.
type EventConfig struct {
	// DefaultTransport is the route fallback when no rule matches.
	DefaultTransport string `config:"default_transport"`
	// AsyncQueueSize is the capacity of the async fan-in queue used
	// by WithAsync publishes.
	AsyncQueueSize int `config:"async_queue_size"`
	// AsyncWorkers is the number of goroutines draining the async
	// fan-in queue.
	AsyncWorkers int `config:"async_workers"`
	// PublishTimeout caps an individual transport.Publish call.
	PublishTimeout time.Duration `config:"publish_timeout"`

	Transports EventTransportsConfig `config:"transports"`
	Middleware EventMiddlewareConfig `config:"middleware"`
	Inbox      EventInboxConfig      `config:"inbox"`
	// Routing is matched top-to-bottom; the first rule whose Pattern
	// matches via path.Match wins. fan-out is expressed by listing
	// multiple transports in Transports.
	Routing []EventRoutingRule `config:"routing"`
}

// EventTransportsConfig groups per-transport configuration blocks.
type EventTransportsConfig struct {
	Memory      EventMemoryTransportConfig      `config:"memory"`
	Outbox      EventOutboxTransportConfig      `config:"outbox"`
	RedisStream EventRedisStreamTransportConfig `config:"redis_stream"`
}

// EventMemoryTransportConfig configures the in-process transport.
type EventMemoryTransportConfig struct {
	QueueSize      int           `config:"queue_size"`
	FullPolicy     string        `config:"full_policy"` // error | block | drop_oldest
	PublishTimeout time.Duration `config:"publish_timeout"`
}

// EventOutboxTransportConfig configures the persistent outbox transport.
type EventOutboxTransportConfig struct {
	Enabled         bool          `config:"enabled"`
	RelayInterval   time.Duration `config:"relay_interval"`
	MaxRetries      int           `config:"max_retries"`
	BatchSize       int           `config:"batch_size"`
	LeaseMultiplier int           `config:"lease_multiplier"`
	MinLease        time.Duration `config:"min_lease"`
	SinkName        string        `config:"sink"`
	CleanupInterval time.Duration `config:"cleanup_interval"`
	CompletedTTL    time.Duration `config:"completed_ttl"`
}

// EventRedisStreamTransportConfig configures the Redis Streams transport.
type EventRedisStreamTransportConfig struct {
	Enabled        bool          `config:"enabled"`
	StreamPrefix   string        `config:"stream_prefix"`
	MaxLenApprox   int64         `config:"max_len_approx"`
	BlockTimeout   time.Duration `config:"block_timeout"`
	ClaimIdle      time.Duration `config:"claim_idle"`
	ClaimInterval  time.Duration `config:"claim_interval"`
	ClaimBatchSize int64         `config:"claim_batch_size"`
	ConsumerID     string        `config:"consumer_id"`
	StartID        string        `config:"start_id"`
}

// EventMiddlewareConfig toggles the built-in consume/publish middlewares.
type EventMiddlewareConfig struct {
	Logging bool `config:"logging"`
	Tracing bool `config:"tracing"`
	// TracingStrict, when true, switches the Tracing middleware to
	// strict mode: incoming TraceIDs from cross-process transports are
	// treated as untrusted and parked under IncomingTraceIDFromContext;
	// a fresh ID is generated for log correlation. Default (false) is
	// W3C / OpenTelemetry-compatible propagation suitable for intra-
	// cluster pub/sub. Toggle this on at the edge between trust zones.
	TracingStrict bool `config:"tracing_strict"`
	Metrics       bool `config:"metrics"`
	Recover       bool `config:"recover"`
	// Inbox controls whether the consume-side idempotency middleware
	// is enabled. It only activates on transports whose Capabilities
	// declare AtLeastOnce regardless of this flag.
	Inbox bool `config:"inbox"`
}

// EventInboxConfig governs the inbox table retention and cleanup.
type EventInboxConfig struct {
	Retention       time.Duration `config:"retention"`
	CleanupInterval time.Duration `config:"cleanup_interval"`
}

// EventRoutingRule matches an event type to one or more transports.
// Pattern uses path.Match semantics ("*", "?", "[abc]"). The list of
// transports expresses fan-out — each frame is dispatched to every
// listed transport.
type EventRoutingRule struct {
	Pattern    string   `config:"pattern"`
	Transports []string `config:"transports"`
}

// EffectiveDefaultTransport applies the default fallback.
func (c *EventConfig) EffectiveDefaultTransport() string {
	if c.DefaultTransport != "" {
		return c.DefaultTransport
	}

	return "memory"
}

// EffectiveAsyncQueueSize applies the default.
func (c *EventConfig) EffectiveAsyncQueueSize() int {
	if c.AsyncQueueSize > 0 {
		return c.AsyncQueueSize
	}

	return 4096
}

// EffectiveAsyncWorkers applies the default.
func (c *EventConfig) EffectiveAsyncWorkers() int {
	if c.AsyncWorkers > 0 {
		return c.AsyncWorkers
	}

	return 4
}

// EffectivePublishTimeout applies the default.
func (c *EventConfig) EffectivePublishTimeout() time.Duration {
	if c.PublishTimeout > 0 {
		return c.PublishTimeout
	}

	return 5 * time.Second
}

// EffectiveCleanupInterval applies the outbox cleanup default.
func (c *EventOutboxTransportConfig) EffectiveCleanupInterval() time.Duration {
	if c.CleanupInterval > 0 {
		return c.CleanupInterval
	}

	return time.Hour
}

// EffectiveCompletedTTL applies the outbox completed-row TTL default.
func (c *EventOutboxTransportConfig) EffectiveCompletedTTL() time.Duration {
	if c.CompletedTTL > 0 {
		return c.CompletedTTL
	}

	return 7 * 24 * time.Hour
}

// EffectiveRetention applies the default of 7 days.
func (c *EventInboxConfig) EffectiveRetention() time.Duration {
	if c.Retention > 0 {
		return c.Retention
	}

	return 7 * 24 * time.Hour
}

// EffectiveCleanupInterval applies the default of 1 hour.
func (c *EventInboxConfig) EffectiveCleanupInterval() time.Duration {
	if c.CleanupInterval > 0 {
		return c.CleanupInterval
	}

	return time.Hour
}

// ErrInboxRetentionTooShort indicates the inbox retention window is
// smaller than the worst-case exponential-backoff horizon implied by
// the outbox max_retries. With such a configuration a duplicate
// delivery from the outbox could arrive after its inbox dedupe entry
// has already been pruned, producing double-execution.
var ErrInboxRetentionTooShort = errors.New(
	"event: inbox.retention is shorter than the outbox exponential-backoff horizon")

// Validate checks invariants that cross multiple subtrees of the
// EventConfig. Called once at fx Start. Currently enforced:
//
//   - The inbox retention window must comfortably outlast the worst
//     case outbox retry horizon (sum of 2^k seconds across max_retries
//     attempts) so dedupe entries survive the longest delayed
//     duplicate.
func (c *EventConfig) Validate() error {
	if !c.Middleware.Inbox || !c.Transports.Outbox.Enabled {
		return nil
	}

	maxRetries := c.Transports.Outbox.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	backoffSecs := math.Pow(2, float64(maxRetries+1)) - 2 // sum_{k=1..N} 2^k
	horizon := time.Duration(backoffSecs) * time.Second

	retention := c.Inbox.EffectiveRetention()
	if retention <= horizon {
		return fmt.Errorf("%w: retention=%s horizon=%s (max_retries=%d)",
			ErrInboxRetentionTooShort, retention, horizon, maxRetries)
	}

	return nil
}
