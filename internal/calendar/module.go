package calendar

import (
	"context"
	"embed"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// Module provides the calendar service, default data source and RPC operations.
var Module = fx.Module("vef:calendar",
	fx.Provide(newStore, fx.Private),
	fx.Provide(func(s *store) calendar.DayLoader { return s }, NewService),
	fx.Provide(fx.Annotate(newResource, fx.ResultTags(`group:"vef:api:resources"`))),
	fx.Invoke(registerMigration),
)

func registerMigration(lc fx.Lifecycle, cfg *config.CalendarConfig, sources *config.DataSourcesConfig, db orm.DB) {
	if cfg.Store.Enabled && cfg.Store.AutoMigrate {
		lc.Append(fx.StartHook(func(ctx context.Context) error { return migrate(ctx, db, sources.Primary().Kind) }))
	}
}

func migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{Label: "calendar", Kind: kind, Scripts: scripts, ExpectedTables: []string{"sys_calendar_override"}})
}
