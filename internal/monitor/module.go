package monitor

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/version"
)

var logger = log.Named("monitor")

// Module is the FX module for system monitoring functionality.
var Module = fx.Module(
	"vef:monitor",
	fx.Decorate(func(cfg *config.MonitorConfig) *config.MonitorConfig {
		cfgToUse := DefaultConfig()
		if cfg == nil {
			return &cfgToUse
		}

		if cfg.SampleInterval > 0 {
			cfgToUse.SampleInterval = cfg.SampleInterval
		}

		if cfg.SampleDuration > 0 {
			cfgToUse.SampleDuration = cfg.SampleDuration
		}

		return &cfgToUse
	}),
	fx.Decorate(
		fx.Annotate(
			func(buildInfo *monitor.BuildInfo) *monitor.BuildInfo {
				if buildInfo == nil {
					buildInfo = &monitor.BuildInfo{
						AppVersion: "v0.0.0",
						BuildTime:  "2022-08-08 01:00:00",
						GitCommit:  "-",
					}
				}

				buildInfo.VEFVersion = version.VEFVersion

				return buildInfo
			},
			fx.ParamTags(`optional:"true"`),
		),
	),
	fx.Provide(
		// Provide monitor service with lifecycle management
		fx.Annotate(
			NewService,
			fx.ParamTags(``, `optional:"true"`),
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
