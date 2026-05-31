package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"time"
	"unicode"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// registry is the datasource.Registry implementation backed by
// collections.ConcurrentMap. The primary entry is owned outside the map so
// callers can grab it on the fast path; every other entry lives inside the map.
// The map only ever holds live sources — Unregister and Shutdown remove entries
// outright — and the underlying *sql.DB is closed asynchronously so callers that
// still hold a DB reference can drain their in-flight queries.
type registry struct {
	entries collections.ConcurrentMap[string, *entry]
	primary *entry

	logger logx.Logger

	reconcileMu sync.Mutex

	closeWg   sync.WaitGroup
	closing   chan struct{}
	closeOnce sync.Once
}

// entry is immutable once stored: a source is mutated by replacing the whole
// entry (Update) or removing it (Unregister), never by editing it in place. That
// immutability is what lets the read methods run lock-free off the map. rawDB is
// the lifecycle handle (ping/close); the *bun.DB built while opening the source
// lives only inside ormDB, never as a separate field.
type entry struct {
	name  string
	cfg   config.DataSourceConfig
	rawDB *sql.DB
	ormDB orm.DB
}

// newRegistry constructs a registry seeded with the primary data source. The
// open is non-blocking — database/sql establishes no connection until first use
// — so a bad primary config fails fast here while the actual reachability ping is
// deferred to start-up, where it runs under the FX start timeout (see
// provideRegistry). Building the registry in the provide phase keeps the primary
// orm.DB available to the rest of the FX graph immediately.
func newRegistry(primaryCfg config.DataSourceConfig, logger logx.Logger) (*registry, error) {
	rawDB, ormDB, err := open(primaryCfg)
	if err != nil {
		return nil, fmt.Errorf("open primary data source: %w", err)
	}

	return fromEntry(&entry{
		name:  datasource.PrimaryName,
		cfg:   primaryCfg,
		rawDB: rawDB,
		ormDB: ormDB,
	}, logger), nil
}

// NewFromDB wraps an already-built orm.DB as the primary data source without
// re-opening it. It is intended for test harnesses (apptest) that want to share
// an existing connection across an FX app without paying the cost of a real
// Open/Ping dance. Production code should always go through the FX module. The
// caller supplies both the *sql.DB lifecycle handle and the orm.DB wrapper built
// over it, so datasource stays unaware of bun.
func NewFromDB(rawDB *sql.DB, primary orm.DB, cfg config.DataSourceConfig, logger logx.Logger) datasource.Registry {
	return fromEntry(&entry{
		name:  datasource.PrimaryName,
		cfg:   cfg,
		rawDB: rawDB,
		ormDB: primary,
	}, logger)
}

func fromEntry(primary *entry, logger logx.Logger) *registry {
	return &registry{
		entries: collections.NewConcurrentHashMap[string, *entry](),
		primary: primary,
		logger:  logger,
		closing: make(chan struct{}),
	}
}

// PrimaryRawDB exposes the raw *sql.DB for the primary source. It is used for
// boot-time version logging and by the schema reflection service.
func (r *registry) PrimaryRawDB() *sql.DB { return r.primary.rawDB }

// Primary returns the orm.DB for the primary data source. It never reports an
// error: the primary source is constructed during FX boot or the entire
// application fails to start.
func (r *registry) Primary() orm.DB { return r.primary.ormDB }

// Get implements datasource.Registry.Get.
func (r *registry) Get(name string) (orm.DB, error) {
	if name == datasource.PrimaryName {
		return r.primary.ormDB, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return nil, datasource.ErrNotFound
	}

	return e.ormDB, nil
}

// Has implements datasource.Registry.Has.
func (r *registry) Has(name string) bool {
	if name == datasource.PrimaryName {
		return true
	}

	_, ok := r.entries.Get(name)

	return ok
}

// Names implements datasource.Registry.Names. The returned slice always contains
// PrimaryName and is sorted lexically so callers can rely on a stable order for
// diagnostics and tests.
func (r *registry) Names() []string {
	keys := r.entries.Keys()
	out := make([]string, 0, len(keys)+1)
	out = append(out, datasource.PrimaryName)
	out = append(out, keys...)

	slices.Sort(out)

	return out
}

// Kind implements datasource.Registry.Kind.
func (r *registry) Kind(name string) (config.DBKind, error) {
	if name == datasource.PrimaryName {
		return r.primary.cfg.Kind, nil
	}

	e, ok := r.entries.Get(name)
	if !ok {
		return "", datasource.ErrNotFound
	}

	return e.cfg.Kind, nil
}

// openAndPing validates name, rejects the reserved primary name, and opens and
// pings a fresh connection — the prologue shared by Register and Update. The
// caller owns the returned *sql.DB and must close it if it decides not to keep
// it (e.g. on a name conflict).
func (*registry) openAndPing(ctx context.Context, name string, cfg config.DataSourceConfig) (*sql.DB, orm.DB, error) {
	if name == datasource.PrimaryName {
		return nil, nil, datasource.ErrPrimaryReserved
	}

	if err := validateName(name); err != nil {
		return nil, nil, err
	}

	rawDB, ormDB, err := open(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open data source %q: %w", name, err)
	}

	if err := rawDB.PingContext(ctx); err != nil {
		_ = rawDB.Close()

		return nil, nil, fmt.Errorf("ping data source %q: %w", name, err)
	}

	return rawDB, ormDB, nil
}

// Register implements datasource.Registry.Register.
func (r *registry) Register(ctx context.Context, name string, cfg config.DataSourceConfig) (orm.DB, error) {
	rawDB, ormDB, err := r.openAndPing(ctx, name, cfg)
	if err != nil {
		return nil, err
	}

	e := &entry{
		name:  name,
		cfg:   cfg,
		rawDB: rawDB,
		ormDB: ormDB,
	}

	if _, inserted := r.entries.PutIfAbsent(name, e); !inserted {
		_ = rawDB.Close()

		return nil, datasource.ErrExists
	}

	return e.ormDB, nil
}

// Update implements datasource.Registry.Update.
func (r *registry) Update(ctx context.Context, name string, cfg config.DataSourceConfig, opts ...datasource.RegisterOption) (orm.DB, error) {
	rawDB, ormDB, err := r.openAndPing(ctx, name, cfg)
	if err != nil {
		return nil, err
	}

	var (
		oldEntry *entry
		notFound bool
	)

	newEntry, _ := r.entries.Compute(name, func(_ string, prev *entry, exists bool) (*entry, bool) {
		if !exists {
			notFound = true

			return nil, false
		}

		oldEntry = prev

		return &entry{
			name:  name,
			cfg:   cfg,
			rawDB: rawDB,
			ormDB: ormDB,
		}, true
	})

	if notFound {
		_ = rawDB.Close()

		return nil, datasource.ErrNotFound
	}

	r.asyncClose(name, oldEntry.rawDB, applyOptions(opts))

	return newEntry.ormDB, nil
}

// Unregister implements datasource.Registry.Unregister. The entry is removed from
// the registry atomically; its underlying connection is closed asynchronously
// (honoring WithCloseGrace) so callers already holding a DB reference can finish
// in-flight queries.
func (r *registry) Unregister(_ context.Context, name string, opts ...datasource.RegisterOption) error {
	if name == datasource.PrimaryName {
		return datasource.ErrPrimaryReserved
	}

	e, ok := r.entries.Remove(name)
	if !ok {
		return datasource.ErrNotFound
	}

	r.asyncClose(name, e.rawDB, applyOptions(opts))

	return nil
}

// Reconcile implements datasource.Registry.Reconcile. Concurrent reconciles are
// serialized by a registry-wide mutex so two refresher ticks never interleave
// add/update/remove on the same name. Direct Register/Update/Unregister calls
// are NOT covered by that mutex; mixing them with a running Reconcile may leave
// the registry diverging from the reconciled set.
func (r *registry) Reconcile(ctx context.Context, specs []datasource.Spec, opts ...datasource.ReconcileOption) (datasource.ReconcileReport, error) {
	r.reconcileMu.Lock()
	defer r.reconcileMu.Unlock()

	ro := applyOptions(opts)

	desired := make(map[string]config.DataSourceConfig, len(specs))
	for _, s := range specs {
		if s.Name == "" || s.Name == datasource.PrimaryName {
			continue
		}

		desired[s.Name] = s.Config
	}

	current := make(map[string]config.DataSourceConfig, r.entries.Size())
	r.entries.ForEach(func(name string, e *entry) bool {
		current[name] = e.cfg

		return true
	})

	adds, updates, removes := diffReconcile(current, desired)

	report := datasource.ReconcileReport{}

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

// HealthCheck implements datasource.Registry.HealthCheck. Every source is pinged
// in parallel; the returned map contains an entry per source with a nil error
// when reachable.
func (r *registry) HealthCheck(ctx context.Context) map[string]error {
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
		record(datasource.PrimaryName, r.primary.rawDB.PingContext(ctx))
	})

	r.entries.ForEach(func(name string, e *entry) bool {
		db := e.rawDB
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
// live sources, drains the close goroutines (bounded by ctx), and finally closes
// the primary. The FX OnStop hook relies on this clean, ctx-bounded drain.
func (r *registry) Shutdown(ctx context.Context) error {
	r.signalClosing()

	var firstErr error

	for _, key := range r.entries.Keys() {
		e, ok := r.entries.Remove(key)
		if !ok {
			continue
		}

		if err := e.rawDB.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close data source %q: %w", key, err)
		}
	}

	if err := r.waitClose(ctx); err != nil && firstErr == nil {
		firstErr = err
	}

	if err := r.primary.rawDB.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close primary data source: %w", err)
	}

	return firstErr
}

func (r *registry) signalClosing() {
	r.closeOnce.Do(func() { close(r.closing) })
}

// waitClose blocks until every async-close goroutine finishes or ctx is done,
// whichever comes first. signalClosing has already woken any goroutine parked on
// its CloseGrace, so this normally returns promptly.
func (r *registry) waitClose(ctx context.Context) error {
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

func (r *registry) asyncClose(name string, db *sql.DB, opts datasource.RegisterOptions) {
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
// datasource.RegisterOption and datasource.ReconcileOption qualify.
func applyOptions[O any, F ~func(*O)](opts []F) O {
	var o O
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// validateName rejects empty names and names that carry whitespace or control
// characters. A data source name is both a registry key and a config selector,
// so it must be a clean single-token identifier.
func validateName(name string) error {
	if name == "" {
		return datasource.ErrNameInvalid
	}

	for _, c := range name {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return datasource.ErrNameInvalid
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
