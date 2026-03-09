package collector

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

var (
	providerType = reflect.TypeFor[api.OperationsProvider]()
	logger       = log.Named("api.collector")
)

// EmbeddedProviderCollector collects API specs from embedded anonymous structs
// that implement the api.OperationsProvider interface.
type EmbeddedProviderCollector struct{}

// NewEmbeddedProviderCollector creates a new collector for embedded providers.
func NewEmbeddedProviderCollector() api.OperationsCollector {
	return new(EmbeddedProviderCollector)
}

// Collect gathers all API specs from embedded providers in the resource.
func (*EmbeddedProviderCollector) Collect(resource api.Resource) []api.OperationSpec {
	var specs []api.OperationSpec

	visitor := reflectx.Visitor{
		VisitField: func(field reflect.StructField, fieldValue reflect.Value, _ int) reflectx.VisitAction {
			if !field.Anonymous {
				return reflectx.Continue
			}

			provider, ok := asProvider(fieldValue)
			if !ok {
				return reflectx.Continue
			}

			ops := provider.Provide()
			specs = append(specs, ops...)

			if len(ops) > 0 {
				logger.Infof("Collected %d API operations from embedded provider: %s",
					len(ops), field.Type.String())
			}

			return reflectx.Continue
		},
	}

	reflectx.VisitOf(resource, visitor)

	return specs
}

// asProvider attempts to extract an OperationsProvider from a reflect.Value.
func asProvider(value reflect.Value) (api.OperationsProvider, bool) {
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, false
		}

		value = value.Elem()
	}

	t := value.Type()
	if !t.Implements(providerType) && !reflect.PointerTo(t).Implements(providerType) {
		return nil, false
	}

	provider, ok := value.Interface().(api.OperationsProvider)

	return provider, ok
}
