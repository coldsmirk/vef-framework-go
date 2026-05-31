package datasource

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// PrimaryName is the reserved name for the data source declared under
// vef.data_sources.primary. The primary source is constructed from the TOML
// configuration, exposed in the FX container as orm.DB, and cannot be mutated
// through Register/Update/Unregister.
const PrimaryName = "primary"

// Registry is the set of named orm.DB instances. Applications inject it whenever
// they need to reach a data source other than the primary one.
//
// All read methods (Get/Has/Names/Kind/Primary) are safe for concurrent use.
// Register/Update/Unregister mutate the registry atomically; Reconcile is
// serialized so concurrent reconciles cannot fight each other.
type Registry interface {
	// Primary returns the orm.DB bound to the primary TOML configuration. It
	// is equivalent to Get(PrimaryName) but never returns an error.
	Primary() orm.DB

	// Get returns the orm.DB registered under name. It returns ErrNotFound when
	// no source is currently registered under name (including after it was
	// Unregister'd).
	Get(name string) (orm.DB, error)

	// Has reports whether name is currently registered and not closed.
	Has(name string) bool

	// Names returns every currently registered name (including primary) in
	// stable lexical order. Closed entries are excluded.
	Names() []string

	// Kind returns the dialect for the named data source. It returns ErrNotFound
	// for the same conditions as Get. Useful for sqlmigration / schema reflection
	// callers that need to branch on dialect without pulling out the full DB.
	Kind(name string) (config.DBKind, error)

	// Register opens a new data source and inserts it under name. It returns
	// ErrExists if the name is already registered, ErrPrimaryReserved if name
	// equals PrimaryName, and ErrNameInvalid if name is empty or contains
	// whitespace/control characters. The newly opened connection is closed and
	// not retained on conflict. Register never closes an existing connection, so
	// it takes no RegisterOption.
	Register(ctx context.Context, name string, cfg config.DataSourceConfig) (orm.DB, error)

	// Update atomically replaces the connection for an existing data source with
	// one opened from cfg. The new connection must Open and Ping successfully
	// before the swap; on failure the existing entry is left untouched. The old
	// underlying *sql.DB is closed asynchronously so callers that already hold a
	// DB reference observe no downtime.
	//
	// Update returns ErrNotFound when name is not registered and
	// ErrPrimaryReserved when name is PrimaryName.
	Update(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...RegisterOption) (orm.DB, error)

	// Unregister removes the named data source from the registry. Subsequent Get
	// calls return ErrNotFound. The underlying *sql.DB is closed asynchronously
	// (honoring WithCloseGrace), so callers already holding a DB reference can
	// finish in-flight queries before the connection pool tears down. Unregister
	// returns ErrPrimaryReserved for the primary source and ErrNotFound when name
	// is not registered.
	Unregister(ctx context.Context, name string, opts ...RegisterOption) error

	// Reconcile drives the registry toward the supplied desired set of
	// non-primary sources. The implementation computes three buckets:
	//
	//   - specs has name but registry does not   → Register
	//   - both have name but cfg differs         → Update
	//   - registry has name but specs does not   → Unregister
	//
	// Reconciles are serialized (not concurrent) so two ticks of a refresher job
	// never race. Per-name failures populate ReconcileReport.Errors and do not
	// interrupt processing of the remaining names. specs entries referencing the
	// primary name are ignored.
	Reconcile(ctx context.Context, specs []Spec, opts ...ReconcileOption) (ReconcileReport, error)

	// HealthCheck pings every registered source in parallel and returns a
	// name -> error map. A nil error indicates the source is reachable.
	HealthCheck(ctx context.Context) map[string]error
}
