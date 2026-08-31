package strategy

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides built-in strategies with FX group extensibility.
//
// The three principal vocabularies — assignee, CC, initiator — are built here
// rather than collected from a group, so that host registrations can be
// overlaid deterministically: a host resolver replaces the built-in of the same
// kind in place, and anything else is appended. Collecting built-ins and host
// entries into one group would make "which one wins" depend on fx's group
// ordering, which is not a contract.
var Module = fx.Module(
	"vef:approval:strategy",

	fx.Provide(
		// Pass rule strategies
		fx.Annotate(NewAllPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewAnyPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),
		fx.Annotate(NewRatioPassStrategy, fx.ResultTags(`group:"vef:approval:pass_rule_strategies"`)),

		// Principal resolvers: framework built-ins overlaid with the host
		// registrations from vef.ProvideApprovalAssigneeResolver and friends.
		fx.Annotate(
			newCompositeAssigneeResolver,
			fx.ParamTags(``, `group:"vef:approval:assignee_resolvers"`),
		),
		fx.Annotate(
			newCompositeCCResolver,
			fx.ParamTags(``, `group:"vef:approval:cc_resolvers"`),
		),
		fx.Annotate(
			newCompositeInitiatorResolver,
			fx.ParamTags(``, `group:"vef:approval:initiator_resolvers"`),
		),

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

// BuiltinAssigneeResolvers returns the framework's own assignee resolvers in
// designer order — the base every host registration is overlaid onto.
func BuiltinAssigneeResolvers(svc approval.AssigneeService) []approval.AssigneeResolver {
	return []approval.AssigneeResolver{
		NewUserAssigneeResolver(),
		NewRoleAssigneeResolver(svc),
		NewDepartmentAssigneeResolver(svc),
		NewSelfAssigneeResolver(),
		NewSuperiorAssigneeResolver(svc),
		NewDepartmentLeaderAssigneeResolver(svc),
		NewFormFieldAssigneeResolver(),
	}
}

// BuiltinCCResolvers returns the framework's own CC resolvers in designer order.
func BuiltinCCResolvers(svc approval.AssigneeService) []approval.CCResolver {
	return []approval.CCResolver{
		NewUserCCResolver(),
		NewRoleCCResolver(svc),
		NewDepartmentCCResolver(svc),
		NewFormFieldCCResolver(),
	}
}

// BuiltinInitiatorResolvers returns the framework's own initiator resolvers in
// designer order.
func BuiltinInitiatorResolvers(svc approval.AssigneeService) []approval.InitiatorResolver {
	return []approval.InitiatorResolver{
		NewUserInitiatorResolver(),
		NewRoleInitiatorResolver(svc),
		NewDepartmentInitiatorResolver(),
	}
}

// BuiltinAssigneeKinds and BuiltinCCKinds return the descriptors of the
// framework's own vocabularies — the default set deploy validation falls back
// to when no composite is injected (a test constructing the service bare).
func BuiltinAssigneeKinds() []approval.KindDescriptor[approval.AssigneeKind] {
	return describeAll(BuiltinAssigneeResolvers(nil))
}

func BuiltinCCKinds() []approval.KindDescriptor[approval.CCKind] {
	return describeAll(BuiltinCCResolvers(nil))
}

func newCompositeAssigneeResolver(svc approval.AssigneeService, hosts []approval.AssigneeResolver) (*CompositeAssigneeResolver, error) {
	return NewCompositeAssigneeResolver(BuiltinAssigneeResolvers(svc), hosts)
}

func newCompositeCCResolver(svc approval.AssigneeService, hosts []approval.CCResolver) (*CompositeCCResolver, error) {
	return NewCompositeCCResolver(BuiltinCCResolvers(svc), hosts)
}

func newCompositeInitiatorResolver(svc approval.AssigneeService, hosts []approval.InitiatorResolver) (*CompositeInitiatorResolver, error) {
	return NewCompositeInitiatorResolver(BuiltinInitiatorResolvers(svc), hosts)
}
