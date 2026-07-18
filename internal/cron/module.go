package cron

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store"
)

// Module provides dependency injection configuration for the cron scheduler:
// the in-memory gocron scheduler for process-local ticks, and the durable
// schedule store (vef.cron.store) for persisted, cluster-single-fire jobs.
var Module = fx.Module(
	"vef:cron",
	fx.Provide(newScheduler, fx.Private),
	fx.Provide(cron.NewScheduler),
	store.Module,
)
