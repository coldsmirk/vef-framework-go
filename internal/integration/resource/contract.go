package resource

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ContractParams contains the create/update parameters for a contract.
type ContractParams struct {
	api.P

	ID           string          `json:"id"`
	Code         string          `json:"code" validate:"required"`
	Name         string          `json:"name" validate:"required"`
	Description  *string         `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	IsEnabled    bool            `json:"isEnabled"`
}

// ContractSearch contains the search parameters for contracts.
type ContractSearch struct {
	crud.Sortable

	Code      string `json:"code" search:"contains"`
	Name      string `json:"name" search:"contains"`
	IsEnabled *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// ContractResource handles contract CRUD using the standard api generics.
type ContractResource struct {
	api.Resource

	crud.FindPage[integration.Contract, ContractSearch]
	crud.FindAll[integration.Contract, ContractSearch]
	crud.Create[integration.Contract, ContractParams]
	crud.Update[integration.Contract, ContractParams]
	crud.Delete[integration.Contract]
}

// NewContractResource creates the contract management resource. Schemas are
// compiled at save time so a broken contract never reaches an invocation.
func NewContractResource() api.Resource {
	validate := func(model *integration.Contract) error {
		return service.ValidateContract(model)
	}

	return &ContractResource{
		Resource: api.NewRPCResource("integration/contract"),
		FindPage: crud.NewFindPage[integration.Contract, ContractSearch]().
			RequiredPermission("integration.contract.query"),
		FindAll: crud.NewFindAll[integration.Contract, ContractSearch]().
			RequiredPermission("integration.contract.query"),
		Create: crud.NewCreate[integration.Contract, ContractParams]().
			RequiredPermission("integration.contract.create").
			WithPreCreate(func(model *integration.Contract, _ *ContractParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return validate(model)
			}),
		Update: crud.NewUpdate[integration.Contract, ContractParams]().
			RequiredPermission("integration.contract.update").
			WithPreUpdate(func(_, model *integration.Contract, _ *ContractParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				return validate(model)
			}),
		Delete: crud.NewDelete[integration.Contract]().
			RequiredPermission("integration.contract.delete").
			WithPreDelete(guardContractRoutes),
	}
}

// guardContractRoutes blocks deleting a contract still referenced by routes:
// the route table's contract column has no foreign key (it carries the ”
// wildcard sentinel), so the check lives here.
func guardContractRoutes(model *integration.Contract, _ orm.DeleteQuery, ctx fiber.Ctx, tx orm.DB) error {
	referenced, err := tx.NewSelect().
		Model((*integration.Route)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("contract_id", model.ID)
		}).
		Exists(ctx.Context())
	if err != nil {
		return err
	}

	if referenced {
		return result.ErrForeignKeyViolation
	}

	return nil
}
