package lock

import "time"

// Default values applied when the corresponding Option is omitted.
const (
	// DefaultTTL is the lease duration when WithTTL is not given.
	DefaultTTL = 30 * time.Second
	// DefaultRetryInterval is the polling cadence of a waiting Acquire.
	DefaultRetryInterval = 100 * time.Millisecond
)

// Option customizes a single acquisition.
type Option func(*acquireConfig)

type acquireConfig struct {
	ttl           time.Duration
	wait          time.Duration
	retryInterval time.Duration
	autoRenew     bool
}

// WithTTL sets the lease duration. The lease auto-expires this long after the
// acquisition (or the last refresh), bounding how long a crashed holder can
// block others. Non-positive values fall back to DefaultTTL.
func WithTTL(ttl time.Duration) Option {
	return func(c *acquireConfig) {
		c.ttl = ttl
	}
}

// WithWait allows Acquire to keep retrying for up to wait before giving up
// with ErrNotAcquired. The default is no waiting: a held lock fails the
// acquisition immediately.
func WithWait(wait time.Duration) Option {
	return func(c *acquireConfig) {
		c.wait = wait
	}
}

// WithRetryInterval sets the polling cadence used while waiting for a held
// lock. Non-positive values fall back to DefaultRetryInterval.
func WithRetryInterval(interval time.Duration) Option {
	return func(c *acquireConfig) {
		c.retryInterval = interval
	}
}

// WithAutoRenew toggles the background watchdog that refreshes the lease at a
// third of its TTL, so a healthy holder never expires mid-work while a crashed
// one still frees the lock within one TTL. Off by default for bare Acquire /
// TryAcquire; WithLock turns it on unless explicitly disabled.
func WithAutoRenew(enabled bool) Option {
	return func(c *acquireConfig) {
		c.autoRenew = enabled
	}
}

// resolveAcquireConfig applies opts over the defaults.
func resolveAcquireConfig(opts []Option) acquireConfig {
	cfg := acquireConfig{
		ttl:           DefaultTTL,
		retryInterval: DefaultRetryInterval,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.ttl <= 0 {
		cfg.ttl = DefaultTTL
	}

	if cfg.retryInterval <= 0 {
		cfg.retryInterval = DefaultRetryInterval
	}

	return cfg
}
