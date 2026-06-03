package config

// Config provides access to application configuration values.
type Config interface {
	// Unmarshal decodes configuration at the given key into target.
	Unmarshal(key string, target any) error
}

// AppConfig defines core application settings.
type AppConfig struct {
	Name      string `config:"name"`
	Port      uint16 `config:"port"`
	BodyLimit string `config:"body_limit"`
	// TrustedProxies lists proxy IPs or CIDR ranges allowed to set
	// X-Forwarded-For. When empty, the client IP is the direct connection peer
	// and a forwarded header from an untrusted client is ignored.
	TrustedProxies []string `config:"trusted_proxies"`
}
