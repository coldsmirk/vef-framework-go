package resource

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/tree"
)

// CategoryParams contains the create/update parameters for flow category.
type CategoryParams struct {
	api.P

	ID string `json:"id"`
	// TenantID is the tenant a new category is created in. Only a super-admin's
	// choice counts — left empty, their own tenant, else the default one;
	// everyone else creates in their own tenant. An update never moves a
	// category, whatever is sent.
	TenantID  string  `json:"tenantId"`
	Code      string  `json:"code" validate:"required"`
	Name      string  `json:"name" validate:"required"`
	Icon      *string `json:"icon"`
	ParentID  *string `json:"parentId"`
	SortOrder int     `json:"sortOrder"`
	IsActive  bool    `json:"isActive"`
	Remark    *string `json:"remark"`
}

// CategorySearch contains the search parameters for flow category.
type CategorySearch struct {
	crud.Sortable

	Name     string `json:"name" search:"contains"`
	IsActive *bool  `json:"isActive" search:"eq,column=is_active"`
	// TenantID narrows a super-admin's read to one tenant —
	// approval.SharedTenantID for the shared categories. Everyone else always
	// reads their own tenant, whatever they send.
	TenantID string `json:"tenantId" search:"-"`
	// IncludeShared adds the shared categories to the tenant read. A picker
	// scoping a delegation needs them, since shared flows sit under shared
	// categories; managing categories and placing a tenant flow do not.
	IncludeShared bool `json:"includeShared" search:"-"`
}

// CategoryResource handles flow category CRUD using standard apis generics.
type CategoryResource struct {
	api.Resource

	crud.FindTree[approval.FlowCategory, CategorySearch]
	crud.Create[approval.FlowCategory, CategoryParams]
	crud.Update[approval.FlowCategory, CategoryParams]
	crud.Delete[approval.FlowCategory]
}

// buildFlowCategoryTree converts flat category records into a nested tree structure.
func buildFlowCategoryTree(flatCategories []approval.FlowCategory) []approval.FlowCategory {
	adapter := tree.Adapter[approval.FlowCategory]{
		GetID: func(c approval.FlowCategory) string {
			return c.ID
		},
		GetParentID: func(c approval.FlowCategory) *string {
			return c.ParentID
		},
		SetChildren: func(c *approval.FlowCategory, children []approval.FlowCategory) {
			c.Children = children
		},
	}

	return tree.Build(flatCategories, adapter)
}

// categoryTenantApplier returns a query applier that scopes category reads to
// the caller's tenant. Super-admin callers see all tenants unless they name
// one; every other caller is confined to their own tenant and fails closed if
// it has none, preventing cross-tenant data exposure. IncludeShared adds the
// shared categories, which every tenant may see.
func categoryTenantApplier(resolver approval.PrincipalTenantResolver) func(query orm.SelectQuery, search CategorySearch, ctx fiber.Ctx) error {
	return func(query orm.SelectQuery, search CategorySearch, ctx fiber.Ctx) error {
		// fiber.Ctx.Value reads fasthttp UserValues (Locals), which is where
		// the principal is stored; ctx.Context() returns the embedded Go
		// context which does NOT carry Locals.
		caller, err := resolveCaller(ctx.Context(), resolver, contextx.Principal(ctx))
		if err != nil {
			return err
		}

		scope, err := caller.TenantScopeFilter(search.TenantID)
		if err != nil {
			return err
		}

		if scope != nil {
			tenantIDs := []string{*scope}
			if search.IncludeShared && *scope != approval.SharedTenantID {
				tenantIDs = append(tenantIDs, approval.SharedTenantID)
			}

			query.Where(func(cb orm.ConditionBuilder) {
				cb.In("tenant_id", tenantIDs)
			})
		}

		return nil
	}
}

// requireParentInTenant rejects placing a category of tenantID under a parent
// that is missing or owned by another tenant, with ErrCategoryNotFound either
// way so the answer reveals nothing about other tenants. A tree never spans
// tenants: a tenant tree hanging off a shared category would surface under
// every tenant's shared view, and one tenant's under another's.
func requireParentInTenant(ctx context.Context, db orm.DB, parentID *string, tenantID string) error {
	if parentID == nil || *parentID == "" {
		return nil
	}

	var parent approval.FlowCategory

	parent.ID = *parentID

	if err := db.NewSelect().
		Model(&parent).
		Select("tenant_id").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return approval.ErrCategoryNotFound
		}

		return fmt.Errorf("load parent category: %w", err)
	}

	if parent.TenantID != tenantID {
		return approval.ErrCategoryNotFound
	}

	return nil
}

// authorizeCategoryTenant gates a category mutation on tenant ownership:
// super-admin callers pass, every other caller must be authorized for the
// entity's tenant. Mirrors authorizeDelegationOwner and collapses the
// byte-identical Update/Delete pre-hook bodies into one place. The resolver is
// passed in because the pre-hooks close over it rather than reading a field.
func authorizeCategoryTenant(ctx fiber.Ctx, resolver approval.PrincipalTenantResolver, entityTenantID string) error {
	principal := contextx.Principal(ctx)
	if approval.IsSuperAdmin(principal) {
		return nil
	}

	caller, err := resolveCaller(ctx.Context(), resolver, principal)
	if err != nil {
		return err
	}

	return caller.Authorize(entityTenantID)
}

// NewCategoryResource creates a new category resource with standard CRUD operations.
func NewCategoryResource(tenantResolver approval.PrincipalTenantResolver) api.Resource {
	tenantApplier := categoryTenantApplier(tenantResolver)

	return &CategoryResource{
		Resource: api.NewRPCResource("approval/category"),
		FindTree: crud.NewFindTree[approval.FlowCategory, CategorySearch](buildFlowCategoryTree).
			RequiredPermission("approval.category.query").
			WithQueryApplier(tenantApplier, crud.QueryBase),
		Create: crud.NewCreate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval.category.create").
			WithPreCreate(func(model *approval.FlowCategory, _ *CategoryParams, _ orm.InsertQuery, ctx fiber.Ctx, db orm.DB) error {
				caller, err := resolveCaller(ctx.Context(), tenantResolver, contextx.Principal(ctx))
				if err != nil {
					return err
				}

				// Non-super-admin callers can only create categories for their own
				// tenant: ResolveWriteTenant ignores the client-submitted tenant and
				// stamps the caller's own (failing closed if it has none), while
				// super-admins keep the requested tenant — the shared one included.
				tenant, err := caller.ResolveWriteTenant(model.TenantID)
				if err != nil {
					return err
				}

				model.TenantID = tenant

				return requireParentInTenant(ctx.Context(), db, model.ParentID, tenant)
			}),
		Update: crud.NewUpdate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval.category.update").
			WithPreUpdate(func(oldModel, newModel *approval.FlowCategory, _ *CategoryParams, _ orm.UpdateQuery, ctx fiber.Ctx, db orm.DB) error {
				if err := authorizeCategoryTenant(ctx, tenantResolver, oldModel.TenantID); err != nil {
					return err
				}

				// A category never changes tenant, for super-admins too: the params
				// carry a client-supplied TenantID that the merge would otherwise
				// write, moving the category — and the flows under it — into
				// another tenant or into the shared one.
				newModel.TenantID = oldModel.TenantID

				return requireParentInTenant(ctx.Context(), db, newModel.ParentID, oldModel.TenantID)
			}),
		Delete: crud.NewDelete[approval.FlowCategory]().
			RequiredPermission("approval.category.delete").
			WithPreDelete(func(model *approval.FlowCategory, _ orm.DeleteQuery, ctx fiber.Ctx, _ orm.DB) error {
				return authorizeCategoryTenant(ctx, tenantResolver, model.TenantID)
			}),
	}
}
