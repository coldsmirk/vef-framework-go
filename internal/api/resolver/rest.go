package resolver

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
)

type REST struct{}

func NewRest() api.HandlerResolver {
	return new(REST)
}

func (*REST) Resolve(resource api.Resource, spec api.OperationSpec) (any, error) {
	if resource.Kind() != api.KindREST {
		return nil, nil
	}

	if spec.Handler == nil {
		return nil, fmt.Errorf("%w (resource: %s, action: %s)",
			shared.ErrHandlerRequired, resource.Name(), spec.Action)
	}

	return resolveHandlerFromSpec(spec, resource)
}
