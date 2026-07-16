package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RouteParams contains the create/update parameters for a route. An empty
// RouteKey is the default route; an empty ContractID scopes the rule to
// every contract.
type RouteParams struct {
	api.P

	ID         string `json:"id"`
	RouteKey   string `json:"routeKey"`
	ContractID string `json:"contractId"`
	SystemID   string `json:"systemId" validate:"required"`
	IsEnabled  bool   `json:"isEnabled"`
}

// RouteSearch contains the search parameters for routes.
type RouteSearch struct {
	crud.Sortable

	RouteKey   string `json:"routeKey" search:"contains,column=route_key"`
	ContractID string `json:"contractId" search:"eq,column=contract_id"`
	SystemID   string `json:"systemId" search:"eq,column=system_id"`
	IsEnabled  *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// RouteResource handles route CRUD. The contract reference is validated at
// save time because the column carries the empty-string wildcard sentinel and
// has no foreign key.
type RouteResource struct {
	api.Resource

	crud.FindPage[integration.Route, RouteSearch]
	crud.FindAll[integration.Route, RouteSearch]
	crud.Create[integration.Route, RouteParams]
	crud.Update[integration.Route, RouteParams]
	crud.Delete[integration.Route]
}

// NewRouteResource creates the route management resource.
func NewRouteResource() api.Resource {
	return &RouteResource{
		Resource: api.NewRPCResource("integration/route"),
		FindPage: crud.NewFindPage[integration.Route, RouteSearch]().
			RequiredPermission("integration.route.query"),
		FindAll: crud.NewFindAll[integration.Route, RouteSearch]().
			RequiredPermission("integration.route.query"),
		Create: crud.NewCreate[integration.Route, RouteParams]().
			RequiredPermission("integration.route.create").
			WithPreCreate(func(model *integration.Route, _ *RouteParams, _ orm.InsertQuery, ctx fiber.Ctx, tx orm.DB) error {
				return definition.ValidateRouteRefs(ctx.Context(), tx, model)
			}),
		Update: crud.NewUpdate[integration.Route, RouteParams]().
			RequiredPermission("integration.route.update").
			WithPreUpdate(func(_, model *integration.Route, _ *RouteParams, _ orm.UpdateQuery, ctx fiber.Ctx, tx orm.DB) error {
				return definition.ValidateRouteRefs(ctx.Context(), tx, model)
			}),
		Delete: crud.NewDelete[integration.Route]().
			RequiredPermission("integration.route.delete"),
	}
}
