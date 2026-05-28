package apptest_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	vef "github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// sqliteCfg makes a fresh SQLite file-based config under t.TempDir so the
// apptest harness can boot multiple isolated data sources at once.
func sqliteCfg(t *testing.T, name string) config.DataSourceConfig {
	t.Helper()

	return config.DataSourceConfig{
		Kind: config.SQLite,
		Path: filepath.Join(t.TempDir(), name+".db"),
	}
}

// TestDataSourcesPrimaryInjectsAsDB asserts that the framework still injects
// orm.DB (the primary source) for the common case so existing handlers and
// resources keep working without referencing DataSources directly.
func TestDataSourcesPrimaryInjectsAsDB(t *testing.T) {
	var primary orm.DB

	_, stop := apptest.NewTestApp(t,
		apptest.WithDataSourcesConfig(&config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{
				"primary": sqliteCfg(t, "primary"),
			},
		}),
		fx.Populate(&primary),
	)
	t.Cleanup(stop)

	require.NotNil(t, primary, "primary orm.DB must be available")

	var hit int
	require.NoError(t, primary.NewRaw("SELECT 1").Scan(context.Background(), &hit))
	require.Equal(t, 1, hit)
}

// TestDataSourcesStaticRegistration boots an app with two static sources and
// verifies the secondary entry is reachable via orm.DataSources.Get.
func TestDataSourcesStaticRegistration(t *testing.T) {
	var sources orm.DataSources

	_, stop := apptest.NewTestApp(t,
		apptest.WithDataSourcesConfig(&config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{
				"primary":   sqliteCfg(t, "primary"),
				"analytics": sqliteCfg(t, "analytics"),
			},
		}),
		fx.Populate(&sources),
	)
	t.Cleanup(stop)

	require.True(t, sources.Has("analytics"), "analytics source should be present")

	analytics, err := sources.Get("analytics")
	require.NoError(t, err, "Get(analytics) should succeed")

	var hit int
	require.NoError(t, analytics.NewRaw("SELECT 1").Scan(context.Background(), &hit))
	require.Equal(t, 1, hit)

	require.ElementsMatch(t, []string{"analytics", "primary"}, sources.Names())
}

// fakeProvider feeds the framework a single extra spec, simulating the
// "tenant table" use case where additional sources live outside the TOML.
type fakeProvider struct {
	spec orm.DataSourceSpec
}

func (*fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) Load(_ context.Context) ([]orm.DataSourceSpec, error) {
	return []orm.DataSourceSpec{p.spec}, nil
}

// TestDataSourceProviderRegistersAdditionalSource verifies that a custom
// DataSourceProvider attached via vef.ProvideDataSourceProvider populates
// the registry during boot.
func TestDataSourceProviderRegistersAdditionalSource(t *testing.T) {
	cfg := sqliteCfg(t, "tenant1")

	var sources orm.DataSources

	_, stop := apptest.NewTestApp(t,
		apptest.WithDataSourcesConfig(&config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{"primary": sqliteCfg(t, "primary")},
		}),
		vef.ProvideDataSourceProvider(func() orm.DataSourceProvider {
			return &fakeProvider{spec: orm.DataSourceSpec{Name: "tenant1", Cfg: cfg}}
		}),
		fx.Populate(&sources),
	)
	t.Cleanup(stop)

	require.True(t, sources.Has("tenant1"), "tenant1 should be present after provider Load")

	tenant, err := sources.Get("tenant1")
	require.NoError(t, err)

	var v int
	require.NoError(t, tenant.NewRaw("SELECT 1").Scan(context.Background(), &v))
	require.Equal(t, 1, v)
}

// TestDataSourcesReconcileCoversAllThreeTransitions exercises the periodic
// sync use case: the user maintains a desired set of non-primary sources,
// calls Reconcile, and the registry adds/updates/removes accordingly.
func TestDataSourcesReconcileCoversAllThreeTransitions(t *testing.T) {
	var sources orm.DataSources

	keepCfg := sqliteCfg(t, "keep")
	updateOldCfg := sqliteCfg(t, "u-old")
	updateNewCfg := sqliteCfg(t, "u-new")
	removeCfg := sqliteCfg(t, "rm")
	freshCfg := sqliteCfg(t, "fresh")

	_, stop := apptest.NewTestApp(t,
		apptest.WithDataSourcesConfig(&config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{
				"primary": sqliteCfg(t, "primary"),
				"keep":    keepCfg,
				"tenant":  updateOldCfg,
				"remove":  removeCfg,
			},
		}),
		fx.Populate(&sources),
	)
	t.Cleanup(stop)

	require.True(t, sources.Has("keep"))
	require.True(t, sources.Has("tenant"))
	require.True(t, sources.Has("remove"))

	specs := []orm.DataSourceSpec{
		{Name: "keep", Cfg: keepCfg},
		{Name: "tenant", Cfg: updateNewCfg},
		{Name: "fresh", Cfg: freshCfg},
	}

	report, err := sources.Reconcile(context.Background(), specs)
	require.NoError(t, err, "Reconcile should not return a top-level error")
	require.Equal(t, []string{"fresh"}, report.Added)
	require.Equal(t, []string{"tenant"}, report.Updated)
	require.Equal(t, []string{"remove"}, report.Removed)
	require.Nil(t, report.Errors)

	require.True(t, sources.Has("fresh"))
	require.True(t, sources.Has("tenant"))
	require.False(t, sources.Has("remove"))

	fresh, err := sources.Get("fresh")
	require.NoError(t, err)

	var v int
	require.NoError(t, fresh.NewRaw("SELECT 1").Scan(context.Background(), &v))
	require.Equal(t, 1, v)
}

// TestDataSourcesPrimaryReservedFromRuntime guards the soft contract that
// Register/Update/Unregister all refuse the "primary" name. Runtime callers
// cannot accidentally replace the FX-injected primary connection.
func TestDataSourcesPrimaryReservedFromRuntime(t *testing.T) {
	var sources orm.DataSources

	_, stop := apptest.NewTestApp(t,
		apptest.WithDataSourcesConfig(&config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{"primary": sqliteCfg(t, "primary")},
		}),
		fx.Populate(&sources),
	)
	t.Cleanup(stop)

	ctx := context.Background()
	_, err := sources.Register(ctx, orm.PrimaryDataSourceName, sqliteCfg(t, "ghost"))
	require.ErrorIs(t, err, orm.ErrPrimaryReserved)

	_, err = sources.Update(ctx, orm.PrimaryDataSourceName, sqliteCfg(t, "ghost"))
	require.ErrorIs(t, err, orm.ErrPrimaryReserved)

	require.ErrorIs(t, sources.Unregister(ctx, orm.PrimaryDataSourceName), orm.ErrPrimaryReserved)
}
