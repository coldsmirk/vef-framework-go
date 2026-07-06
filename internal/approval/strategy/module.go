package strategy

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides built-in strategies with FX group extensibility.
var Module = fx.Module(
	"vef:approval:strategy",

	fx.Provide(
		// Pass rule strategies
		fx.Annotate(NewAllPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewAnyPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewRatioPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),

		// Assignee resolvers
		fx.Annotate(NewUserAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewRoleAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewDepartmentAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewSelfAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewSuperiorAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewDepartmentLeaderAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewFormFieldAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),

		// Aggregators (detail-table folds for aggregate field conditions)
		fx.Annotate(NewSumAggregator, fx.ResultTags(`group:"vef:approval:aggregators"`)),
		fx.Annotate(NewCountAggregator, fx.ResultTags(`group:"vef:approval:aggregators"`)),
		fx.Annotate(NewAvgAggregator, fx.ResultTags(`group:"vef:approval:aggregators"`)),

		// Condition evaluators
		fx.Annotate(
			func(aggregators []approval.Aggregator) approval.ConditionEvaluator {
				return NewFieldConditionEvaluator(aggregators...)
			},
			fx.ParamTags(`group:"vef:approval:aggregators"`),
			fx.ResultTags(`group:"vef:approval:condition_evaluators"`),
		),
		fx.Annotate(NewExpressionConditionEvaluator, fx.ResultTags(`group:"vef:approval:condition_evaluators"`)),

		// Strategy registry
		fx.Annotate(
			NewStrategyRegistry,
			fx.ParamTags(
				`group:"vef:approval:pass_rule_strategies"`,
				`group:"vef:approval:assignee_resolvers"`,
				`group:"vef:approval:condition_evaluators"`,
			),
		),
	),

	fx.Invoke(func(r *StrategyRegistry) error { return r.ValidateBuiltins() }),
	fx.Invoke(fx.Annotate(
		func(aggregators []approval.Aggregator) error { return ValidateBuiltinAggregators(aggregators) },
		fx.ParamTags(`group:"vef:approval:aggregators"`),
	)),
)
