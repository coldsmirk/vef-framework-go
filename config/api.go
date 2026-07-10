package config

import "time"

// Default values for APIRateLimitConfig, applied by the Effective* accessors.
const (
	DefaultAPIRateLimitMax    = 100
	DefaultAPIRateLimitPeriod = 5 * time.Minute
)

// APIConfig configures cross-cutting behavior of the API engine
// (`vef.api`). Zero values resolve to defaults through the Effective*
// accessors.
type APIConfig struct {
	// RateLimit is the default rate limit applied to every operation that does
	// not declare its own via api.OperationSpec.RateLimit.
	RateLimit APIRateLimitConfig `config:"rate_limit"`
}

// APIRateLimitConfig bounds request throughput. The limiter counts per
// operation and client (resource:version:action + IP + principal) in a sliding
// window held in process memory, so in a multi-node deployment each node
// enforces the limit independently.
type APIRateLimitConfig struct {
	// Max is the number of requests admitted per window; 0 resolves to the
	// default. Default: 100.
	Max int `config:"max"`
	// Period is the sliding-window length; 0 resolves to the default.
	// Default: 5m.
	Period time.Duration `config:"period"`
}

// EffectiveMax returns Max or its default.
func (c *APIRateLimitConfig) EffectiveMax() int {
	return coalescePositive(c.Max, DefaultAPIRateLimitMax)
}

// EffectivePeriod returns Period or its default.
func (c *APIRateLimitConfig) EffectivePeriod() time.Duration {
	return coalescePositive(c.Period, DefaultAPIRateLimitPeriod)
}
