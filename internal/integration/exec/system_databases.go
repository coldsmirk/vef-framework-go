package exec

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// systemSourcePrefix namespaces the registry entries the integration engine
// manages; the prefix is reserved and documented so static sources cannot
// collide.
const systemSourcePrefix = "itg:"

// systemDatabases lazily materializes each system's direct database connection
// in the datasource registry. Entries are keyed by the content hash of the
// stored definition, so a saved change re-registers on the next invocation —
// the same invalidation-free scheme the HTTP client cache uses.
type systemDatabases struct {
	registry datasource.Registry
	codec    *definition.SecretCodec

	mu      sync.Mutex
	sources map[string]*systemSource
}

// systemSource serializes registration per system: dialing one system's
// database (which can block for seconds when it is unreachable) must never
// stall invocations against other systems. Hash is the content hash of the
// registered definition, guarded by the same per-system lock.
type systemSource struct {
	mu   sync.Mutex
	hash string
}

func newSystemDatabases(registry datasource.Registry, codec *definition.SecretCodec) *systemDatabases {
	return &systemDatabases{registry: registry, codec: codec, sources: make(map[string]*systemSource)}
}

// sourceFor returns the per-system lock entry, creating it on first sight.
// Entries are never removed; the map is bounded by the set of system codes.
func (v *systemDatabases) sourceFor(code string) *systemSource {
	v.mu.Lock()
	defer v.mu.Unlock()

	source, ok := v.sources[code]
	if !ok {
		source = new(systemSource)
		v.sources[code] = source
	}

	return source
}

// DBFor returns the connection and dialect for system's data source,
// registering or updating the registry entry when the stored definition
// changed. Credential faults surface as API errors; connection faults are
// wrapped as transport errors for classification.
func (v *systemDatabases) DBFor(ctx context.Context, system *integration.System) (orm.DB, config.DBKind, error) {
	name := systemSourcePrefix + system.Code
	hash := dataSourceHash(system.DataSource)

	source := v.sourceFor(system.Code)
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.hash == hash && v.registry.Has(name) {
		// A Get miss here means the entry raced away (an unmanaged
		// Unregister) between Has and Get; falling through re-registers it.
		db, err := v.registry.Get(name)
		if err == nil {
			return db, system.DataSource.Kind, nil
		}
	}

	decrypted, err := v.codec.DecryptDataSource(system.DataSource)
	if err != nil {
		return nil, "", integration.ErrInvalidDataSource(err.Error())
	}

	cfg := decrypted.ToConfig()

	var db orm.DB
	if v.registry.Has(name) {
		db, err = v.registry.Update(ctx, name, cfg)
	} else {
		db, err = v.registry.Register(ctx, name, cfg)
	}

	if err != nil {
		return nil, "", &transportError{err: err}
	}

	source.hash = hash

	return db, cfg.Kind, nil
}

// Release drops the registry entry of a deleted system (or one whose data
// source was removed); the connection closes asynchronously per the
// registry's grace handling. Releasing an unknown system is a no-op. It
// holds the system's lock, so a concurrent DBFor either completes before the
// release or re-registers after it.
func (v *systemDatabases) Release(ctx context.Context, systemCode string) error {
	name := systemSourcePrefix + systemCode

	source := v.sourceFor(systemCode)
	source.mu.Lock()
	defer source.mu.Unlock()

	source.hash = ""

	if !v.registry.Has(name) {
		return nil
	}

	return v.registry.Unregister(ctx, name)
}

// Probe tests the data source with a throwaway connection, never touching
// the registry. A connection failure is data (Reachable=false); a credential
// fault is an error.
func (v *systemDatabases) Probe(ctx context.Context, ds *integration.DataSourceConfig) (*DatabaseProbe, error) {
	decrypted, err := v.codec.DecryptDataSource(ds)
	if err != nil {
		return nil, integration.ErrInvalidDataSource(err.Error())
	}

	start := time.Now()

	info, err := v.registry.TestConnection(ctx, decrypted.ToConfig())
	if err != nil {
		// A connection failure is the probe's answer, not an error of the
		// probe itself.
		return &DatabaseProbe{DurationMs: time.Since(start).Milliseconds(), Error: err.Error()}, nil //nolint:nilerr
	}

	return &DatabaseProbe{
		Reachable:  true,
		Version:    info.Version,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// dataSourceHash keys the ensured-registration cache by definition content;
// the password is hashed in its encrypted form.
func dataSourceHash(ds *integration.DataSourceConfig) string {
	payload, _ := json.Marshal(ds)

	return hashx.SHA256Bytes(payload)
}
