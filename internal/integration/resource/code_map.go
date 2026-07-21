package resource

import (
	"context"
	"fmt"
	"slices"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// CodeMapParams contains the create/update parameters for a code map.
type CodeMapParams struct {
	api.P

	ID                string                     `json:"id"`
	SystemID          string                     `json:"systemId" validate:"required"`
	CodeSet           string                     `json:"codeSet" validate:"required"`
	Name              string                     `json:"name" validate:"required"`
	Entries           []integration.CodeMapEntry `json:"entries"`
	OnUnmapped        integration.UnmappedPolicy `json:"onUnmapped"`
	FallbackCanonical any                        `json:"fallbackCanonical"`
	FallbackExternal  any                        `json:"fallbackExternal"`
	IsEnabled         bool                       `json:"isEnabled"`
}

// CodeMapSearch contains the search parameters for code maps.
type CodeMapSearch struct {
	crud.Sortable

	SystemID  string `json:"systemId" search:"eq,column=system_id"`
	CodeSet   string `json:"codeSet" search:"contains,column=code_set"`
	Name      string `json:"name" search:"contains"`
	IsEnabled *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// CodeMapResource handles code map CRUD. Entries are index-built at save time
// so a colliding or malformed map never reaches a lookup; the database's
// unique and foreign keys guard the system binding.
type CodeMapResource struct {
	api.Resource

	crud.FindPage[integration.CodeMap, CodeMapSearch]
	crud.FindAll[integration.CodeMap, CodeMapSearch]
	crud.Create[integration.CodeMap, CodeMapParams]
	crud.Update[integration.CodeMap, CodeMapParams]
	crud.Delete[integration.CodeMap]
}

// NewCodeMapResource creates the code map management resource.
func NewCodeMapResource(loader mold.CodeSetLoader, resolver mold.CodeSetResolver) api.Resource {
	inspector := resolveCodeSetInspector(loader, resolver)

	return &CodeMapResource{
		Resource: api.NewRPCResource("integration/code_map"),
		FindPage: crud.NewFindPage[integration.CodeMap, CodeMapSearch]().
			RequiredPermission("integration.code_map.query"),
		FindAll: crud.NewFindAll[integration.CodeMap, CodeMapSearch]().
			RequiredPermission("integration.code_map.query"),
		Create: crud.NewCreate[integration.CodeMap, CodeMapParams]().
			RequiredPermission("integration.code_map.create").
			WithPreCreate(func(model *integration.CodeMap, _ *CodeMapParams, _ orm.InsertQuery, ctx fiber.Ctx, _ orm.DB) error {
				return sealCodeMap(ctx.Context(), inspector, model)
			}),
		Update: crud.NewUpdate[integration.CodeMap, CodeMapParams]().
			RequiredPermission("integration.code_map.update").
			WithPreUpdate(func(_, model *integration.CodeMap, _ *CodeMapParams, _ orm.UpdateQuery, ctx fiber.Ctx, _ orm.DB) error {
				return sealCodeMap(ctx.Context(), inspector, model)
			}),
		Delete: crud.NewDelete[integration.CodeMap]().
			RequiredPermission("integration.code_map.delete"),
	}
}

// sealCodeMap normalizes and validates a code map before persistence: an
// omitted unmapped policy defaults to reject (fail closed), and the entries
// must build a collision-free lookup index. An enumerable host catalog also
// constrains the identifier to one of its registered code sets.
func sealCodeMap(ctx context.Context, inspector mold.CodeSetInspector, model *integration.CodeMap) error {
	if model.OnUnmapped == "" {
		model.OnUnmapped = integration.UnmappedPolicyReject
	}

	if err := definition.ValidateCodeMap(model); err != nil {
		return err
	}

	if inspector == nil {
		return nil
	}

	codeSets, err := inspector.ListCodeSets(ctx)
	if err != nil {
		// The identifier cannot be confirmed, so the save is rejected — as the
		// catalog fault it is, never a raw host error escaping the integration
		// error vocabulary and never as a verdict on the definition.
		return integration.ErrCodeSetCatalogFailed(err.Error())
	}

	if !slices.ContainsFunc(codeSets, func(info mold.CodeSetInfo) bool {
		return info.CodeSet == model.CodeSet
	}) {
		return integration.ErrInvalidCodeMap(fmt.Sprintf("code set %q is not registered by the host catalog", model.CodeSet))
	}

	return nil
}
