package strategy

import "go.uber.org/fx"

// Module provides built-in strategies with FX group extensibility.
var Module = fx.Module(
	"vef:approval:strategy",

	fx.Provide(
		// Pass rule strategies
		fx.Annotate(NewAllPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewOnePassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewRatioPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewOneRejectStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),

		// Assignee resolvers
		fx.Annotate(NewUserAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewRoleAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewDeptAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewSelfAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewSuperiorAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewDeptLeaderAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),
		fx.Annotate(NewFormFieldAssigneeResolver, fx.ResultTags(`group:"vef:approval:assignee_resolvers"`)),

		// Condition evaluators
		fx.Annotate(NewFieldConditionEvaluator, fx.ResultTags(`group:"vef:approval:condition_evaluators"`)),
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
)
