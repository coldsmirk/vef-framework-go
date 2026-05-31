package datasource

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
)

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
