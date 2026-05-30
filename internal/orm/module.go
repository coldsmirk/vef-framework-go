package orm

import (
	"go.uber.org/fx"
)

// Module derives the primary orm.DB from the data sources registry provided
// by DataSourcesModule. Most callers inject orm.DB directly and get the
// primary source; cross-source access goes through orm.DataSources.
var Module = fx.Module(
	"vef:orm",
	fx.Provide(providePrimary),
)

func providePrimary(ds DataSources) DB {
	return ds.Primary()
}
