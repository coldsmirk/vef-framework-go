package config

// Environment variable keys.
const (
	EnvKeyPrefix    = "VEF"
	EnvNodeID       = EnvKeyPrefix + "_NODE_ID"       // XID node identifier
	EnvLogLevel     = EnvKeyPrefix + "_LOG_LEVEL"     // Log level (debug|info|warn|error)
	EnvConfigPath   = EnvKeyPrefix + "_CONFIG_PATH"   // Custom config file path
	EnvI18NLanguage = EnvKeyPrefix + "_I18N_LANGUAGE" // Override default language
)
