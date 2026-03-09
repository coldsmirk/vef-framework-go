package resource

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/crud"
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

// NewCategoryResource creates a new category resource with standard CRUD operations.
func NewCategoryResource() api.Resource {
	return &CategoryResource{
		Resource:        api.NewRPCResource("approval/category"),
		FindTree:        crud.NewFindTree[approval.FlowCategory, CategorySearch](buildFlowCategoryTree).PermToken("approval:category:query"),
		FindTreeOptions: crud.NewFindTreeOptions[approval.FlowCategory, CategorySearch]().PermToken("approval:category:query"),
		Create:          crud.NewCreate[approval.FlowCategory, CategoryParams]().PermToken("approval:category:create"),
		Update:          crud.NewUpdate[approval.FlowCategory, CategoryParams]().PermToken("approval:category:update"),
		Delete:          crud.NewDelete[approval.FlowCategory]().PermToken("approval:category:delete"),
	}
}
