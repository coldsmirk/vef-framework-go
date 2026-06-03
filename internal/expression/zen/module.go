package zen

import (
	"go.uber.org/fx"

	internalexpression "github.com/coldsmirk/vef-framework-go/internal/expression"
)

// Module provides the Zen-backed expression.Engine together with the
// backend-agnostic integrations (API handler parameter resolver and mold field
// transformer). It is re-exported through the public expression/zen package.
var Module = fx.Module(
	"vef:expression:zen",
	fx.Provide(New),
	internalexpression.Module,
)
