package collector

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/api"
)

// ResourceProviderCollector collects API specs if the resource itself implements
// the api.OperationsProvider interface.
type ResourceProviderCollector struct{}

// NewResourceProviderCollector creates a new collector for resource provider implementation.
func NewResourceProviderCollector() api.OperationsCollector {
	return new(ResourceProviderCollector)
}

// Collect checks if the resource implements api.OperationsProvider and collects its specs.
func (*ResourceProviderCollector) Collect(resource api.Resource) []api.OperationSpec {
	specs := resource.Operations()
	if len(specs) > 0 {
		logger.Infof(
			"Collected %d API operations from resource provider: %s",
			len(specs), reflect.TypeOf(resource).String(),
		)
	}

	return specs
}
