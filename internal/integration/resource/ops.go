package resource

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// DryRunParams contains the parameters of a dry run. Script may be unsaved
// editor content; empty falls back to the saved adapter script.
type DryRunParams struct {
	api.P

	SystemCode   string          `json:"systemCode" validate:"required"`
	ContractCode string          `json:"contractCode" validate:"required"`
	Script       string          `json:"script"`
	Input        json.RawMessage `json:"input"`
}

// DryRunInboundParams contains the parameters of an inbound dry run. Script
// may be unsaved editor content (empty falls back to the saved inbound
// adapter script); HandlerOutput is the sample the stubbed business handler
// returns.
type DryRunInboundParams struct {
	api.P

	SystemCode    string               `json:"systemCode" validate:"required"`
	ContractCode  string               `json:"contractCode" validate:"required"`
	Script        string               `json:"script"`
	Request       InboundRequestParams `json:"request"`
	HandlerOutput json.RawMessage      `json:"handlerOutput"`
}

// InboundRequestParams is the synthetic external request of an inbound dry run.
type InboundRequestParams struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    string            `json:"body"`
}

// TestConnectionParams contains the parameters of a connection probe against
// a saved system.
type TestConnectionParams struct {
	api.P

	SystemCode string `json:"systemCode" validate:"required"`
	Method     string `json:"method"`
	Path       string `json:"path"`
}

// OpsResource hosts the operational endpoints of the integration engine: the
// script test consoles (dry_run, dry_run_inbound), the connection probe
// (test_connection), and the routing diagnosis (diagnose_routes). Dry run
// and probing operate on disabled definitions too — testing precedes
// enabling.
type OpsResource struct {
	api.Resource

	invoker  *exec.Invoker
	receiver *exec.Receiver
}

// NewOpsResource creates the operational resource.
func NewOpsResource(invoker *exec.Invoker, receiver *exec.Receiver) api.Resource {
	return &OpsResource{
		invoker:  invoker,
		receiver: receiver,
		Resource: api.NewRPCResource(
			"integration/ops",
			api.WithOperations(
				api.OperationSpec{Action: "dry_run", RequiredPermission: "integration.ops.dry_run"},
				api.OperationSpec{Action: "dry_run_inbound", RequiredPermission: "integration.ops.dry_run_inbound"},
				api.OperationSpec{Action: "test_connection", RequiredPermission: "integration.ops.test_connection"},
				api.OperationSpec{Action: "diagnose_routes", RequiredPermission: "integration.ops.diagnose_routes"},
			),
		),
	}
}

// DryRun executes a script against a system under a contract and returns the
// output, the failure classification, and the full wire trace. The calls it
// makes are real; nothing is recorded to statistics or the invocation log.
func (r *OpsResource) DryRun(ctx fiber.Ctx, db orm.DB, params DryRunParams) error {
	contract, system, script, err := dryRunTarget(ctx.Context(), db, params.SystemCode, params.ContractCode, params.Script, integration.DirectionOutbound)
	if err != nil {
		return err
	}

	var input any
	if len(params.Input) > 0 {
		if err := json.Unmarshal(params.Input, &input); err != nil {
			return integration.ErrInputInvalid(err.Error())
		}
	}

	return result.Ok(r.invoker.DryRun(ctx.Context(), contract, system, script, input)).Response(ctx)
}

// DryRunInbound executes an inbound script against a synthetic external request
// with the business handler stubbed to return the supplied sample output.
// Nothing runs against business code and nothing is recorded; verification is
// bypassed — the console tests translation, not credentials.
func (r *OpsResource) DryRunInbound(ctx fiber.Ctx, db orm.DB, params DryRunInboundParams) error {
	contract, system, script, err := dryRunTarget(ctx.Context(), db, params.SystemCode, params.ContractCode, params.Script, integration.DirectionInbound)
	if err != nil {
		return err
	}

	var handlerOutput any
	if len(params.HandlerOutput) > 0 {
		if err := json.Unmarshal(params.HandlerOutput, &handlerOutput); err != nil {
			return integration.ErrOutputInvalid(err.Error())
		}
	}

	req := &integration.InboundRequest{
		SystemCode:   system.Code,
		ContractCode: contract.Code,
		Protocol:     "http",
		Method:       params.Request.Method,
		Path:         params.Request.Path,
		Headers:      lowercaseKeys(params.Request.Headers),
		Query:        params.Request.Query,
		Body:         []byte(params.Request.Body),
	}

	return result.Ok(r.receiver.DryRun(ctx.Context(), contract, system, script, req, handlerOutput)).Response(ctx)
}

// lowercaseKeys normalizes the synthetic request headers to the envelope
// contract (lowercased names, as an HTTP gateway would deliver them).
func lowercaseKeys(values map[string]string) map[string]string {
	normalized := make(map[string]string, len(values))
	for name, value := range values {
		normalized[strings.ToLower(name)] = value
	}

	return normalized
}

// DiagnoseRoutes reports the routing table's configuration gaps — dangling
// adapters, disabled targets, uncovered contracts — before they surface as
// runtime errors.
func (*OpsResource) DiagnoseRoutes(ctx fiber.Ctx, db orm.DB) error {
	report, err := definition.DiagnoseRoutes(ctx.Context(), db)
	if err != nil {
		return err
	}

	return result.Ok(report).Response(ctx)
}

// TestConnection probes a saved system with a single request.
func (r *OpsResource) TestConnection(ctx fiber.Ctx, db orm.DB, params TestConnectionParams) error {
	system, err := findByCode[integration.System](ctx.Context(), db, params.SystemCode, integration.ErrSystemNotFound)
	if err != nil {
		return err
	}

	check, err := r.invoker.TestConnection(ctx.Context(), system, params.Method, params.Path)
	if err != nil {
		return err
	}

	return result.Ok(check).Response(ctx)
}

// dryRunTarget loads the contract and system of a dry run by code, falling
// back to the saved adapter script of the given direction when the caller
// submitted none. Disabled definitions are intentionally accepted — testing
// precedes enabling.
func dryRunTarget(ctx context.Context, db orm.DB, systemCode, contractCode, script string, direction integration.Direction) (*integration.Contract, *integration.System, string, error) {
	contract, err := findByCode[integration.Contract](ctx, db, contractCode, integration.ErrContractNotFound)
	if err != nil {
		return nil, nil, "", err
	}

	system, err := findByCode[integration.System](ctx, db, systemCode, integration.ErrSystemNotFound)
	if err != nil {
		return nil, nil, "", err
	}

	if script == "" {
		adapter, err := definition.FindOne[integration.Adapter](ctx, db, integration.ErrAdapterNotFound,
			func(cb orm.ConditionBuilder) {
				cb.Equals("system_id", system.ID).
					Equals("contract_id", contract.ID).
					Equals("direction", direction)
			})
		if err != nil {
			return nil, nil, "", err
		}

		script = adapter.Script
	}

	return contract, system, script, nil
}

// findByCode loads a definition by its unique code, mapping a missing row to
// notFound. Disabled definitions are intentionally returned.
func findByCode[T any](ctx context.Context, db orm.DB, code string, notFound error) (*T, error) {
	return definition.FindOne[T](ctx, db, notFound, func(cb orm.ConditionBuilder) {
		cb.Equals("code", code)
	})
}
