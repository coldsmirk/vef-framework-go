package approval

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/approval/auth"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/resource"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
)

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	auth.Module,
	strategy.Module,
	behavior.Module,
	binding.Module,
	engine.Module,
	service.Module,
	command.Module,
	query.Module,
	resource.Module,
	timeout.Module,
	migration.Module,
)
