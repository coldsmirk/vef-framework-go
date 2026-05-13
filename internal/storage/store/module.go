package store

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// Module wires the default bun-backed ClaimStore and DeleteQueue
// implementations into the fx graph, and exposes the high-level
// storage.Files facade composed over them.
//
// Each store constructor is registered twice through fx.As: once under
// the framework-internal interface (consumed by storage_resource and
// the worker), once under the minimal public interface (consumed by
// storage.NewFiles and any business code that takes the facade as a
// dependency). Both registrations resolve to the same underlying
// instance.
var Module = fx.Module(
	"vef:storage:store",

	fx.Provide(
		fx.Annotate(
			NewClaimStore,
			fx.As(fx.Self()),
			fx.As(new(storage.ClaimConsumer)),
		),
		fx.Annotate(
			NewDeleteQueue,
			fx.As(fx.Self()),
			fx.As(new(storage.DeleteScheduler)),
		),
		NewUploadPartStore,
	),
	fx.Provide(storage.NewFiles),
)
