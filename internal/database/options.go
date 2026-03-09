package database

import (
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlguard"
	"github.com/coldsmirk/vef-framework-go/log"
)

type databaseOptions struct {
	Config          *config.DataSourceConfig
	PoolConfig      *ConnectionPoolConfig
	EnableQueryHook bool
	Logger          log.Logger
	BunOptions      []bun.DBOption
	SQLGuardConfig  *sqlguard.Config
}

type Option func(*databaseOptions)

func newDefaultOptions(cfg *config.DataSourceConfig) *databaseOptions {
	var guardConfig *sqlguard.Config
	if cfg.EnableSQLGuard {
		guardConfig = sqlguard.DefaultConfig()
	}

	return &databaseOptions{
		Config:          cfg,
		PoolConfig:      NewDefaultConnectionPoolConfig(),
		EnableQueryHook: true,
		Logger:          logger,
		BunOptions:      []bun.DBOption{bun.WithDiscardUnknownColumns()},
		SQLGuardConfig:  guardConfig,
	}
}

func WithConnectionPool(poolConfig *ConnectionPoolConfig) Option {
	return func(opts *databaseOptions) {
		opts.PoolConfig = poolConfig
	}
}

// DisableQueryHook disables query logging which is enabled by default.
func DisableQueryHook() Option {
	return func(opts *databaseOptions) {
		opts.EnableQueryHook = false
	}
}

func WithLogger(logger log.Logger) Option {
	return func(opts *databaseOptions) {
		opts.Logger = logger
	}
}

func WithBunOptions(bunOpts ...bun.DBOption) Option {
	return func(opts *databaseOptions) {
		opts.BunOptions = append(opts.BunOptions, bunOpts...)
	}
}

// WithSQLGuardConfig sets a custom sql guard configuration.
func WithSQLGuardConfig(cfg *sqlguard.Config) Option {
	return func(opts *databaseOptions) {
		opts.SQLGuardConfig = cfg
	}
}

// DisableSQLGuard disables the sql guard.
func DisableSQLGuard() Option {
	return func(opts *databaseOptions) {
		opts.SQLGuardConfig = nil
	}
}

func (opts *databaseOptions) apply(options ...Option) {
	for _, opt := range options {
		opt(opts)
	}
}
