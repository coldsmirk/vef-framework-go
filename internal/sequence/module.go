package sequence

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/sequence"
)

// Module provides the sequence generation functionality for the VEF framework.
var Module = fx.Module(
	"vef:sequence",
	fx.Provide(
		sequence.NewMemoryStore,
		NewGenerator,
	),
)
