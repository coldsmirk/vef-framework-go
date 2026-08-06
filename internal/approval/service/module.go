package service

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
)

// Module provides all approval services.
var Module = fx.Module(
	"vef:approval:service",

	fx.Provide(
		// Deploy validation accepts exactly the boot-registered aggregate
		// kinds, so host aggregators registered through
		// vef.ProvideApprovalAggregator become deployable automatically.
		fx.Annotate(
			func(aggregators []approval.Aggregator) *FlowDefinitionService {
				kinds := make([]approval.AggregateKind, 0, len(aggregators))
				for _, aggregator := range aggregators {
					kinds = append(kinds, aggregator.Kind())
				}

				return NewFlowDefinitionService(kinds...)
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
		func(assigneeSvc approval.AssigneeService, cfg *config.ApprovalConfig) *ValidationService {
			return NewValidationService(assigneeSvc, WithFormDataMaxBytes(cfg.EffectiveFormDataMaxBytes()))
		},
		NewNodeService,
		NewInstanceService,
	),
)
