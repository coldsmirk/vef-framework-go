package behavior

import (
	"go.uber.org/fx"
)

// Module provides all CQRS behavior middlewares for the approval module.
// Behaviors are aggregated via FX group `group:"vef:cqrs:behaviors"`; the
// CQRS bus sorts them by their Order() method so wrapping order is
// independent of FX's group-resolution timing.
//
// Order assignments (see cqrs.Ordered):
//
//   - Transaction  (Order 0)   wraps every inner behavior and the handler.
//   - ActionLog    (Order 100) persists audit rows after the handler succeeds.
//   - EventPublish (Order 200) emits events last, still inside the same tx.
var Module = fx.Module(
	"vef:approval:behavior",

	fx.Provide(
		fx.Annotate(
			NewTransactionBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
		fx.Annotate(
			NewActionLogBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
		fx.Annotate(
			NewEventPublishBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
	),
)
