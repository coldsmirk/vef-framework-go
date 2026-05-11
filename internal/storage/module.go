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
	),

	migration.Module,
	store.Module,
	worker.Module,
)
