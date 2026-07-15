package jsevents

// libConfig collects the settings resolved from Options.
type libConfig struct {
	allowedTypes []string
}

// Option customizes the events library.
type Option func(*libConfig)

// WithAllowedTypes restricts the event types scripts may publish. A pattern
// is either an exact type ("report.generated"), a prefix wildcard matching
// descendants ("script.*"), or "*" for everything. Without this option every
// type is allowed — restrict it when framework event namespaces (vef.*,
// approval.*) must stay out of the scripts' reach.
func WithAllowedTypes(patterns ...string) Option {
	return func(c *libConfig) {
		c.allowedTypes = append(c.allowedTypes, patterns...)
	}
}
