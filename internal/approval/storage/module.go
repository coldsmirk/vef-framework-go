package storage

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Module provides the form-data storage strategies and their Dispatcher. The
// TableStorage strategy is bound to the primary data source's dialect — the
// same dialect the approval migration module targets — so its generated DDL
// matches the schema the rest of the module migrates into.
var Module = fx.Module(
	"vef:approval:storage",

	fx.Provide(
		fx.Private,
		NewJSONStorage,
		newTableStorage,
	),

	fx.Provide(
		NewDispatcher,
	),
)

// newTableStorage adapts NewTableStorage to FX by resolving the dialect from
// the primary data source configuration.
func newTableStorage(dataSources *config.DataSourcesConfig) *TableStorage {
	return NewTableStorage(dataSources.Primary().Kind)
}
