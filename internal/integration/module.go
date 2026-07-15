package integration

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/internal/integration/migration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/resource"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
)

// Module is the integration engine module: contract/system/adapter/route
// definitions, the script-executing Invoker, the management API, and the
// invocation statistics the monitor module reads.
var Module = fx.Module(
	"vef:integration",

	fx.Provide(
		fx.Annotate(auth.NewRegistry, fx.ParamTags(`group:"vef:integration:auth_schemes"`)),
		service.NewSecretCodec,
		exec.NewTableRouteResolver,
		fx.Annotate(
			exec.NewInvoker,
			fx.As(fx.Self()),
			fx.As(new(integration.Invoker)),
			fx.As(new(integration.StatsInspector)),
		),
	),

	resource.Module,
	migration.Module,
)
