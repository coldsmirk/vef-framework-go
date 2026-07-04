package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// UpdateFlowCmd updates an existing flow.
type UpdateFlowCmd struct {
	cqrs.BaseCommand

	FlowID                 string
	Name                   string
	Icon                   *string
	Description            *string
	BindingMode            approval.BindingMode
	BusinessTable          *string
	BusinessPkField        *string
	BusinessStatusField    *string
	AdminUserIDs           []string
	IsAllInitiationAllowed bool
	InstanceTitleTemplate  string
	Initiators             []shared.CreateFlowInitiatorCmd
	Caller                 approval.CallerContext
}

// UpdateFlowHandler handles the UpdateFlowCmd command.
type UpdateFlowHandler struct {
	db orm.DB
}

// NewUpdateFlowHandler creates a new UpdateFlowHandler.
func NewUpdateFlowHandler(db orm.DB) *UpdateFlowHandler {
	return &UpdateFlowHandler{db: db}
}

func (h *UpdateFlowHandler) Handle(ctx context.Context, cmd UpdateFlowCmd) (*approval.Flow, error) {
	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = cmd.FlowID

	if err := db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("query flow: %w", err)
	}

	if err := cmd.Caller.Authorize(flow.TenantID); err != nil {
		return nil, shared.ErrFlowNotFound
	}

	if err := validateInstanceTitleTemplate(cmd.InstanceTitleTemplate); err != nil {
		return nil, err
	}

	if err := validateBusinessIdentifiers(cmd.BindingMode, cmd.BusinessTable, cmd.BusinessPkField, cmd.BusinessStatusField); err != nil {
		return nil, err
	}

	if err := validateBusinessBindingComplete(cmd.BindingMode, cmd.BusinessTable, cmd.BusinessPkField, cmd.BusinessStatusField); err != nil {
		return nil, err
	}

	// Freeze the business binding while instances are running: re-pointing (or
	// clearing) the binding mid-flight would make in-flight instances write their
	// outcome back to a different business record — or none — than they were
	// started against. Only a real binding change is blocked, so editing name,
	// initiators, admins, etc. stays allowed while instances run.
	if bindingConfigChanged(&flow, cmd) {
		running, err := db.NewSelect().
			Model((*approval.Instance)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_id", cmd.FlowID)
				cb.Equals("status", string(approval.InstanceRunning))
			}).
			Exists(ctx)
		if err != nil {
			return nil, fmt.Errorf("check running instances before binding change: %w", err)
		}

		if running {
			return nil, shared.ErrFlowBindingLocked
		}
	}

	flow.Name = cmd.Name
	flow.Icon = cmd.Icon
	flow.Description = cmd.Description
	flow.BindingMode = cmd.BindingMode
	flow.BusinessTable = cmd.BusinessTable
	flow.BusinessPkField = cmd.BusinessPkField
	flow.BusinessStatusField = cmd.BusinessStatusField
	flow.AdminUserIDs = cmd.AdminUserIDs
	flow.IsAllInitiationAllowed = cmd.IsAllInitiationAllowed
	flow.InstanceTitleTemplate = cmd.InstanceTitleTemplate

	if _, err := db.NewUpdate().
		Model(&flow).
		Select(
			"name", "icon", "description",
			"binding_mode", "business_table", "business_pk_field", "business_status_field",
			"admin_user_ids", "is_all_initiation_allowed", "instance_title_template",
		).
		WherePK().
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("update flow: %w", err)
	}

	if _, err := db.NewDelete().
		Model((*approval.FlowInitiator)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", cmd.FlowID)
		}).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("delete existing initiators: %w", err)
	}

	if len(cmd.Initiators) > 0 {
		initiators := make([]approval.FlowInitiator, len(cmd.Initiators))
		for i, init := range cmd.Initiators {
			initiators[i] = approval.FlowInitiator{
				FlowID: cmd.FlowID,
				Kind:   init.Kind,
				IDs:    init.IDs,
			}
		}

		if _, err := db.NewInsert().
			Model(&initiators).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert flow initiators: %w", err)
		}
	}

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewFlowUpdatedEvent(&flow),
	)

	return &flow, nil
}

// bindingConfigChanged reports whether cmd alters any business-binding field
// (mode / table / pk / status) relative to the flow's persisted state. Only a
// binding change is gated by the running-instance guard; non-binding edits are
// always allowed.
func bindingConfigChanged(current *approval.Flow, cmd UpdateFlowCmd) bool {
	return current.BindingMode != cmd.BindingMode ||
		!stringPtrEqual(current.BusinessTable, cmd.BusinessTable) ||
		!stringPtrEqual(current.BusinessPkField, cmd.BusinessPkField) ||
		!stringPtrEqual(current.BusinessStatusField, cmd.BusinessStatusField)
}

// stringPtrEqual reports whether two optional strings hold the same value,
// treating both-nil as equal.
func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
