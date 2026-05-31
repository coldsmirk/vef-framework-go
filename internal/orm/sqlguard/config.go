package sqlguard

// Config holds the configuration for the SQL guard.
type Config struct {
	Enabled bool
}

// DefaultConfig returns the default SQL guard configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
	}
}
