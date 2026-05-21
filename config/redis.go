package config

// RedisConfig defines Redis connection settings.
type RedisConfig struct {
	// Enabled gates the Redis client construction. When false (default) the
	// framework provides a nil *redis.Client into the fx graph and skips the
	// connection PING during start-up; downstream modules that ask for the
	// client via `optional:"true"` therefore stay dormant. Set this to true
	// only when the application actually depends on Redis (cache, redisstream
	// transport, nonce store, etc.).
	Enabled  bool   `config:"enabled"`
	Host     string `config:"host"`
	Port     uint16 `config:"port"`
	User     string `config:"user"`
	Password string `config:"password"`
	Database uint8  `config:"database"` // Database number (0-15)
	Network  string `config:"network"`  // "tcp" or "unix" (default: "tcp")
}
