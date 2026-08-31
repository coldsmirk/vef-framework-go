package command

import (
	"context"
	"fmt"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
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
	Labels                 map[string]string
	BindingMode            approval.BindingMode
	BusinessBinding        *approval.BusinessBindingConfig
	AdminUserIDs           []string
	IsAllInitiationAllowed bool
	InstanceTitleTemplate  string
	Initiators             []shared.CreateFlowInitiatorCmd
	Caller                 approval.CallerContext
}

// UpdateFlowHandler handles the UpdateFlowCmd command.
type UpdateFlowHandler struct {
	db               orm.DB
	bindingValidator *binding.ConfigValidator
	// initiatorKinds is the boot-registered initiator vocabulary with the
	// input each kind requires. Save-time validation accepts exactly this set,
	// so a host kind registered through vef.ProvideApprovalInitiatorResolver
	// becomes configurable with no framework change.
	initiatorKinds map[approval.InitiatorKind]approval.SelectionMode
}

// NewUpdateFlowHandler creates a new UpdateFlowHandler.
func NewUpdateFlowHandler(db orm.DB, bindingValidator *binding.ConfigValidator, initiators *strategy.CompositeInitiatorResolver) *UpdateFlowHandler {
	return &UpdateFlowHandler{
		db:               db,
		bindingValidator: bindingValidator,
		initiatorKinds:   shared.SelectionIndex(initiators.Descriptors()),
	}
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

	if err := validateBindingMode(cmd.BindingMode); err != nil {
		return nil, err
	}

	if err := validateInitiatorRules(cmd.IsAllInitiationAllowed, cmd.Initiators, h.initiatorKinds); err != nil {
		return nil, err
	}

	if err := validateFlowLabels(cmd.Labels); err != nil {
		return nil, err
	}

	businessBinding, err := binding.NormalizeConfig(cmd.BindingMode, cmd.BusinessBinding)
	if err != nil {
		return nil, err
	}

	cmd.BusinessBinding = businessBinding
	bindingChanged := flow.BindingMode != cmd.BindingMode || !reflect.DeepEqual(flow.BusinessBinding, businessBinding)

	// Published versions own immutable binding snapshots, so changing the flow
	// only affects the next deployed version and cannot redirect live instances.
	if bindingChanged && businessBinding != nil {
		if err := h.bindingValidator.ValidateSchema(ctx, businessBinding); err != nil {
			return nil, err
		}
	}

	flow.Name = cmd.Name
	flow.Icon = cmd.Icon
	flow.Description = cmd.Description
	flow.Labels = cmd.Labels
	flow.BindingMode = cmd.BindingMode
	flow.BusinessBinding = businessBinding
	flow.AdminUserIDs = cmd.AdminUserIDs
	flow.IsAllInitiationAllowed = cmd.IsAllInitiationAllowed
	flow.InstanceTitleTemplate = cmd.InstanceTitleTemplate

	if _, err := db.NewUpdate().
		Model(&flow).
		Select(
			"name", "icon", "description", "labels",
			"binding_mode", "business_binding",
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
