package integration

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/internal/integration/migration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/resource"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/internal/integration/worker"
)

// Module is the integration engine module: contract/system/adapter/route
// definitions, the script-executing Invoker (outbound) and Receiver
// (inbound), the management API, and the invocation statistics the monitor
// module reads.
var Module = fx.Module(
	"vef:integration",

	fx.Provide(
		fx.Annotate(auth.NewRegistry, fx.ParamTags(`group:"vef:integration:auth_schemes"`)),
		fx.Annotate(auth.NewInboundRegistry, fx.ParamTags(``, `group:"vef:integration:inbound_auth_schemes"`)),
		service.NewSecretCodec,
		exec.NewTableRouteResolver,
		fx.Annotate(
			exec.NewInvoker,
			fx.As(fx.Self()),
			fx.As(new(integration.Invoker)),
			fx.As(new(integration.StatsInspector)),
		),
		fx.Annotate(
			exec.NewReceiver,
			fx.ParamTags(``, ``, ``, `group:"vef:integration:inbound_handlers"`),
		),
	),

	resource.Module,
	migration.Module,
	worker.Module,
)
