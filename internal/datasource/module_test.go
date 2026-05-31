package datasource

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
)

type TestProvider struct {
	name  string
	specs []datasource.Spec
	err   error
}

func (p TestProvider) Name() string {
	return p.name
}

func (p TestProvider) Load(context.Context) ([]datasource.Spec, error) {
	return p.specs, p.err
}

// TestProvideRegistryClosesPrimaryOnSeedFailure pins the lifecycle contract that
// a mid-start failure (here, a static source with an unsupported dialect) still
// drains the already-opened primary. Fx only runs a hook's OnStop when that same
// hook's OnStart succeeded, so the Shutdown hook must stay separate from the
// fallible seed/provider start work — otherwise the primary connection leaks.
// This test fails if the two are folded back into a single hook.
func TestProvideRegistryClosesPrimaryOnSeedFailure(t *testing.T) {
	ctx := context.Background()
	lc := fxtest.NewLifecycle(t)

	cfg := &config.DataSourcesConfig{
		Map: map[string]config.DataSourceConfig{
			datasource.PrimaryName: {Kind: config.SQLite},
			"broken":               {Kind: "no-such-dialect"},
		},
	}

	out, err := provideRegistry(lc, cfg, ProviderParams{})
	require.NoError(t, err, "ProvideRegistry should open the primary in the provide phase")
	require.NotNil(t, out.RawDB, "Primary raw *sql.DB should be exposed")
	require.NoError(t, out.RawDB.PingContext(ctx), "Primary should be healthy before start")

	require.Error(t, lc.Start(ctx),
		"Start should fail when a static source has an unsupported dialect")

	require.NoError(t, lc.Stop(ctx), "Stop should drain cleanly")

	require.Error(t, out.RawDB.PingContext(ctx),
		"Primary connection should be closed by the Shutdown hook despite the seed failure")
}

// TestProvideRegistryDefersPrimaryPingToStart pins that the provide phase does
// not ping the primary: database/sql opens lazily, so a primary that opens but is
// unreachable still builds the graph, and the reachability check happens in the
// OnStart hook (where it is bounded by the FX start timeout). This fails if the
// ping is moved back into the provide phase.
func TestProvideRegistryDefersPrimaryPingToStart(t *testing.T) {
	ctx := context.Background()
	lc := fxtest.NewLifecycle(t)

	// A SQLite path under a missing directory: sql.Open succeeds (lazy) but the
	// first connection fails because the parent directory does not exist.
	cfg := &config.DataSourcesConfig{
		Map: map[string]config.DataSourceConfig{
			datasource.PrimaryName: {
				Kind: config.SQLite,
				Path: filepath.Join(t.TempDir(), "missing-dir", "primary.db"),
			},
		},
	}

	out, err := provideRegistry(lc, cfg, ProviderParams{})
	require.NoError(t, err, "Provide phase should open lazily and avoid pinging the primary")
	require.NotNil(t, out.RawDB, "Primary raw *sql.DB should be exposed")

	require.Error(t, lc.Start(ctx),
		"Start should fail when the unreachable primary is pinged in the OnStart hook")

	require.NoError(t, lc.Stop(ctx), "Stop should drain cleanly")
}

func TestRunProviders(t *testing.T) {
	ctx := context.Background()
	loadErr := errors.New("tenant catalog failed")

	tests := []struct {
		name      string
		provider  datasource.Provider
		assertion func(t *testing.T, r *registry, err error)
	}{
		{
			name: "Load error",
			provider: TestProvider{
				name: "tenant-catalog",
				err:  loadErr,
			},
			assertion: func(t *testing.T, _ *registry, err error) {
				require.ErrorIs(t, err, loadErr, "Provider load failure should wrap the provider error")
				require.Contains(t, err.Error(), `data source provider "tenant-catalog"`,
					"Provider load failure should identify the provider name")
			},
		},
		{
			name: "Register specs",
			provider: TestProvider{
				name: "tenant-catalog",
				specs: []datasource.Spec{
					{Name: "tenant", Config: newSQLiteCfg(t, "provider-tenant")},
					{Name: "analytics", Config: newSQLiteCfg(t, "provider-analytics")},
				},
			},
			assertion: func(t *testing.T, r *registry, err error) {
				require.NoError(t, err, "Provider specs should register without error")
				require.True(t, r.Has("tenant"), "Provider spec should register the tenant data source")
				require.True(t, r.Has("analytics"), "Provider spec should register the analytics data source")

				tenantKind, err := r.Kind("tenant")
				require.NoError(t, err, "Provider tenant source kind lookup should succeed")
				require.Equal(t, config.SQLite, tenantKind, "Provider tenant source should preserve the configured kind")
			},
		},
		{
			name: "Register duplicate spec name",
			provider: TestProvider{
				name: "tenant-catalog",
				specs: []datasource.Spec{
					{Name: "duplicate", Config: newSQLiteCfg(t, "provider-duplicate-first")},
					{Name: "duplicate", Config: newSQLiteCfg(t, "provider-duplicate-second")},
					{Name: "after-conflict", Config: newSQLiteCfg(t, "provider-after-conflict")},
				},
			},
			assertion: func(t *testing.T, r *registry, err error) {
				require.ErrorIs(t, err, datasource.ErrExists, "Duplicate provider spec should propagate the registry conflict")
				require.Contains(t, err.Error(), `register tenant-catalog data source "duplicate"`,
					"Duplicate provider spec error should identify the provider source")
				require.True(t, r.Has("duplicate"), "First duplicate provider spec should remain registered before the conflict")
				require.False(t, r.Has("after-conflict"), "Provider specs after a conflict should not be registered")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)

			err := runProviders(ctx, r, []datasource.Provider{tt.provider})
			tt.assertion(t, r, err)
		})
	}
}
