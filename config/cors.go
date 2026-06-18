package config

// CORSConfig defines CORS middleware settings.
type CORSConfig struct {
	Enabled      bool     `config:"enabled"`
	AllowOrigins []string `config:"allow_origins"`
}
