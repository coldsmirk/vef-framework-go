package resource

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/tree"
)

// CategoryParams contains the create/update parameters for flow category.
type CategoryParams struct {
	api.P

	ID string `json:"id"`
	// TenantID is the tenant the category belongs to. An update never moves a
	// category, whatever is sent.
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
	// TenantID narrows the read to one tenant's categories; omitted, every
	// tenant's are read.
	TenantID string `json:"tenantId" search:"-"`
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

// applyCategoryTenant narrows a category read to the tenant the search names.
// Flow categories are managed across tenants, gated by permission, so a read
// naming no tenant covers every tenant.
func applyCategoryTenant(query orm.SelectQuery, search CategorySearch, _ fiber.Ctx) error {
	if search.TenantID == "" {
		return nil
	}

	query.Where(func(cb orm.ConditionBuilder) {
		cb.Equals("tenant_id", search.TenantID)
	})

	return nil
}

// requireParentInTenant rejects placing a category of tenantID under a parent
// that is missing or owned by another tenant, with ErrCategoryNotFound either
// way so the answer reveals nothing about other tenants. A tree never spans
// tenants: one tenant's categories would otherwise surface under another's.
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

// NewCategoryResource creates a new category resource with standard CRUD
// operations. Categories are managed across tenants, each operation gated by its
// permission; a tree never spans tenants and a category never changes tenant.
func NewCategoryResource() api.Resource {
	return &CategoryResource{
		Resource: api.NewRPCResource("approval/category"),
		FindTree: crud.NewFindTree[approval.FlowCategory, CategorySearch](buildFlowCategoryTree).
			RequiredPermission("approval.category.query").
			WithQueryApplier(applyCategoryTenant, crud.QueryBase),
		Create: crud.NewCreate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval.category.create").
			WithPreCreate(func(model *approval.FlowCategory, _ *CategoryParams, _ orm.InsertQuery, ctx fiber.Ctx, db orm.DB) error {
				return requireParentInTenant(ctx.Context(), db, model.ParentID, model.TenantID)
			}),
		Update: crud.NewUpdate[approval.FlowCategory, CategoryParams]().
			RequiredPermission("approval.category.update").
			WithPreUpdate(func(oldModel, newModel *approval.FlowCategory, _ *CategoryParams, _ orm.UpdateQuery, ctx fiber.Ctx, db orm.DB) error {
				// A category never changes tenant: the params carry a TenantID
				// that the merge would otherwise write, moving the category — and
				// the flows under it — into another tenant.
				newModel.TenantID = oldModel.TenantID

				return requireParentInTenant(ctx.Context(), db, newModel.ParentID, oldModel.TenantID)
			}),
		Delete: crud.NewDelete[approval.FlowCategory]().
			RequiredPermission("approval.category.delete"),
	}
}
