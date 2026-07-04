package auth

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides identity-resolution defaults (tenant resolver, instance
// globals resolver). Hosts override a resolver via fx.Replace — the tenant
// resolver when their principal carries tenant info in a typed struct instead
// of a generic map, the globals resolver when their flows route on
// host-defined global variables.
var Module = fx.Module(
	"vef:approval:auth",

	fx.Provide(
		fx.Annotate(
			NewDefaultPrincipalTenantResolver,
			fx.As(new(approval.PrincipalTenantResolver)),
		),
		fx.Annotate(
			NewDefaultInstanceGlobalsResolver,
			fx.As(new(approval.InstanceGlobalsResolver)),
		),
	),
)
