package orm

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"time"
	"unicode"

	"github.com/coldsmirk/go-collections"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Registry is the DataSources implementation backed by collections.ConcurrentMap.
// The primary entry is owned outside the map so callers can grab it on the fast
// path; every other entry lives inside the map. The map only ever holds live
// sources — Unregister and Shutdown remove entries outright — and the underlying
// *sql.DB is closed asynchronously so callers that still hold a DB reference can
// drain their in-flight queries.
type Registry struct {
	entries collections.ConcurrentMap[string, *registryEntry]
	primary *registryEntry

	logger logx.Logger

	reconcileMu sync.Mutex

	closeWg   sync.WaitGroup
	closing   chan struct{}
	closeOnce sync.Once
}

// registryEntry is immutable once stored: a source is mutated by replacing the
// whole entry (Update) or removing it (Unregister), never by editing it in place.
// That immutability is what lets the read methods run lock-free off the map.
// sqlDB is the lifecycle handle (ping/close); the *bun.DB built while opening the
// source lives only inside ormDB, never as a separate field.
type registryEntry struct {
	name  string
	cfg   config.DataSourceConfig
	sqlDB *sql.DB
	ormDB DB
}

// NewRegistry constructs a Registry seeded with the primary data source. The
// primary is opened, Pinged, and on failure the constructor returns an error so
// the FX boot can fail-fast.
func NewRegistry(ctx context.Context, primaryCfg config.DataSourceConfig, logger logx.Logger) (*Registry, error) {
	sqlDB, ormDB, err := openDataSource(primaryCfg)
	if err != nil {
		return nil, fmt.Errorf("open primary data source: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf("ping primary data source: %w", err)
	}

	return newRegistryFromEntry(&registryEntry{
		name:  PrimaryDataSourceName,
		cfg:   primaryCfg,
		sqlDB: sqlDB,
		ormDB: ormDB,
	}, logger), nil
}

// NewRegistryFromBunDB wraps an already-opened *bun.DB as the primary data
// source without re-opening it. It is intended for test harnesses
// (apptest.SetupAppWithDBConfig) that want to share an existing connection
// across an FX app without paying the cost of a real Open/Ping dance.
// Production code should always use NewRegistry. The *sql.DB lifecycle handle is
// taken from the supplied *bun.DB; the bun.DB itself is retained only inside the
// derived orm.DB.
func NewRegistryFromBunDB(primary *bun.DB, cfg config.DataSourceConfig, logger logx.Logger) *Registry {
	return newRegistryFromEntry(&registryEntry{
		name:  PrimaryDataSourceName,
		cfg:   cfg,
		sqlDB: primary.DB,
		ormDB: New(primary),
	}, logger)
}

func newRegistryFromEntry(primary *registryEntry, logger logx.Logger) *Registry {
	return &Registry{
		entries: collections.NewConcurrentHashMap[string, *registryEntry](),
		primary: primary,
		logger:  logger,
		closing: make(chan struct{}),
	}
}

// PrimarySQLDB exposes the raw *sql.DB for the primary source. It is used for
// boot-time version logging and by the schema reflection service.
func (r *Registry) PrimarySQLDB() *sql.DB { return r.primary.sqlDB }

// Primary returns the orm.DB for the primary data source. It never reports
// an error: the primary source is constructed during FX boot or the entire
// application fails to start.
func (r *Registry) Primary() DB { return r.primary.ormDB }

// Get implements DataSources.Get.
func (r *Registry) Get(name string) (DB, error) {
	if name == PrimaryDataSourceName {
		return r.primary.ormDB, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return nil, ErrDataSourceNotFound
	}

	return e.ormDB, nil
}

// Has implements DataSources.Has.
func (r *Registry) Has(name string) bool {
	if name == PrimaryDataSourceName {
		return true
	}

	_, ok := r.entries.Get(name)

	return ok
}

// Names implements DataSources.Names. The returned slice always contains
// PrimaryDataSourceName and is sorted lexically so callers can rely on a
// stable order for diagnostics and tests.
func (r *Registry) Names() []string {
	keys := r.entries.Keys()
	out := make([]string, 0, len(keys)+1)
	out = append(out, PrimaryDataSourceName)
	out = append(out, keys...)

	slices.Sort(out)

	return out
}

// Kind implements DataSources.Kind.
func (r *Registry) Kind(name string) (config.DBKind, error) {
	if name == PrimaryDataSourceName {
		return r.primary.cfg.Kind, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return "", ErrDataSourceNotFound
	}

	return e.cfg.Kind, nil
}

// openAndPing validates name, rejects the reserved primary name, and opens and
// pings a fresh connection — the prologue shared by Register and Update. The
// caller owns the returned *sql.DB and must close it if it decides not to keep
// it (e.g. on a name conflict).
func (*Registry) openAndPing(ctx context.Context, name string, cfg config.DataSourceConfig) (*sql.DB, DB, error) {
	if name == PrimaryDataSourceName {
		return nil, nil, ErrPrimaryReserved
	}

	if err := validateDataSourceName(name); err != nil {
		return nil, nil, err
	}

	sqlDB, ormDB, err := openDataSource(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open data source %q: %w", name, err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()

		return nil, nil, fmt.Errorf("ping data source %q: %w", name, err)
	}

	return sqlDB, ormDB, nil
}

// Register implements DataSources.Register.
func (r *Registry) Register(ctx context.Context, name string, cfg config.DataSourceConfig) (DB, error) {
	sqlDB, ormDB, err := r.openAndPing(ctx, name, cfg)
	if err != nil {
		return nil, err
	}

	entry := &registryEntry{
		name:  name,
		cfg:   cfg,
		sqlDB: sqlDB,
		ormDB: ormDB,
	}

	if _, inserted := r.entries.PutIfAbsent(name, entry); !inserted {
		_ = sqlDB.Close()

		return nil, ErrDataSourceExists
	}

	return entry.ormDB, nil
}

// Update implements DataSources.Update.
func (r *Registry) Update(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...RegisterOption) (DB, error) {
	sqlDB, ormDB, err := r.openAndPing(ctx, name, cfg)
	if err != nil {
		return nil, err
	}

	var (
		oldEntry *registryEntry
		notFound bool
	)

	newEntry, _ := r.entries.Compute(name, func(_ string, prev *registryEntry, exists bool) (*registryEntry, bool) {
		if !exists {
			notFound = true

			return nil, false
		}

		oldEntry = prev

		return &registryEntry{
			name:  name,
			cfg:   cfg,
			sqlDB: sqlDB,
			ormDB: ormDB,
		}, true
	})

	if notFound {
		_ = sqlDB.Close()

		return nil, ErrDataSourceNotFound
	}

	r.asyncClose(name, oldEntry.sqlDB, applyOptions(opts))

	return newEntry.ormDB, nil
}

// Unregister implements DataSources.Unregister. The entry is removed from the
// registry atomically; its underlying connection is closed asynchronously
// (honoring WithCloseGrace) so callers already holding a DB reference can
// finish in-flight queries.
func (r *Registry) Unregister(_ context.Context, name string, opts ...RegisterOption) error {
	if name == PrimaryDataSourceName {
		return ErrPrimaryReserved
	}

	e, ok := r.entries.Remove(name)
	if !ok {
		return ErrDataSourceNotFound
	}

	r.asyncClose(name, e.sqlDB, applyOptions(opts))

	return nil
}

// Reconcile implements DataSources.Reconcile. Concurrent reconciles are
// serialized by a registry-wide mutex so two refresher ticks never interleave
// add/update/remove on the same name. Direct Register/Update/Unregister calls
// are NOT covered by that mutex; mixing them with a running Reconcile may leave
// the registry diverging from the reconciled set.
func (r *Registry) Reconcile(ctx context.Context, specs []DataSourceSpec, opts ...ReconcileOption) (ReconcileReport, error) {
	r.reconcileMu.Lock()
	defer r.reconcileMu.Unlock()

	ro := applyOptions(opts)

	desired := make(map[string]config.DataSourceConfig, len(specs))
	for _, s := range specs {
		if s.Name == "" || s.Name == PrimaryDataSourceName {
			continue
		}

		desired[s.Name] = s.Cfg
	}

	current := make(map[string]config.DataSourceConfig, r.entries.Size())
	r.entries.ForEach(func(name string, e *registryEntry) bool {
		current[name] = e.cfg

		return true
	})

	adds, updates, removes := diffReconcile(current, desired)

	report := ReconcileReport{}

	if ro.DryRun {
		report.Added = adds
		report.Updated = updates
		report.Removed = removes

		return report, nil
	}

	errs := map[string]error{}

	for _, name := range adds {
		if _, err := r.Register(ctx, name, desired[name]); err != nil {
			errs[name] = err

			continue
		}

		report.Added = append(report.Added, name)
	}

	for _, name := range updates {
		if _, err := r.Update(ctx, name, desired[name]); err != nil {
			errs[name] = err

			continue
		}

		report.Updated = append(report.Updated, name)
	}

	for _, name := range removes {
		if err := r.Unregister(ctx, name); err != nil {
			errs[name] = err

			continue
		}

		report.Removed = append(report.Removed, name)
	}

	if len(errs) > 0 {
		report.Errors = errs
	}

	return report, nil
}

// HealthCheck implements DataSources.HealthCheck. Every source is pinged in
// parallel; the returned map contains an entry per source with a nil error
// when reachable.
func (r *Registry) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	record := func(name string, err error) {
		mu.Lock()
		results[name] = err
		mu.Unlock()
	}

	wg.Go(func() {
		record(PrimaryDataSourceName, r.primary.sqlDB.PingContext(ctx))
	})

	r.entries.ForEach(func(name string, e *registryEntry) bool {
		db := e.sqlDB
		wg.Go(func() {
			record(name, db.PingContext(ctx))
		})

		return true
	})

	wg.Wait()

	return results
}

// Shutdown closes every registered data source. It first signals any pending
// async-close goroutines to stop waiting on their CloseGrace, then closes the
// live sources, drains the close goroutines (bounded by ctx), and finally
// closes the primary. The FX OnStop hook relies on this clean, ctx-bounded
// drain.
func (r *Registry) Shutdown(ctx context.Context) error {
	r.signalClosing()

	var firstErr error

	for _, k := range r.entries.Keys() {
		e, ok := r.entries.Remove(k)
		if !ok {
			continue
		}

		if err := e.sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close data source %q: %w", k, err)
		}
	}

	if err := r.waitClose(ctx); err != nil && firstErr == nil {
		firstErr = err
	}

	if err := r.primary.sqlDB.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close primary data source: %w", err)
	}

	return firstErr
}

func (r *Registry) signalClosing() {
	r.closeOnce.Do(func() { close(r.closing) })
}

// waitClose blocks until every async-close goroutine finishes or ctx is done,
// whichever comes first. signalClosing has already woken any goroutine parked
// on its CloseGrace, so this normally returns promptly.
func (r *Registry) waitClose(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		r.closeWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Registry) asyncClose(name string, db *sql.DB, opts RegisterOptions) {
	r.closeWg.Go(func() {
		if opts.CloseGrace > 0 {
			select {
			case <-time.After(opts.CloseGrace):
			case <-r.closing:
			}
		}

		if err := db.Close(); err != nil && r.logger != nil {
			r.logger.Warnf("close data source %q: %v", name, err)
		}
	})
}

// applyOptions folds a slice of functional options into a fresh options value.
// It works for any option family whose underlying type is func(*O) — both
// RegisterOption and ReconcileOption qualify.
func applyOptions[O any, F ~func(*O)](opts []F) O {
	var o O
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// validateDataSourceName rejects empty names and names that carry whitespace or
// control characters. A data source name is both a registry key and a config
// selector, so it must be a clean single-token identifier.
func validateDataSourceName(name string) error {
	if name == "" {
		return ErrDataSourceNameInvalid
	}

	for _, c := range name {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return ErrDataSourceNameInvalid
		}
	}

	return nil
}

func diffReconcile(current, desired map[string]config.DataSourceConfig) (adds, updates, removes []string) {
	for name, cfg := range desired {
		if cur, ok := current[name]; !ok {
			adds = append(adds, name)
		} else if cur != cfg {
			updates = append(updates, name)
		}
	}

	for name := range current {
		if _, ok := desired[name]; !ok {
			removes = append(removes, name)
		}
	}

	slices.Sort(adds)
	slices.Sort(updates)
	slices.Sort(removes)

	return adds, updates, removes
}
