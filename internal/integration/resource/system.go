package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// SystemParams contains the create/update parameters for a system. Sensitive
// auth parameter values may carry integration.MaskedSecret to keep the
// stored value unchanged.
type SystemParams struct {
	api.P

	ID        string                   `json:"id"`
	Code      string                   `json:"code" validate:"required"`
	Name      string                   `json:"name" validate:"required"`
	BaseURL   string                   `json:"baseUrl"`
	Auth      *integration.AuthConfig  `json:"auth"`
	Params    map[string]string        `json:"params"`
	TimeoutMs int                      `json:"timeoutMs"`
	Retry     *integration.RetryPolicy `json:"retry"`
	IsEnabled bool                     `json:"isEnabled"`
}

// SystemSearch contains the search parameters for systems.
type SystemSearch struct {
	crud.Sortable

	Code      string `json:"code" search:"contains"`
	Name      string `json:"name" search:"contains"`
	IsEnabled *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// SystemResource handles system CRUD. Writes encrypt sensitive auth
// parameters and resolve masked placeholders; reads always mask them.
type SystemResource struct {
	api.Resource

	crud.FindPage[integration.System, SystemSearch]
	crud.FindAll[integration.System, SystemSearch]
	crud.Create[integration.System, SystemParams]
	crud.Update[integration.System, SystemParams]
	crud.Delete[integration.System]
}

// NewSystemResource creates the system management resource.
func NewSystemResource(registry *auth.Registry, codec *service.SecretCodec) api.Resource {
	seal := func(model *integration.System, prior *integration.AuthConfig) error {
		scheme, ok := registry.Resolve(model.Auth)
		if !ok {
			return integration.ErrUnknownAuthScheme(model.Auth.Scheme)
		}

		if err := codec.EncryptAuth(scheme, model.Auth, prior); err != nil {
			return integration.ErrInvalidAuthParams(err.Error())
		}

		return service.ValidateSystem(registry, codec, model)
	}

	mask := func(models []integration.System, _ SystemSearch, _ fiber.Ctx) any {
		for i := range models {
			system := &models[i]
			scheme, _ := registry.Resolve(system.Auth)
			system.Auth = service.MaskAuth(scheme, system.Auth)
		}

		return models
	}

	return &SystemResource{
		Resource: api.NewRPCResource("integration/system"),
		FindPage: crud.NewFindPage[integration.System, SystemSearch]().
			RequiredPermission("integration.system.query").
			WithProcessor(mask),
		FindAll: crud.NewFindAll[integration.System, SystemSearch]().
			RequiredPermission("integration.system.query").
			WithProcessor(mask),
		Create: crud.NewCreate[integration.System, SystemParams]().
			RequiredPermission("integration.system.create").
			WithPreCreate(func(model *integration.System, _ *SystemParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return seal(model, nil)
			}),
		Update: crud.NewUpdate[integration.System, SystemParams]().
			RequiredPermission("integration.system.update").
			WithPreUpdate(func(oldModel, model *integration.System, _ *SystemParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				return seal(model, oldModel.Auth)
			}),
		Delete: crud.NewDelete[integration.System]().
			RequiredPermission("integration.system.delete"),
	}
}
