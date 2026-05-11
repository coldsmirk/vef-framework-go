package store

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// Module wires the default bun-backed ClaimStore and DeleteQueue
// implementations into the fx graph, and exposes the high-level
// storage.Files facade composed over them.
var Module = fx.Module(
	"vef:storage:store",

	fx.Provide(NewClaimStore),
	fx.Provide(NewDeleteQueue),
	fx.Provide(storage.NewFiles),
)
