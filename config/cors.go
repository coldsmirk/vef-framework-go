package config

// CorsConfig defines CORS middleware settings.
type CorsConfig struct {
	Enabled      bool     `config:"enabled"`
	AllowOrigins []string `config:"allow_origins"`
}
