package zen

import (
	"go.uber.org/fx"

	internalexpression "github.com/coldsmirk/vef-framework-go/internal/expression"
)

// Module enables the Zen-backed expression engine. Import this package and add
// Module to vef.Run(...) to opt in. Doing so introduces a CGO dependency
// (zen-go), so applications that do not need expressions stay pure Go.
//
// It provides the expression.Engine plus the backend-agnostic integrations
// (API handler parameter resolver and mold field transformer).
var Module = fx.Module(
	"vef:expression:zen",
	fx.Provide(New),
	internalexpression.Module,
)
