package sequence

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/sequence"
)

// Module provides the sequence generation functionality for the VEF framework.
//
// The in-memory store is exposed both as its concrete *sequence.MemoryStore
// and as the sequence.Store interface the Generator depends on, so applications
// can inject the concrete store and seed rules via MemoryStore.Register during
// their own start-up hook.
var Module = fx.Module(
	"vef:sequence",
	fx.Provide(
		fx.Annotate(
			sequence.NewMemoryStore,
			fx.As(fx.Self()),
			fx.As(new(sequence.Store)),
		),
		NewGenerator,
	),
)
