package config

// RedisConfig defines Redis connection settings.
type RedisConfig struct {
	Host     string `config:"host"`
	Port     uint16 `config:"port"`
	User     string `config:"user"`
	Password string `config:"password"`
	Database uint8  `config:"database"` // Database number (0-15)
	Network  string `config:"network"`  // "tcp" or "unix" (default: "tcp")
}
