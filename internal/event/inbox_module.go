package event

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	pubinbox "github.com/coldsmirk/vef-framework-go/event/inbox"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/event/inbox"
	internalmw "github.com/coldsmirk/vef-framework-go/internal/event/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var inboxLogger = logx.Named("event:inbox")

// InboxModule wires the sys_event_inbox repository, runs its migration,
// schedules retention cleanup, and contributes the Inbox consume
// middleware. The middleware only activates on transports that declare
// AtLeastOnce semantics so in-process delivery stays cheap.
var InboxModule = fx.Module(
	"vef:event:inbox",
	fx.Provide(
		fx.Annotate(
			newInboxRepository,
			fx.As(fx.Self()),
			fx.As(new(pubinbox.Repository)),
		),
		fx.Annotate(
			newInboxMiddleware,
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
			fx.As(new(pubmw.ConsumeMiddleware)),
		),
	),
	fx.Invoke(runInboxMigration),
	fx.Invoke(registerInboxCleanup),
)

func newInboxRepository(db orm.DB) *inbox.DefaultRepository {
	return inbox.NewRepository(db)
}

func newInboxMiddleware(cfg *config.EventConfig, repo pubinbox.Repository) pubmw.ConsumeMiddleware {
	if !cfg.Middleware.Inbox {
		return nil
	}

	return internalmw.NewInbox(repo)
}

func runInboxMigration(
	lc fx.Lifecycle,
	cfg *config.EventConfig,
	dsCfg *config.DataSourceConfig,
	db orm.DB,
) {
	if !cfg.Middleware.Inbox {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := inbox.Migrate(ctx, db, dsCfg.Kind); err != nil {
				return fmt.Errorf("inbox migration: %w", err)
			}

			return nil
		},
	})
}

func registerInboxCleanup(
	cfg *config.EventConfig,
	scheduler cron.Scheduler,
	repo pubinbox.Repository,
) error {
	if !cfg.Middleware.Inbox {
		return nil
	}

	cleaner := inbox.NewCleaner(repo, cfg.Inbox.EffectiveRetention(), inboxLogger)
	interval := cfg.Inbox.EffectiveCleanupInterval()

	job, err := scheduler.NewJob(cron.NewDurationJob(
		interval,
		cron.WithName("vef:event:inbox:cleanup"),
		cron.WithTags("vef", "event", "inbox"),
		cron.WithTask(cleaner.Cleanup),
	))
	if err != nil {
		return fmt.Errorf("register inbox cleanup job: %w", err)
	}

	inboxLogger.Infof("Inbox cleanup job [%s] registered, polling every %s", job.Name(), interval)

	return nil
}
