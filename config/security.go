package config

import "time"

// SecurityConfig defines security settings.
type SecurityConfig struct {
	// Secret is the hex-encoded key used to sign and verify JWT tokens.
	// Leave empty in development to have the framework generate an ephemeral
	// key at startup; set a stable per-deployment value in production.
	Secret           string        `config:"secret"`
	TokenExpires     time.Duration `config:"token_expires"`
	RefreshNotBefore time.Duration `config:"refresh_not_before"`
	LoginRateLimit   int           `config:"login_rate_limit"`
	RefreshRateLimit int           `config:"refresh_rate_limit"`
}
