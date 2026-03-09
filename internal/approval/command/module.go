package command

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// Module provides all command handlers and registers them with the Bus.
var Module = fx.Module(
	"vef:approval:command",

	fx.Provide(
		fx.Private,
		// Commands — Flow
		NewCreateFlowHandler,
		NewDeployFlowHandler,
		NewPublishVersionHandler,
		NewUpdateFlowHandler,
		NewToggleFlowActiveHandler,
		// Commands — Task processing
		NewApproveTaskHandler,
		NewRejectTaskHandler,
		NewTransferTaskHandler,
		NewRollbackTaskHandler,
		// Commands — Instance lifecycle
		NewStartInstanceHandler,
		NewWithdrawHandler,
		NewResubmitHandler,
		NewAddCCHandler,
		NewMarkCCReadHandler,
		NewAddAssigneeHandler,
		NewRemoveAssigneeHandler,
		NewUrgeTaskHandler,
		NewTerminateInstanceHandler,
		NewReassignTaskHandler,
	),

	fx.Invoke(registerHandlers),
)

//nolint:revive // FX dependency injection requires all handlers as parameters
func registerHandlers(
	bus cqrs.Bus,
	createFlow *CreateFlowHandler,
	deployFlow *DeployFlowHandler,
	publishVersion *PublishVersionHandler,
	updateFlow *UpdateFlowHandler,
	toggleFlowActive *ToggleFlowActiveHandler,
	approveTask *ApproveTaskHandler,
	rejectTask *RejectTaskHandler,
	transferTask *TransferTaskHandler,
	rollbackTask *RollbackTaskHandler,
	startInstance *StartInstanceHandler,
	withdraw *WithdrawHandler,
	resubmit *ResubmitHandler,
	addCC *AddCCHandler,
	markCCRead *MarkCCReadHandler,
	addAssignee *AddAssigneeHandler,
	removeAssignee *RemoveAssigneeHandler,
	urgeTask *UrgeTaskHandler,
	terminateInstance *TerminateInstanceHandler,
	reassignTask *ReassignTaskHandler,
) {
	// Commands — Flow
	cqrs.Register(bus, createFlow)
	cqrs.Register(bus, deployFlow)
	cqrs.Register(bus, publishVersion)
	cqrs.Register(bus, updateFlow)
	cqrs.Register(bus, toggleFlowActive)

	// Commands — Task processing
	cqrs.Register(bus, approveTask)
	cqrs.Register(bus, rejectTask)
	cqrs.Register(bus, transferTask)
	cqrs.Register(bus, rollbackTask)

	// Commands — Instance lifecycle
	cqrs.Register(bus, startInstance)
	cqrs.Register(bus, withdraw)
	cqrs.Register(bus, resubmit)
	cqrs.Register(bus, addCC)
	cqrs.Register(bus, markCCRead)
	cqrs.Register(bus, addAssignee)
	cqrs.Register(bus, removeAssignee)
	cqrs.Register(bus, urgeTask)
	cqrs.Register(bus, terminateInstance)
	cqrs.Register(bus, reassignTask)
}
