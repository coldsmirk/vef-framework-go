package query

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// Module provides all query handlers and registers them with the Bus.
var Module = fx.Module(
	"vef:approval:query",

	fx.Provide(
		fx.Private,
		NewGetFlowGraphHandler,
		NewFindMyInitiatedHandler,
		NewFindMyPendingTasksHandler,
		NewFindMyCompletedTasksHandler,
		NewFindMyCCRecordsHandler,
		NewGetMyPendingCountsHandler,
		NewGetMyInstanceDetailHandler,
		NewFindAvailableFlowsHandler,
		NewFindAdminInstancesHandler,
		NewFindAdminTasksHandler,
		NewGetAdminInstanceDetailHandler,
		NewFindAdminActionLogsHandler,
		NewFindFlowsHandler,
		NewFindFlowVersionsHandler,
	),

	fx.Invoke(registerHandlers),
)

//nolint:revive // FX dependency injection requires all handlers as parameters
func registerHandlers(
	bus cqrs.Bus,
	getFlowGraph *GetFlowGraphHandler,
	findMyInitiated *FindMyInitiatedHandler,
	findMyPendingTasks *FindMyPendingTasksHandler,
	findMyCompletedTasks *FindMyCompletedTasksHandler,
	findMyCCRecords *FindMyCCRecordsHandler,
	getMyPendingCounts *GetMyPendingCountsHandler,
	getMyInstanceDetail *GetMyInstanceDetailHandler,
	findAvailableFlows *FindAvailableFlowsHandler,
	findAdminInstances *FindAdminInstancesHandler,
	findAdminTasks *FindAdminTasksHandler,
	getAdminInstanceDetail *GetAdminInstanceDetailHandler,
	findAdminActionLogs *FindAdminActionLogsHandler,
	findFlows *FindFlowsHandler,
	findFlowVersions *FindFlowVersionsHandler,
) {
	cqrs.Register(bus, getFlowGraph)
	cqrs.Register(bus, findMyInitiated)
	cqrs.Register(bus, findMyPendingTasks)
	cqrs.Register(bus, findMyCompletedTasks)
	cqrs.Register(bus, findMyCCRecords)
	cqrs.Register(bus, getMyPendingCounts)
	cqrs.Register(bus, getMyInstanceDetail)
	cqrs.Register(bus, findAvailableFlows)
	cqrs.Register(bus, findAdminInstances)
	cqrs.Register(bus, findAdminTasks)
	cqrs.Register(bus, getAdminInstanceDetail)
	cqrs.Register(bus, findAdminActionLogs)
	cqrs.Register(bus, findFlows)
	cqrs.Register(bus, findFlowVersions)
}
