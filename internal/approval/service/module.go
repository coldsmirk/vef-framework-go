package service

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
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
		NewTaskService,
		NewNodeService,
		NewValidationService,
		NewInstanceService,
	),
)
