package param

import (
	"fmt"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

type FactoryParamResolverFunc func() (reflect.Value, error)

type FactoryParamResolverManager struct {
	resolvers map[reflect.Type]FactoryParamResolverFunc
}

func NewFactoryParamResolverManager(resolvers []api.FactoryParamResolver) *FactoryParamResolverManager {
	resolverMap := make(map[reflect.Type]FactoryParamResolverFunc, len(resolvers))

	for _, resolver := range resolvers {
		t := resolver.Type()
		resolverMap[t] = resolver.Resolve
	}

	return &FactoryParamResolverManager{
		resolvers: resolverMap,
	}
}

func (m *FactoryParamResolverManager) Resolve(
	target reflect.Value,
	paramType reflect.Type,
) (FactoryParamResolverFunc, error) {
	if resolver, ok := m.resolvers[paramType]; ok {
		return resolver, nil
	}

	if field := findFieldInStruct(target, paramType); field.IsValid() {
		return buildFactoryFieldResolver(field, paramType)
	}

	return nil, fmt.Errorf("%w: %s", ErrResolveFactoryParamType, paramType.String())
}

func buildFactoryFieldResolver(
	field reflect.Value,
	targetType reflect.Type,
) (FactoryParamResolverFunc, error) {
	converted, err := reflectx.ConvertValue(field, targetType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert field value: %w", err)
	}

	return func() (reflect.Value, error) { return converted, nil }, nil
}
