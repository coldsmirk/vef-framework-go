package config

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
