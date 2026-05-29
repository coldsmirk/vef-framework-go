package database

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uptrace/bun"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Registry is the orm.DataSources implementation backed by collections.ConcurrentMap.
// The primary entry is owned outside the map so callers can grab it on the fast
// path; every other entry lives inside the map and is mutated atomically.
type Registry struct {
	entries collections.ConcurrentMap[string, *registryEntry]
	primary *registryEntry

	logger logx.Logger
	dbOpts []Option

	reconcileMu sync.Mutex
	closeWg     sync.WaitGroup
}

type registryEntry struct {
	name   string
	cfg    config.DataSourceConfig
	bunDB  *bun.DB
	ormDB  orm.DB
	closed atomic.Bool
}

// NewRegistry constructs a Registry seeded with the primary data source. The
// primary is opened, Pinged, and on failure the constructor returns an error
// so the FX boot can fail-fast. dbOpts are applied to every Open call
// (primary and dynamically registered sources alike) so per-source overrides
// stay opt-in through RegisterOption.
func NewRegistry(ctx context.Context, primaryCfg config.DataSourceConfig, logger logx.Logger, dbOpts ...Option) (*Registry, error) {
	bunDB, err := Open(orm.PrimaryDataSourceName, primaryCfg, dbOpts...)
	if err != nil {
		return nil, fmt.Errorf("open primary data source: %w", err)
	}

	if err := bunDB.PingContext(ctx); err != nil {
		_ = bunDB.Close()

		return nil, fmt.Errorf("ping primary data source: %w", err)
	}

	return newRegistryFromEntry(&registryEntry{
		name:  orm.PrimaryDataSourceName,
		cfg:   primaryCfg,
		bunDB: bunDB,
		ormDB: orm.New(bunDB),
	}, logger, dbOpts), nil
}

// NewRegistryFromBunDB wraps an already-opened *bun.DB as the primary data
// source without re-opening it. It is intended for test harnesses
// (apptest.SetupAppWithDBConfig) that want to share an existing connection
// across an FX app without paying the cost of a real Open/Ping dance.
// Production code should always use NewRegistry.
func NewRegistryFromBunDB(primary *bun.DB, cfg config.DataSourceConfig, logger logx.Logger) *Registry {
	return newRegistryFromEntry(&registryEntry{
		name:  orm.PrimaryDataSourceName,
		cfg:   cfg,
		bunDB: primary,
		ormDB: orm.New(primary),
	}, logger, nil)
}

func newRegistryFromEntry(primary *registryEntry, logger logx.Logger, dbOpts []Option) *Registry {
	return &Registry{
		entries: collections.NewConcurrentHashMap[string, *registryEntry](),
		primary: primary,
		logger:  logger,
		dbOpts:  dbOpts,
	}
}

// PrimaryBunDB exposes the underlying *bun.DB for the primary source. It is
// kept package-private to FX wiring and to the schema reflection service,
// which still needs a raw *sql.DB handle.
func (r *Registry) PrimaryBunDB() *bun.DB { return r.primary.bunDB }

// Primary returns the orm.DB for the primary data source. It never reports
// an error: the primary source is constructed during FX boot or the entire
// application fails to start.
func (r *Registry) Primary() orm.DB { return r.primary.ormDB }

// Get implements orm.DataSources.Get.
func (r *Registry) Get(name string) (orm.DB, error) {
	if name == orm.PrimaryDataSourceName {
		return r.primary.ormDB, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return nil, orm.ErrDataSourceNotFound
	}

	if e.closed.Load() {
		return nil, orm.ErrDataSourceClosed
	}

	return e.ormDB, nil
}

// Has implements orm.DataSources.Has.
func (r *Registry) Has(name string) bool {
	if name == orm.PrimaryDataSourceName {
		return true
	}

	e, ok := r.entries.Get(name)

	return ok && !e.closed.Load()
}

// Names implements orm.DataSources.Names. The returned slice always contains
// PrimaryDataSourceName and is sorted lexically so callers can rely on a
// stable order for diagnostics and tests.
func (r *Registry) Names() []string {
	keys := r.entries.Keys()
	out := make([]string, 0, len(keys)+1)
	out = append(out, orm.PrimaryDataSourceName)

	for _, k := range keys {
		if e, ok := r.entries.Get(k); ok && !e.closed.Load() {
			out = append(out, k)
		}
	}

	slices.Sort(out)

	return out
}

// Kind implements orm.DataSources.Kind.
func (r *Registry) Kind(name string) (config.DBKind, error) {
	if name == orm.PrimaryDataSourceName {
		return r.primary.cfg.Kind, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return "", orm.ErrDataSourceNotFound
	}

	if e.closed.Load() {
		return "", orm.ErrDataSourceClosed
	}

	return e.cfg.Kind, nil
}

// Register implements orm.DataSources.Register.
func (r *Registry) Register(ctx context.Context, name string, cfg config.DataSourceConfig, _ ...orm.RegisterOption) (orm.DB, error) {
	if err := validateDataSourceName(name); err != nil {
		return nil, err
	}

	if name == orm.PrimaryDataSourceName {
		return nil, orm.ErrPrimaryReserved
	}

	bunDB, err := Open(name, cfg, r.dbOpts...)
	if err != nil {
		return nil, fmt.Errorf("open data source %q: %w", name, err)
	}

	if err := bunDB.PingContext(ctx); err != nil {
		_ = bunDB.Close()

		return nil, fmt.Errorf("ping data source %q: %w", name, err)
	}

	entry := &registryEntry{
		name:  name,
		cfg:   cfg,
		bunDB: bunDB,
		ormDB: orm.New(bunDB),
	}

	var existsOpen bool

	stored, _ := r.entries.Compute(name, func(_ string, prev *registryEntry, exists bool) (*registryEntry, bool) {
		if exists && !prev.closed.Load() {
			existsOpen = true

			return prev, true
		}

		return entry, true
	})

	if existsOpen {
		_ = bunDB.Close()

		return nil, orm.ErrDataSourceExists
	}

	return stored.ormDB, nil
}

// Update implements orm.DataSources.Update.
func (r *Registry) Update(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...orm.RegisterOption) (orm.DB, error) {
	if name == orm.PrimaryDataSourceName {
		return nil, orm.ErrPrimaryReserved
	}

	if err := validateDataSourceName(name); err != nil {
		return nil, err
	}

	bunDB, err := Open(name, cfg, r.dbOpts...)
	if err != nil {
		return nil, fmt.Errorf("open data source %q: %w", name, err)
	}

	if err := bunDB.PingContext(ctx); err != nil {
		_ = bunDB.Close()

		return nil, fmt.Errorf("ping data source %q: %w", name, err)
	}

	var (
		oldEntry *registryEntry
		notFound bool
		closed   bool
	)

	newEntry, _ := r.entries.Compute(name, func(_ string, prev *registryEntry, exists bool) (*registryEntry, bool) {
		if !exists {
			notFound = true

			return nil, false
		}

		if prev.closed.Load() {
			closed = true

			return prev, true
		}

		oldEntry = prev

		return &registryEntry{
			name:  name,
			cfg:   cfg,
			bunDB: bunDB,
			ormDB: orm.New(bunDB),
		}, true
	})

	if notFound {
		_ = bunDB.Close()

		return nil, orm.ErrDataSourceNotFound
	}

	if closed {
		_ = bunDB.Close()

		return nil, orm.ErrDataSourceClosed
	}

	oldEntry.closed.Store(true)
	r.asyncClose(name, oldEntry.bunDB, applyRegisterOptions(opts))

	return newEntry.ormDB, nil
}

// Unregister implements orm.DataSources.Unregister.
func (r *Registry) Unregister(ctx context.Context, name string) error {
	if name == orm.PrimaryDataSourceName {
		return orm.ErrPrimaryReserved
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return orm.ErrDataSourceNotFound
	}

	if e.closed.Swap(true) {
		return orm.ErrDataSourceClosed
	}

	r.asyncClose(name, e.bunDB, orm.RegisterOptions{})

	_ = ctx

	return nil
}

// Reconcile implements orm.DataSources.Reconcile. The registry-wide
// reconcile mutex serializes concurrent reconciles so they cannot interleave
// add/update/remove on the same name.
func (r *Registry) Reconcile(ctx context.Context, specs []orm.DataSourceSpec, opts ...orm.ReconcileOption) (orm.ReconcileReport, error) {
	r.reconcileMu.Lock()
	defer r.reconcileMu.Unlock()

	var ro orm.ReconcileOptions
	for _, opt := range opts {
		opt(&ro)
	}

	desired := make(map[string]config.DataSourceConfig, len(specs))
	for _, s := range specs {
		if s.Name == "" || s.Name == orm.PrimaryDataSourceName {
			continue
		}

		desired[s.Name] = s.Cfg
	}

	currentKeys := r.entries.Keys()
	current := make(map[string]config.DataSourceConfig, len(currentKeys))

	for _, k := range currentKeys {
		if e, ok := r.entries.Get(k); ok && !e.closed.Load() {
			current[k] = e.cfg
		}
	}

	adds, updates, removes := diffReconcile(current, desired)

	report := orm.ReconcileReport{}

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

// HealthCheck implements orm.DataSources.HealthCheck. Every source is pinged
// in parallel; the returned map contains an entry per source with a nil error
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
		record(orm.PrimaryDataSourceName, r.primary.bunDB.PingContext(ctx))
	})

	for _, k := range r.entries.Keys() {
		e, ok := r.entries.Get(k)
		if !ok || e.closed.Load() {
			continue
		}

		name, db := k, e.bunDB
		wg.Go(func() {
			record(name, db.PingContext(ctx))
		})
	}

	wg.Wait()

	return results
}

// Shutdown closes every registered data source. It waits for any in-flight
// async-close goroutines (e.g. those queued by Update/Unregister with a
// CloseGrace) so the FX OnStop hook can rely on a clean drain.
func (r *Registry) Shutdown(ctx context.Context) error {
	var firstErr error

	for _, k := range r.entries.Keys() {
		e, ok := r.entries.Remove(k)
		if !ok {
			continue
		}

		if e.closed.Swap(true) {
			continue
		}

		if err := e.bunDB.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close data source %q: %w", k, err)
		}
	}

	r.closeWg.Wait()

	if err := r.primary.bunDB.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close primary data source: %w", err)
	}

	_ = ctx

	return firstErr
}

func (r *Registry) asyncClose(name string, db *bun.DB, opts orm.RegisterOptions) {
	r.closeWg.Go(func() {
		if opts.CloseGrace > 0 {
			time.Sleep(opts.CloseGrace)
		}

		if err := db.Close(); err != nil && r.logger != nil {
			r.logger.Warnf("close data source %q: %v", name, err)
		}
	})
}

func applyRegisterOptions(opts []orm.RegisterOption) orm.RegisterOptions {
	var o orm.RegisterOptions
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

func validateDataSourceName(name string) error {
	if name == "" {
		return orm.ErrDataSourceNameInvalid
	}

	return nil
}

func diffReconcile(current, desired map[string]config.DataSourceConfig) (adds, updates, removes []string) {
	for name, cfg := range desired {
		if cur, ok := current[name]; !ok {
			adds = append(adds, name)
		} else if !reflect.DeepEqual(cur, cfg) {
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
