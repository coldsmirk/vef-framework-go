package service

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
)

// Module provides all approval services.
var Module = fx.Module(
	"vef:approval:service",

	fx.Provide(
		// Deploy validation accepts exactly the boot-registered vocabularies
		// — aggregate kinds from vef.ProvideApprovalAggregator, assignee and
		// CC kinds from the composite resolvers — so a host extension becomes
		// deployable automatically, with no second list to update.
		fx.Annotate(
			func(
				aggregators []approval.Aggregator,
				assignees *strategy.CompositeAssigneeResolver,
				ccs *strategy.CompositeCCResolver,
			) *FlowDefinitionService {
				kinds := make([]approval.AggregateKind, 0, len(aggregators))
				for _, aggregator := range aggregators {
					kinds = append(kinds, aggregator.Kind())
				}

				return NewFlowDefinitionService(
					WithAggregateKinds(kinds...),
					WithAssigneeKinds(assignees.Descriptors()...),
					WithCCKinds(ccs.Descriptors()...),
				)
			},
			fx.ParamTags(`group:"vef:approval:aggregators"`),
		),
		// The form-data cap is host policy, so it is resolved from
		// configuration here rather than baked into the constructors — a flow
		// whose detail tables carry thousands of rows raises it without a
		// framework rebuild.
		func(cfg *config.ApprovalConfig) *TaskService {
			return NewTaskService(WithFormDataMaxBytes(cfg.EffectiveFormDataMaxBytes()))
		},
		func(initiators *strategy.CompositeInitiatorResolver, cfg *config.ApprovalConfig) *ValidationService {
			return NewValidationService(initiators, WithFormDataMaxBytes(cfg.EffectiveFormDataMaxBytes()))
		},
		NewNodeService,
		NewInstanceService,
	),
)
