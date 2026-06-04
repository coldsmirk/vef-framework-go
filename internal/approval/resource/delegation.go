package resource

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// DelegationParams contains the create/update parameters for delegation.
type DelegationParams struct {
	api.P

	ID             string     `json:"id"`
	DelegatorID    string     `json:"delegatorId" validate:"required"`
	DelegateeID    string     `json:"delegateeId" validate:"required"`
	FlowCategoryID *string    `json:"flowCategoryId"`
	FlowID         *string    `json:"flowId"`
	StartTime      *time.Time `json:"startTime"`
	EndTime        *time.Time `json:"endTime"`
	IsActive       bool       `json:"isActive"`
	Reason         *string    `json:"reason"`
}

// DelegationSearch contains the search parameters for delegation.
type DelegationSearch struct {
	crud.Sortable

	DelegatorID string `json:"delegatorId" search:"eq,column=delegator_id"`
	DelegateeID string `json:"delegateeId" search:"eq,column=delegatee_id"`
	IsActive    *bool  `json:"isActive" search:"eq,column=is_active"`
}

// DelegationResource handles delegation CRUD using standard apis generics.
type DelegationResource struct {
	api.Resource

	crud.FindPage[approval.Delegation, DelegationSearch]
	crud.Create[approval.Delegation, DelegationParams]
	crud.Update[approval.Delegation, DelegationParams]
	crud.Delete[approval.Delegation]
}

// NewDelegationResource creates a new delegation resource with standard CRUD operations.
func NewDelegationResource() api.Resource {
	return &DelegationResource{
		Resource: api.NewRPCResource("approval/delegation"),
		FindPage: crud.NewFindPage[approval.Delegation, DelegationSearch]().
			RequiredPermission("approval:delegation:query").
			WithQueryApplier(func(query orm.SelectQuery, _ DelegationSearch, ctx fiber.Ctx) error {
				principal := contextx.Principal(ctx)
				// Super-admin callers may query all delegations; everyone else
				// is confined to records they own as delegator.
				if approval.IsSuperAdmin(principal) {
					return nil
				}

				if principal != nil {
					query.Where(func(cb orm.ConditionBuilder) {
						cb.Equals("delegator_id", principal.ID)
					})
				}

				return nil
			}),
		Create: crud.NewCreate[approval.Delegation, DelegationParams]().
			RequiredPermission("approval:delegation:create").
			WithPreCreate(func(model *approval.Delegation, _ *DelegationParams, _ orm.InsertQuery, ctx fiber.Ctx, _ orm.DB) error {
				principal := contextx.Principal(ctx)
				// Non-super-admin callers can only create delegations on their
				// own behalf; stamp the delegatorId from the principal so the
				// client cannot forge a delegation for another user.
				if approval.IsSuperAdmin(principal) {
					return nil
				}

				if principal != nil {
					model.DelegatorID = principal.ID
				}

				return nil
			}),
		Update: crud.NewUpdate[approval.Delegation, DelegationParams]().
			RequiredPermission("approval:delegation:update").
			WithPreUpdate(func(oldModel, _ *approval.Delegation, _ *DelegationParams, _ orm.UpdateQuery, ctx fiber.Ctx, _ orm.DB) error {
				return authorizeDelegationOwner(ctx, oldModel)
			}),
		Delete: crud.NewDelete[approval.Delegation]().
			RequiredPermission("approval:delegation:delete").
			WithPreDelete(func(model *approval.Delegation, _ orm.DeleteQuery, ctx fiber.Ctx, _ orm.DB) error {
				return authorizeDelegationOwner(ctx, model)
			}),
	}
}

// authorizeDelegationOwner confines non-super-admin callers to delegations they
// own as delegator. crud loads and mutates the target row purely by primary key,
// so without this check any caller holding the update/delete permission could
// modify or remove another delegator's record by supplying its id. Mirrors the
// delegator_id scoping already enforced on FindPage.
func authorizeDelegationOwner(ctx fiber.Ctx, model *approval.Delegation) error {
	principal := contextx.Principal(ctx)
	if approval.IsSuperAdmin(principal) {
		return nil
	}

	if principal == nil || model.DelegatorID != principal.ID {
		return approval.ErrCrossTenantAccess
	}

	return nil
}
