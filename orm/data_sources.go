package orm

import "github.com/coldsmirk/vef-framework-go/internal/orm"

type (
	DataSources        = orm.DataSources
	DataSourceProvider = orm.DataSourceProvider
	DataSourceSpec     = orm.DataSourceSpec
	RegisterOption     = orm.RegisterOption
	RegisterOptions    = orm.RegisterOptions
	ReconcileOption    = orm.ReconcileOption
	ReconcileOptions   = orm.ReconcileOptions
	ReconcileReport    = orm.ReconcileReport
)

const PrimaryDataSourceName = orm.PrimaryDataSourceName

var (
	ErrDataSourceNotFound    = orm.ErrDataSourceNotFound
	ErrDataSourceExists      = orm.ErrDataSourceExists
	ErrDataSourceClosed      = orm.ErrDataSourceClosed
	ErrPrimaryReserved       = orm.ErrPrimaryReserved
	ErrDataSourceNameInvalid = orm.ErrDataSourceNameInvalid

	WithCloseGrace      = orm.WithCloseGrace
	WithReconcileDryRun = orm.WithReconcileDryRun
)
