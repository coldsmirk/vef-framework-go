package database

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var (
	logger = log.Named("database")
	Module = fx.Module(
		"vef:database",
		fx.Provide(
			fx.Annotate(
				func(lc fx.Lifecycle, cfg *config.DataSourceConfig) (db *bun.DB, err error) {
					if db, err = New(cfg); err != nil {
						return db, err
					}

					provider, exists := registry.provider(cfg.Kind)
					if !exists {
						return nil, newUnsupportedDBKindError(cfg.Kind)
					}

					lc.Append(
						fx.StartStopHook(
							func(ctx context.Context) error {
								if err := db.PingContext(ctx); err != nil {
									return wrapPingError(provider.Kind(), err)
								}

								if err := logDBVersion(provider, db, logger); err != nil {
									return err
								}

								logger.Infof("Database client started successfully: %s", provider.Kind())

								return nil
							},
							func() error {
								logger.Info("Closing database connection...")

								return db.Close()
							},
						),
					)

					return db, err
				},
				fx.As(new(bun.IDB)),
				fx.As(fx.Self()),
			),
			func(db *bun.DB) *sql.DB {
				return db.DB
			},
		),
	)
)
