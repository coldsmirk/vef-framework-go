package storage

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/storage/worker"
	"github.com/coldsmirk/vef-framework-go/storage"
)

var logger = logx.Named("storage")

// ErrEventRouteNotTransactional indicates the framework's event bus is
// not configured to deliver a storage domain event through a
// transactional transport. Storage publishes with event.WithTx; without
// such a route the first publish would fail at runtime, so the module
// fails fast at start-up instead. The wrapped formatted error names the
// offending event type and points operators at the configuration that
// must be set.
var ErrEventRouteNotTransactional = errors.New("storage: event must route to a transactional transport")

var Module = fx.Module(
	"vef:storage",
	fx.Provide(
		fx.Annotate(
			NewService,
			fx.OnStart(func(ctx context.Context, service storage.Service) error {
				if initializer, ok := service.(contract.Initializer); ok {
					if err := initializer.Init(ctx); err != nil {
						return fmt.Errorf("failed to initialize storage service: %w", err)
					}
				}

				return nil
			}),
		),
		fx.Annotate(
			NewResource,
			fx.ResultTags(`group:"vef:api:resources"`),
		),
		fx.Annotate(
			NewProxyMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		// Default FileACL: pub-only reads, all listing denied. Business
		// modules override via vef.SupplyFileACL when they need to grant
		// access to private keys based on their own ownership / ACL data.
		newDefaultFileACL,
		// Default URLKeyMapper: identity. Business modules override via
		// vef.SupplyURLKeyMapper when they embed proxy / CDN URLs in
		// richtext / markdown fields and need them translated back to
		// storage keys during reconciliation.
		newDefaultURLKeyMapper,
	),

	migration.Module,
	store.Module,
	worker.Module,

	fx.Invoke(verifyEventRouting),
)

// verifyEventRouting fails fast at start-up when the framework's event
// bus is not configured to deliver storage events via a transactional
// transport. Storage publishes every domain event with event.WithTx so
// that consumers see the event iff the originating business transaction
// commits; without a transactional route the first publish would fail
// at runtime with ErrTxRequired.
//
// The check itself is deferred to OnStart so the bus has built its
// router by the time we query it (bus.Start runs first in the lifecycle
// order — see bootstrap module ordering).
func verifyEventRouting(lc fx.Lifecycle, inspector event.RouteInspector) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			required := []string{
				storage.EventTypeFileClaimed,
				storage.EventTypeFileDeleted,
				storage.EventTypeDeleteDeadLetter,
			}

			for _, et := range required {
				if !inspector.HasTransactionalRoute(et) {
					return fmt.Errorf(
						"%w: %q (enable vef.event.transports.outbox.enabled=true and add a "+
							"routing rule for pattern \"vef.storage.*\" → [\"outbox\"], "+
							"or set vef.event.default_transport=\"outbox\")",
						ErrEventRouteNotTransactional, et)
				}
			}

			return nil
		},
	})
}

// newDefaultFileACL is a constructor (not a literal supply) so business
// code can replace it through fx.Decorate or fx.Replace via the
// vef.SupplyFileACL helper.
func newDefaultFileACL() storage.FileACL {
	return new(storage.DefaultFileACL)
}

// newDefaultURLKeyMapper is a constructor (not a literal supply) so
// business code can replace it through fx.Decorate via the
// vef.SupplyURLKeyMapper helper.
func newDefaultURLKeyMapper() storage.URLKeyMapper {
	return storage.ProxyURLKeyMapper{}
}
