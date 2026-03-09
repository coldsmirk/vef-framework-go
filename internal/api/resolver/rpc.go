package resolver

import (
	"reflect"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
)

type RPC struct{}

func NewRPC() api.HandlerResolver {
	return new(RPC)
}

func (*RPC) Resolve(resource api.Resource, spec api.OperationSpec) (any, error) {
	if resource.Kind() != api.KindRPC {
		return nil, nil
	}

	// 1. Try explicit handler in Spec (highest priority)
	if spec.Handler != nil {
		return resolveHandlerFromSpec(spec, resource)
	}

	// 2. Fallback to Action name -> PascalCase method lookup
	method, err := findHandlerMethod(reflect.ValueOf(resource), lo.PascalCase(spec.Action))
	if err != nil {
		return nil, err
	}

	if err := validateHandler(method); err != nil {
		return nil, err
	}

	return newFuncHandler(isHandlerFactory(method.Type()), method), nil
}
