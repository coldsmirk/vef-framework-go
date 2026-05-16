package auth

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides identity-resolution defaults (tenant resolver). Hosts
// override the resolver via fx.Replace when their principal carries tenant
// info in a typed struct instead of a generic map.
var Module = fx.Module(
	"vef:approval:auth",

	fx.Provide(
		fx.Annotate(
			NewDefaultPrincipalTenantResolver,
			fx.As(new(approval.PrincipalTenantResolver)),
		),
	),
)
