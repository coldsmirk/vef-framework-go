package jscache

// libConfig collects the settings resolved from Options.
type libConfig struct {
	keyPrefix string
}

// Option customizes the cache library.
type Option func(*libConfig)

// WithKeyPrefix prepends prefix to every key the scripts touch, sandboxing
// them into their own key namespace so host cache entries stay out of reach.
// The prefix is applied verbatim — include a separator (e.g. "js:") if one is
// wanted.
func WithKeyPrefix(prefix string) Option {
	return func(c *libConfig) {
		c.keyPrefix = prefix
	}
}
