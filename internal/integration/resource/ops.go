package resource

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/integration"
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

// TestConnectionParams contains the parameters of a connection probe against
// a saved system.
type TestConnectionParams struct {
	api.P

	SystemCode string `json:"systemCode" validate:"required"`
	Method     string `json:"method"`
	Path       string `json:"path"`
}

// OpsResource hosts the operational endpoints of the integration engine:
// the script test console (dry_run) and the connection probe
// (test_connection). Both operate on disabled definitions too — testing
// precedes enabling.
type OpsResource struct {
	api.Resource

	invoker *exec.Invoker
}

// NewOpsResource creates the operational resource.
func NewOpsResource(invoker *exec.Invoker) api.Resource {
	return &OpsResource{
		invoker: invoker,
		Resource: api.NewRPCResource(
			"integration/ops",
			api.WithOperations(
				api.OperationSpec{Action: "dry_run", RequiredPermission: "integration.ops.dry_run"},
				api.OperationSpec{Action: "test_connection", RequiredPermission: "integration.ops.test_connection"},
			),
		),
	}
}

// DryRun executes a script against a system under a contract and returns the
// output, the failure classification, and the full wire trace. The calls it
// makes are real; nothing is recorded to statistics or the invocation log.
func (r *OpsResource) DryRun(ctx fiber.Ctx, db orm.DB, params DryRunParams) error {
	contract, err := findByCode[integration.Contract](ctx.Context(), db, params.ContractCode, integration.ErrContractNotFound)
	if err != nil {
		return err
	}

	system, err := findByCode[integration.System](ctx.Context(), db, params.SystemCode, integration.ErrSystemNotFound)
	if err != nil {
		return err
	}

	script := params.Script
	if script == "" {
		if script, err = r.savedScript(ctx.Context(), db, system, contract); err != nil {
			return err
		}
	}

	var input any
	if len(params.Input) > 0 {
		if err := json.Unmarshal(params.Input, &input); err != nil {
			return integration.ErrInputInvalid(err.Error())
		}
	}

	return result.Ok(r.invoker.DryRun(ctx.Context(), contract, system, script, input)).Response(ctx)
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

// savedScript loads the script of the adapter binding system to contract.
func (*OpsResource) savedScript(ctx context.Context, db orm.DB, system *integration.System, contract *integration.Contract) (string, error) {
	adapter := new(integration.Adapter)

	err := db.NewSelect().
		Model(adapter).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("system_id", system.ID).
				Equals("contract_id", contract.ID)
		}).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return "", integration.ErrAdapterNotFound
		}

		return "", err
	}

	return adapter.Script, nil
}

// findByCode loads a definition by its unique code, mapping a missing row to
// notFound. Disabled definitions are intentionally returned.
func findByCode[T any](ctx context.Context, db orm.DB, code string, notFound error) (*T, error) {
	model := new(T)

	err := db.NewSelect().
		Model(model).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", code)
		}).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, notFound
		}

		return nil, err
	}

	return model, nil
}
