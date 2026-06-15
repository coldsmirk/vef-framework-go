package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/tree"
)

// CategoryParams contains the create/update parameters for flow category.
type CategoryParams struct {
	api.P

	ID        string  `json:"id"`
	TenantID  string  `json:"tenantId" validate:"required"`
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
}

// CategoryResource handles flow category CRUD using standard apis generics.
type CategoryResource struct {
	api.Resource

	crud.FindTree[approval.FlowCategory, CategorySearch]
	crud.FindTreeOptions[approval.FlowCategory, CategorySearch]
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
// the caller's tenant. Super-admin callers see all tenants; every other caller
// is confined to their own tenant and fails closed if it has none, preventing
// cross-tenant data exposure.
func categoryTenantApplier(resolver approval.PrincipalTenantResolver) func(query orm.SelectQuery, search CategorySearch, ctx fiber.Ctx) error {
	return func(query orm.SelectQuery, _ CategorySearch, ctx fiber.Ctx) error {
		// fiber.Ctx.Value reads fasthttp UserValues (Locals), which is where
		// the principal is stored; ctx.Context() returns the embedded Go
		// context which does NOT carry Locals.
		caller, err := resolveCaller(ctx.Context(), resolver, contextx.Principal(ctx))
		if err != nil {
			return err
		}

		scope, err := caller.TenantScopeFilter("")
		if err != nil {
			return err
		}

		if scope != nil {
			query.Where(func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", *scope)
			})
		}

		return nil
	}
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
			RequiredPermission("approval:category:query").
			WithQueryApplier(tenantApplier, crud.QueryBase),
		FindTreeOptions: crud.NewFindTreeOptions[approval.FlowCategory, CategorySearch]().
			RequiredPermission("approval:category:query").
			WithQueryApplier(tenantApplier, crud.QueryBase),
		Create: crud.NewCreate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval:category:create").
			WithPreCreate(func(model *approval.FlowCategory, _ *CategoryParams, _ orm.InsertQuery, ctx fiber.Ctx, _ orm.DB) error {
				caller, err := resolveCaller(ctx.Context(), tenantResolver, contextx.Principal(ctx))
				if err != nil {
					return err
				}

				// Non-super-admin callers can only create categories for their own
				// tenant: ResolveWriteTenant ignores the client-submitted tenant and
				// stamps the caller's own (failing closed if it has none), while
				// super-admins keep the requested tenant.
				tenant, err := caller.ResolveWriteTenant(model.TenantID)
				if err != nil {
					return err
				}

				model.TenantID = tenant

				return nil
			}),
		Update: crud.NewUpdate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval:category:update").
			WithPreUpdate(func(oldModel, _ *approval.FlowCategory, _ *CategoryParams, _ orm.UpdateQuery, ctx fiber.Ctx, _ orm.DB) error {
				return authorizeCategoryTenant(ctx, tenantResolver, oldModel.TenantID)
			}),
		Delete: crud.NewDelete[approval.FlowCategory]().
			RequiredPermission("approval:category:delete").
			WithPreDelete(func(model *approval.FlowCategory, _ orm.DeleteQuery, ctx fiber.Ctx, _ orm.DB) error {
				return authorizeCategoryTenant(ctx, tenantResolver, model.TenantID)
			}),
	}
}
