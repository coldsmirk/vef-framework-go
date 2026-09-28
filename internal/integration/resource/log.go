package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/result"
)

// LogSearch contains the search parameters for invocation logs.
type LogSearch struct {
	crud.Sortable

	SystemCode   string                `json:"systemCode" search:"eq,column=system_code"`
	ContractCode string                `json:"contractCode" search:"eq,column=contract_code"`
	Direction    integration.Direction `json:"direction" search:"eq,column=direction"`
	FailureKind  string                `json:"failureKind" search:"eq,column=failure_kind"`
	RequestID    string                `json:"requestId" search:"eq,column=request_id"`
}

// ReplayParams contains the parameters of a replay. Script may be unsaved
// editor content run in place of the saved adapter script.
type ReplayParams struct {
	api.P

	ID     string `json:"id" validate:"required"`
	Script string `json:"script"`
}

// LogResource exposes the invocation log: the page view for browsing, the
// single-record view for the full captures, and replay for re-running a
// recorded invocation against the current definitions.
type LogResource struct {
	api.Resource

	crud.FindPage[integration.InvocationLog, LogSearch]
	crud.FindOne[integration.InvocationLog, LogSearch]

	replayer *exec.Replayer
}

// NewLogResource creates the invocation log resource.
func NewLogResource(replayer *exec.Replayer) api.Resource {
	return &LogResource{
		Resource: api.NewRPCResource(
			"integration/log",
			api.WithOperations(
				api.OperationSpec{Action: "replay", RequiredPermission: "integration.log.replay", EnableAudit: true},
			),
		),
		FindPage: crud.NewFindPage[integration.InvocationLog, LogSearch]().
			RequiredPermission("integration.log.query"),
		FindOne: crud.NewFindOne[integration.InvocationLog, LogSearch]().
			RequiredPermission("integration.log.query"),
		replayer: replayer,
	}
}

// Replay re-runs a recorded invocation and returns its outcome shaped like the
// log entry, for side-by-side comparison. The wire calls an outbound replay
// makes are real; nothing is recorded to statistics or the invocation log.
func (r *LogResource) Replay(ctx fiber.Ctx, params ReplayParams) error {
	replay, err := r.replayer.Replay(ctx.Context(), params.ID, params.Script)
	if err != nil {
		return err
	}

	return result.Ok(replay).Response(ctx)
}
