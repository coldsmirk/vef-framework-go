package facade

import "go.uber.org/fx"

// Module exposes approval.Service — the programmatic control surface — in DI.
// The API resources consume it as well, so it is provided publicly rather than
// fx.Private.
var Module = fx.Module(
	"vef:approval:facade",

	fx.Provide(NewService),
)
