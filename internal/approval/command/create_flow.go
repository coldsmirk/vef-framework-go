package command

import (
	"context"
	"fmt"

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

// CreateFlowCmd creates a new flow with its initiator configurations.
type CreateFlowCmd struct {
	cqrs.BaseCommand

	// TenantID is the tenant the flow belongs to, as the request names it —
	// approval.SharedTenantID for a flow offered to every tenant. Managing flow
	// definitions is gated by permission, not by the caller's own tenant.
	TenantID               string
	Code                   string
	Name                   string
	CategoryID             string
	Icon                   *string
	Description            *string
	Labels                 map[string]string
	BindingMode            approval.BindingMode
	BusinessBinding        *approval.BusinessBindingConfig
	AdminUserIDs           []string
	IsAllInitiationAllowed bool
	InstanceTitleTemplate  string
	Initiators             []shared.CreateFlowInitiatorCmd
}

// CreateFlowHandler handles the CreateFlowCmd command.
type CreateFlowHandler struct {
	db               orm.DB
	bindingValidator *binding.ConfigValidator
	// initiatorKinds is the boot-registered initiator vocabulary with the
	// input each kind requires. Save-time validation accepts exactly this set,
	// so a host kind registered through vef.ProvideApprovalInitiatorResolver
	// becomes configurable with no framework change.
	initiatorKinds map[approval.InitiatorKind]approval.SelectionMode
}

// NewCreateFlowHandler creates a new CreateFlowHandler.
func NewCreateFlowHandler(db orm.DB, bindingValidator *binding.ConfigValidator, initiators *strategy.CompositeInitiatorResolver) *CreateFlowHandler {
	return &CreateFlowHandler{
		db:               db,
		bindingValidator: bindingValidator,
		initiatorKinds:   shared.SelectionIndex(initiators.Descriptors()),
	}
}

func (h *CreateFlowHandler) Handle(ctx context.Context, cmd CreateFlowCmd) (*approval.Flow, error) {
	db := contextx.DB(ctx, h.db)
	tenantID := cmd.TenantID

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

	if businessBinding != nil {
		if err := h.bindingValidator.ValidateSchema(ctx, businessBinding); err != nil {
			return nil, err
		}
	}

	if err := validateInstanceTitleTemplate(cmd.InstanceTitleTemplate); err != nil {
		return nil, err
	}

	exists, err := db.NewSelect().
		Model((*approval.Flow)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("tenant_id", tenantID).
				Equals("code", cmd.Code)
		}).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("query flow by code: %w", err)
	}

	if exists {
		return nil, approval.ErrFlowCodeExists
	}

	if err := requireTenantCategory(ctx, db, cmd.CategoryID, tenantID); err != nil {
		return nil, err
	}

	flow := approval.Flow{
		TenantID:               tenantID,
		CategoryID:             cmd.CategoryID,
		Code:                   cmd.Code,
		Name:                   cmd.Name,
		Icon:                   cmd.Icon,
		Description:            cmd.Description,
		Labels:                 cmd.Labels,
		BindingMode:            cmd.BindingMode,
		BusinessBinding:        businessBinding,
		AdminUserIDs:           cmd.AdminUserIDs,
		IsAllInitiationAllowed: cmd.IsAllInitiationAllowed,
		InstanceTitleTemplate:  cmd.InstanceTitleTemplate,
		IsActive:               true,
		CurrentVersion:         0,
	}
	if _, err := db.NewInsert().
		Model(&flow).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert flow: %w", err)
	}

	if len(cmd.Initiators) > 0 {
		initiators := make([]approval.FlowInitiator, len(cmd.Initiators))
		for i, init := range cmd.Initiators {
			initiators[i] = approval.FlowInitiator{
				FlowID: flow.ID,
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
		approval.NewFlowCreatedEvent(&flow),
	)

	return &flow, nil
}

// requireTenantCategory rejects placing a flow of tenantID under a category
// that is missing or owned by another tenant, with ErrCategoryNotFound either
// way so the answer reveals nothing about other tenants. A flow and its
// category always share a tenant: a shared flow sits under a shared category,
// whose name every tenant may see, and no tenant's category name reaches
// another tenant through a flow.
func requireTenantCategory(ctx context.Context, db orm.DB, categoryID, tenantID string) error {
	var category approval.FlowCategory

	category.ID = categoryID

	if err := db.NewSelect().
		Model(&category).
		Select("tenant_id").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return approval.ErrCategoryNotFound
		}

		return fmt.Errorf("load flow category: %w", err)
	}

	if category.TenantID != tenantID {
		return approval.ErrCategoryNotFound
	}

	return nil
}
