package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AdapterParams contains the create/update parameters for an adapter.
type AdapterParams struct {
	api.P

	ID         string                `json:"id"`
	SystemID   string                `json:"systemId" validate:"required"`
	ContractID string                `json:"contractId" validate:"required"`
	Direction  integration.Direction `json:"direction"`
	Script     string                `json:"script" validate:"required"`
	TimeoutMs  int                   `json:"timeoutMs"`
	IsEnabled  bool                  `json:"isEnabled"`
}

// AdapterSearch contains the search parameters for adapters.
type AdapterSearch struct {
	crud.Sortable

	SystemID   string                `json:"systemId" search:"eq,column=system_id"`
	ContractID string                `json:"contractId" search:"eq,column=contract_id"`
	Direction  integration.Direction `json:"direction" search:"eq,column=direction"`
	IsEnabled  *bool                 `json:"isEnabled" search:"eq,column=is_enabled"`
}

// AdapterResource handles adapter CRUD. Scripts are compile-checked at save
// time; the database's unique and foreign keys guard the binding itself.
type AdapterResource struct {
	api.Resource

	crud.FindPage[integration.Adapter, AdapterSearch]
	crud.FindAll[integration.Adapter, AdapterSearch]
	crud.Create[integration.Adapter, AdapterParams]
	crud.Update[integration.Adapter, AdapterParams]
	crud.Delete[integration.Adapter]
}

// NewAdapterResource creates the adapter management resource.
func NewAdapterResource() api.Resource {
	return &AdapterResource{
		Resource: api.NewRPCResource("integration/adapter"),
		FindPage: crud.NewFindPage[integration.Adapter, AdapterSearch]().
			RequiredPermission("integration.adapter.query"),
		FindAll: crud.NewFindAll[integration.Adapter, AdapterSearch]().
			RequiredPermission("integration.adapter.query"),
		Create: crud.NewCreate[integration.Adapter, AdapterParams]().
			RequiredPermission("integration.adapter.create").
			WithPreCreate(func(model *integration.Adapter, _ *AdapterParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return sealAdapter(model)
			}),
		Update: crud.NewUpdate[integration.Adapter, AdapterParams]().
			RequiredPermission("integration.adapter.update").
			WithPreUpdate(func(_, model *integration.Adapter, _ *AdapterParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				return sealAdapter(model)
			}),
		Delete: crud.NewDelete[integration.Adapter]().
			RequiredPermission("integration.adapter.delete"),
	}
}

// sealAdapter normalizes and validates an adapter before persistence: an
// omitted direction defaults to outbound, an unknown one is rejected, and the
// script must compile.
func sealAdapter(model *integration.Adapter) error {
	if model.Direction == "" {
		model.Direction = integration.DirectionOutbound
	}

	if !model.Direction.IsValid() {
		return integration.ErrInvalidDirection
	}

	return definition.ValidateAdapterScript(model.Script)
}
