package orm

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
)

// PrimaryDataSourceName is the reserved name for the data source declared under
// vef.data_sources.primary. The primary source is constructed from the TOML
// configuration, exposed in the FX container as orm.DB, and cannot be mutated
// through Register/Update/Unregister.
const PrimaryDataSourceName = "primary"

// DataSources is the registry of named orm.DB instances. Applications inject
// it whenever they need to reach a data source other than the primary one.
//
// All read methods (Get/Has/Names/Kind/Primary) are safe for concurrent use.
// Register/Update/Unregister mutate the registry atomically; Reconcile is
// serialized so concurrent reconciles cannot fight each other.
type DataSources interface {
	// Primary returns the orm.DB bound to the primary TOML configuration. It
	// is equivalent to Get(PrimaryDataSourceName) but never returns an error.
	Primary() DB

	// Get returns the orm.DB registered under name. It returns
	// ErrDataSourceNotFound when the name has never been registered and
	// ErrDataSourceClosed when the entry was Unregister'd.
	Get(name string) (DB, error)

	// Has reports whether name is currently registered and not closed.
	Has(name string) bool

	// Names returns every currently registered name (including primary) in
	// stable lexical order. Closed entries are excluded.
	Names() []string

	// Kind returns the dialect for the named data source. It returns
	// ErrDataSourceNotFound / ErrDataSourceClosed for the same conditions as
	// Get. Useful for sqlmigration / schema reflection callers that need to
	// branch on dialect without pulling out the full DB.
	Kind(name string) (config.DBKind, error)

	// Register opens a new data source and inserts it under name. It returns
	// ErrDataSourceExists if the name is already registered and
	// ErrPrimaryReserved if name equals PrimaryDataSourceName. The newly
	// opened connection is closed and not retained on conflict.
	Register(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...RegisterOption) (DB, error)

	// Update atomically replaces the connection for an existing data source
	// with one opened from cfg. The new connection must Open and Ping
	// successfully before the swap; on failure the existing entry is left
	// untouched. The old underlying *sql.DB is closed asynchronously so
	// callers that already hold a DB reference observe no downtime.
	//
	// Update returns ErrDataSourceNotFound when name is not registered and
	// ErrPrimaryReserved when name is PrimaryDataSourceName.
	Update(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...RegisterOption) (DB, error)

	// Unregister soft-closes the named data source. Subsequent Get calls
	// return ErrDataSourceClosed; callers already holding a DB reference
	// will fail their next query with a driver-level error once the
	// underlying *sql.DB is closed. Unregister returns ErrPrimaryReserved
	// for the primary source and ErrDataSourceNotFound when name is not
	// registered.
	Unregister(ctx context.Context, name string) error

	// Reconcile drives the registry toward the supplied desired set of
	// non-primary sources. The implementation computes three buckets:
	//
	//   - specs has name but registry does not   → Register
	//   - both have name but cfg differs         → Update
	//   - registry has name but specs does not   → Unregister
	//
	// Reconciles are serialized (not concurrent) so two ticks of a refresher
	// job never race. Per-name failures populate ReconcileReport.Errors and
	// do not interrupt processing of the remaining names. specs entries
	// referencing the primary name are ignored.
	Reconcile(ctx context.Context, specs []DataSourceSpec, opts ...ReconcileOption) (ReconcileReport, error)

	// HealthCheck pings every registered source in parallel and returns a
	// name -> error map. A nil error indicates the source is reachable.
	HealthCheck(ctx context.Context) map[string]error
}

// DataSourceProvider supplies additional data sources during application
// startup. The framework calls Load on every registered provider after the
// primary and static (TOML) data sources are already in the registry; each
// returned DataSourceSpec is then Register'd. Provider order is undefined,
// and a name collision (with TOML or another provider) is reported as a
// startup failure.
type DataSourceProvider interface {
	// Name identifies the provider in error messages and logs. It does not
	// need to be globally unique but should be descriptive (for example
	// "tenant-table" or "vault-secrets").
	Name() string
	// Load returns the data sources this provider knows about. It runs once
	// during the FX startup phase and any error aborts boot.
	Load(ctx context.Context) ([]DataSourceSpec, error)
}

// DataSourceSpec is the (name, config) pair produced by a DataSourceProvider
// and consumed by DataSources.Reconcile.
type DataSourceSpec struct {
	// Name is the registry key. Must be non-empty and not equal to
	// PrimaryDataSourceName.
	Name string
	// Cfg is the connection configuration applied by Open.
	Cfg config.DataSourceConfig
}

// RegisterOption tunes a single Register or Update call. Options compose;
// later options override earlier ones for the same field.
type RegisterOption func(*RegisterOptions)

// RegisterOptions holds the tunables a Register/Update call honors. It is
// exported so registry implementations can read it; user code should
// construct it via the RegisterOption helpers.
type RegisterOptions struct {
	// CloseGrace controls how long the registry waits before closing a
	// replaced or unregistered *bun.DB. Zero (the default) closes immediately
	// on a background goroutine.
	CloseGrace time.Duration
}

// WithCloseGrace returns a RegisterOption that delays the asynchronous close
// of a replaced/unregistered underlying *bun.DB by d. Use it to give in-flight
// queries some time to drain before the connection pool tears down.
func WithCloseGrace(d time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		if d > 0 {
			o.CloseGrace = d
		}
	}
}

// ReconcileOption tunes a single Reconcile invocation.
type ReconcileOption func(*ReconcileOptions)

// ReconcileOptions holds the tunables a Reconcile call honors.
type ReconcileOptions struct {
	// DryRun makes Reconcile compute the diff and return it in the report
	// without performing Register/Update/Unregister. Useful for previewing
	// what a refresher job would do.
	DryRun bool
}

// WithReconcileDryRun returns a ReconcileOption that flips Reconcile into
// preview mode: the report still lists Added/Updated/Removed but no
// connections are opened or closed.
func WithReconcileDryRun() ReconcileOption {
	return func(o *ReconcileOptions) {
		o.DryRun = true
	}
}

// ReconcileReport summarizes the result of a Reconcile call. Errors is keyed
// by data source name and is nil when every action succeeded.
type ReconcileReport struct {
	Added   []string
	Updated []string
	Removed []string
	Errors  map[string]error
}
