package storage

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/storage/worker"
	"github.com/coldsmirk/vef-framework-go/storage"
)

var logger = logx.Named("storage")

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
)

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
