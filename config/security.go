package config

import "time"

// SecurityConfig defines security settings.
type SecurityConfig struct {
	TokenExpires     time.Duration `config:"token_expires"`
	RefreshNotBefore time.Duration `config:"refresh_not_before"`
	LoginRateLimit   int           `config:"login_rate_limit"`
	RefreshRateLimit int           `config:"refresh_rate_limit"`
}
