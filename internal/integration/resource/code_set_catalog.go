package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ListCodesParams contains the parameters for enumerating one host code set.
type ListCodesParams struct {
	api.P

	CodeSet string `json:"codeSet" validate:"required"`
}

// CodeSetCatalog is the list_code_sets reply. Supported=false means the host
// registered no enumerable catalog and the mapping editor falls back to
// free-text input.
type CodeSetCatalog struct {
	Supported bool               `json:"supported"`
	CodeSets  []mold.CodeSetInfo `json:"codeSets,omitempty"`
}

// CodeCatalog is the list_codes reply for one code set.
type CodeCatalog struct {
	Supported bool            `json:"supported"`
	Codes     []mold.CodeInfo `json:"codes,omitempty"`
}

// CodeSetResource exposes the host's canonical code catalog to the mapping
// editor: when the host's code set registration also implements
// mold.CodeSetInspector, the editor offers code set and value pickers; without
// it the editor degrades to free-text input. The operations reuse the code map
// query permission — the catalog exists solely to support that page.
type CodeSetResource struct {
	api.Resource

	inspector mold.CodeSetInspector
}

// NewCodeSetResource creates the host catalog resource. The inspector is
// asserted from the loader first — the common registration path, where the
// mold module wraps the loader in its cached resolver and would hide the
// enumeration — then from the resolver (hosts that replace it wholesale).
// Neither enumerating reports unsupported.
func NewCodeSetResource(loader mold.CodeSetLoader, resolver mold.CodeSetResolver) api.Resource {
	inspector := resolveCodeSetInspector(loader, resolver)

	return &CodeSetResource{
		Resource: api.NewRPCResource(
			"integration/code_set",
			api.WithOperations(
				api.OperationSpec{Action: "list_code_sets", RequiredPermission: "integration.code_map.query"},
				api.OperationSpec{Action: "list_codes", RequiredPermission: "integration.code_map.query"},
			),
		),
		inspector: inspector,
	}
}

func resolveCodeSetInspector(loader mold.CodeSetLoader, resolver mold.CodeSetResolver) mold.CodeSetInspector {
	inspector, ok := loader.(mold.CodeSetInspector)
	if !ok {
		inspector, _ = resolver.(mold.CodeSetInspector)
	}

	return inspector
}

// ListCodeSets enumerates the code sets the host catalog exposes.
func (r *CodeSetResource) ListCodeSets(ctx fiber.Ctx) error {
	if r.inspector == nil {
		return result.Ok(new(CodeSetCatalog)).Response(ctx)
	}

	codeSets, err := r.inspector.ListCodeSets(ctx.Context())
	if err != nil {
		return integration.ErrCodeSetCatalogFailed(err.Error())
	}

	return result.Ok(&CodeSetCatalog{Supported: true, CodeSets: codeSets}).Response(ctx)
}

// ListCodes enumerates one host code set's codes with display labels.
func (r *CodeSetResource) ListCodes(ctx fiber.Ctx, params ListCodesParams) error {
	if r.inspector == nil {
		return result.Ok(new(CodeCatalog)).Response(ctx)
	}

	codes, err := r.inspector.ListCodes(ctx.Context(), params.CodeSet)
	if err != nil {
		return integration.ErrCodeSetCatalogFailed(err.Error())
	}

	return result.Ok(&CodeCatalog{Supported: true, Codes: codes}).Response(ctx)
}
