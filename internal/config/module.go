package config

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"vef:config",
	fx.Provide(
		newConfig,
		newAppConfig,
		newDataSourcesConfig,
		newCORSConfig,
		newSecurityConfig,
		newRedisConfig,
		newStorageConfig,
		newMonitorConfig,
		newMCPConfig,
		newApprovalConfig,
		newEventConfig,
	),
)
