package monitor

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/monitor"
)

var logger = logx.Named("monitor")

// Module is the FX module for system monitoring functionality.
// NewService owns all config and build-info defaulting (including stamping the
// framework version), so both inputs are supplied optionally and need no decorator.
var Module = fx.Module(
	"vef:monitor",
	fx.Provide(
		// Provide monitor service with lifecycle management
		fx.Annotate(
			NewService,
			fx.ParamTags(`optional:"true"`, `optional:"true"`),
			fx.OnStart(func(ctx context.Context, svc monitor.Service) error {
				if initializer, ok := svc.(contract.Initializer); ok {
					if err := initializer.Init(ctx); err != nil {
						return fmt.Errorf("failed to initialize monitor service: %w", err)
					}
				}

				return nil
			}),
			fx.OnStop(func(svc monitor.Service) error {
				if closer, ok := svc.(io.Closer); ok {
					if err := closer.Close(); err != nil {
						return fmt.Errorf("failed to close monitor service: %w", err)
					}
				}

				return nil
			}),
		),
		// Provide monitor resource
		fx.Annotate(
			NewResource,
			fx.ResultTags(`group:"vef:api:resources"`),
		),
	),
)
