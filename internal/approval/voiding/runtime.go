package voiding

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
)

type Runtime struct {
	fx.Out

	Registry  *business.Registry
	Hooks     *engine.LifecycleHookRunner
	Instances *service.InstanceService
	Core      *Core
}

// NewRuntime wires the request handler to the same lifecycle runner that invokes
// it. An approved request can then reverse its target inside the decision transaction.
func NewRuntime(hosts []approval.BusinessHandler, defs *service.FlowDefinitionService, projector engine.InstanceProjector, hooks []approval.InstanceLifecycleHook, bus event.Bus) (Runtime, error) {
	request := &RequestHandler{}

	registry, err := business.NewRegistry(hosts, defs.ValidateFormFields, request)
	if err != nil {
		return Runtime{}, err
	}

	runner := engine.NewLifecycleHookRunner(projector, hooks, registry)
	instances := service.NewInstanceService(runner)
	request.core = NewCore(instances, registry, bus)

	return Runtime{Registry: registry, Hooks: runner, Instances: instances, Core: request.core}, nil
}
