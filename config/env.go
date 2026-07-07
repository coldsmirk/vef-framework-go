package config

// Environment variable keys.
const (
	EnvPrefix       = "VEF"
	EnvLogLevel     = EnvPrefix + "_LOG_LEVEL"     // Log level (debug|info|warn|error)
	EnvConfigPath   = EnvPrefix + "_CONFIG_PATH"   // Custom config file path
	EnvI18NLanguage = EnvPrefix + "_I18N_LANGUAGE" // Override default language
)
