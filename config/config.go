package config

// Config provides access to application configuration values.
type Config interface {
	// Unmarshal decodes configuration at the given key into target.
	Unmarshal(key string, target any) error
}
