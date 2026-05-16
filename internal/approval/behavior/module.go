package behavior

import (
	"go.uber.org/fx"
)

// Module provides all CQRS behavior middlewares for the approval module.
// Behaviors are aggregated via FX group `group:"vef:cqrs:behaviors"`; the
// CQRS bus wraps them outside-in so the slot order at registration is the
// execution order.
//
// Transaction wraps the entire pipeline so every inner behavior and the
// final handler share a single DB transaction. EventPublish runs inside
// the transaction so PublishBatch can enroll via event.WithTx(db).
var Module = fx.Module(
	"vef:approval:behavior",

	fx.Provide(
		fx.Annotate(
			NewTransactionBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
		fx.Annotate(
			NewEventPublishBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
	),
)
