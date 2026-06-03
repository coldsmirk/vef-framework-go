package config

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
)

// unmarshalConfig is a generic helper that unmarshals configuration from a given key.
func unmarshalConfig[T any](cfg config.Config, key string, target *T) (*T, error) {
	if err := cfg.Unmarshal(key, target); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s config: %w", key, err)
	}

	return target, nil
}

func newAppConfig(cfg config.Config) (*config.AppConfig, error) {
	return unmarshalConfig(cfg, "vef.app", new(config.AppConfig))
}

func newCorsConfig(cfg config.Config) (*config.CorsConfig, error) {
	return unmarshalConfig(cfg, "vef.cors", new(config.CorsConfig))
}

func newSecurityConfig(cfg config.Config) (*config.SecurityConfig, error) {
	return unmarshalConfig(cfg, "vef.security", new(config.SecurityConfig))
}

func newRedisConfig(cfg config.Config) (*config.RedisConfig, error) {
	return unmarshalConfig(cfg, "vef.redis", new(config.RedisConfig))
}

func newStorageConfig(cfg config.Config) (*config.StorageConfig, error) {
	return unmarshalConfig(cfg, "vef.storage", new(config.StorageConfig))
}

func newMonitorConfig(cfg config.Config) (*config.MonitorConfig, error) {
	return unmarshalConfig(cfg, "vef.monitor", new(config.MonitorConfig))
}

func newMCPConfig(cfg config.Config) (*config.MCPConfig, error) {
	return unmarshalConfig(cfg, "vef.mcp", new(config.MCPConfig))
}

func newApprovalConfig(cfg config.Config) (*config.ApprovalConfig, error) {
	approvalConfig, err := unmarshalConfig(cfg, "vef.approval", new(config.ApprovalConfig))
	if err != nil {
		return nil, err
	}

	approvalConfig.ApplyDefaults()

	return approvalConfig, nil
}

func newEventConfig(cfg config.Config) (*config.EventConfig, error) {
	return unmarshalConfig(cfg, "vef.event", new(config.EventConfig))
}
