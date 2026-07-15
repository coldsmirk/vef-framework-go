package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// SystemParams contains the create/update parameters for a system. Sensitive
// auth parameter values and the data source password may carry
// integration.MaskedSecret to keep the stored value unchanged.
type SystemParams struct {
	api.P

	ID          string                         `json:"id"`
	Code        string                         `json:"code" validate:"required"`
	Name        string                         `json:"name" validate:"required"`
	BaseURL     string                         `json:"baseUrl"`
	Auth        *integration.AuthConfig        `json:"auth"`
	InboundAuth *integration.InboundAuthConfig `json:"inboundAuth"`
	DataSource  *integration.DataSourceConfig  `json:"dataSource"`
	Params      map[string]string              `json:"params"`
	TimeoutMs   int                            `json:"timeoutMs"`
	Retry       *integration.RetryPolicy       `json:"retry"`
	IsEnabled   bool                           `json:"isEnabled"`
}

// SystemSearch contains the search parameters for systems.
type SystemSearch struct {
	crud.Sortable

	Code      string `json:"code" search:"contains"`
	Name      string `json:"name" search:"contains"`
	IsEnabled *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// SystemResource handles system CRUD. Writes encrypt sensitive auth
// parameters and the data source password, resolving masked placeholders;
// reads always mask them. Deleting a system (or removing/renaming its data
// source) releases its datasource registry entry.
type SystemResource struct {
	api.Resource

	crud.FindPage[integration.System, SystemSearch]
	crud.FindAll[integration.System, SystemSearch]
	crud.Create[integration.System, SystemParams]
	crud.Update[integration.System, SystemParams]
	crud.Delete[integration.System]
}

// NewSystemResource creates the system management resource.
func NewSystemResource(registry *auth.Registry, inboundRegistry *auth.InboundRegistry, codec *service.SecretCodec, invoker *exec.Invoker) api.Resource {
	seal := func(model, prior *integration.System) error {
		scheme, ok := registry.Resolve(model.Auth)
		if !ok {
			return integration.ErrUnknownAuthScheme(model.Auth.Scheme)
		}

		if err := auth.ValidateInboundAuth(inboundRegistry, model.InboundAuth); err != nil {
			return err
		}

		var priorAuth *integration.AuthConfig

		var priorInbound *integration.InboundAuthConfig

		var priorDS *integration.DataSourceConfig

		if prior != nil {
			priorAuth, priorInbound, priorDS = prior.Auth, prior.InboundAuth, prior.DataSource
		}

		if err := codec.EncryptAuth(scheme, model.Auth, priorAuth); err != nil {
			return integration.ErrInvalidAuthParams(err.Error())
		}

		if model.InboundAuth != nil {
			inboundScheme, _ := inboundRegistry.Resolve(model.InboundAuth)
			if err := codec.EncryptInboundAuth(inboundScheme, model.InboundAuth, priorInbound); err != nil {
				return integration.ErrInvalidAuthParams(err.Error())
			}
		}

		if err := codec.EncryptDataSource(model.DataSource, priorDS); err != nil {
			return integration.ErrInvalidDataSource(err.Error())
		}

		return service.ValidateSystem(registry, codec, model)
	}

	mask := func(models []integration.System, _ SystemSearch, _ fiber.Ctx) any {
		for i := range models {
			system := &models[i]
			scheme, _ := registry.Resolve(system.Auth)
			system.Auth = service.MaskAuth(scheme, system.Auth)
			inboundScheme, _ := inboundRegistry.Resolve(system.InboundAuth)
			system.InboundAuth = service.MaskInboundAuth(inboundScheme, system.InboundAuth)
			system.DataSource = service.MaskDataSource(system.DataSource)
		}

		return models
	}

	// release drops a stale datasource registry entry, best effort: the next
	// invocation re-registers whatever is still configured.
	release := func(ctx fiber.Ctx, systemCode string) {
		if err := invoker.ReleaseSystem(ctx.Context(), systemCode); err != nil {
			logger.Errorf("Failed to release data source of system %s: %v", systemCode, err)
		}
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
				return seal(model, oldModel)
			}).
			WithPostUpdate(func(oldModel, model *integration.System, _ *SystemParams, ctx fiber.Ctx, _ orm.DB) error {
				if oldModel.DataSource != nil && (model.DataSource == nil || oldModel.Code != model.Code) {
					release(ctx, oldModel.Code)
				}

				return nil
			}),
		Delete: crud.NewDelete[integration.System]().
			RequiredPermission("integration.system.delete").
			WithPostDelete(func(model *integration.System, ctx fiber.Ctx, _ orm.DB) error {
				if model.DataSource != nil {
					release(ctx, model.Code)
				}

				return nil
			}),
	}
}
