package js_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	internaljs "github.com/coldsmirk/vef-framework-go/internal/js"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// StubLogger is a no-op logger; Named returns itself so console construction
// succeeds without a real logging backend.
type StubLogger struct {
	logx.Logger
}

func (l *StubLogger) Named(string) logx.Logger {
	return l
}

// StubBus discards published events for wiring tests.
type StubBus struct{}

func (*StubBus) Publish(context.Context, event.Event, ...event.PublishOption) error {
	return nil
}

func (*StubBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

func (*StubBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

// MarkerLib installs a string global under its own name, so a test can prove
// which library owns a name (framework default vs application override).
type MarkerLib struct {
	name  string
	value string
}

func (l *MarkerLib) Name() string {
	return l.name
}

func (l *MarkerLib) Install(rt *js.Runtime) error {
	return rt.Set(l.name, l.value)
}

// newEngine builds the framework engine over an in-memory database with the
// given application libraries.
func newEngine(t *testing.T, appLibs ...js.Lib) *js.Engine {
	t.Helper()

	engine, err := internaljs.NewEngine(new(StubLogger), new(StubBus), testx.NewTestDB(t), config.SQLite, appLibs)
	require.NoError(t, err, "NewEngine should succeed")

	return engine
}

// alwaysOnNames are the framework utility libraries installed into every
// runtime; optInNames are the capability libraries gated behind EnableLibs.
var (
	alwaysOnNames = []string{"console", "crypto", "cache"}
	optInNames    = []string{"events", "http", "sql"}
)

// TestBuiltinLibraries tests the seeded capability defaults and their tiers.
func TestBuiltinLibraries(t *testing.T) {
	t.Run("UtilitiesAlwaysOn", func(t *testing.T) {
		engine := newEngine(t)

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		for _, name := range alwaysOnNames {
			result, err := rt.RunString(t.Context(), `typeof `+name)
			require.NoErrorf(t, err, "Script for %q should execute successfully", name)
			assert.Equalf(t, "object", result.String(), "Utility %q should install without EnableLibs", name)
		}
	})

	t.Run("CapabilitiesOptIn", func(t *testing.T) {
		engine := newEngine(t)

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `[typeof events, typeof http, typeof sql].join(',')`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined,undefined,undefined", result.String(), "Capability libraries must stay inert until EnableLibs activates them")

		for _, name := range optInNames {
			_, err := engine.NewRuntime(js.EnableLibs(name))
			require.NoErrorf(t, err, "Capability %q should be activatable", name)
		}
	})

	t.Run("CryptoWorksWithoutOptIn", func(t *testing.T) {
		engine := newEngine(t)

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `crypto.md5('abc')`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "900150983cd24fb0d6963f7d28e17f72", result.String(), "The always-on crypto library should be functional")
	})

	t.Run("SQLReadOnlyByDefault", func(t *testing.T) {
		db := testx.NewTestDB(t)

		_, err := db.NewRaw(`CREATE TABLE t (id INTEGER PRIMARY KEY, n INTEGER NOT NULL)`).Exec(t.Context())
		require.NoError(t, err, "Schema should be created")

		_, err = db.NewRaw(`INSERT INTO t (n) VALUES (1), (2)`).Exec(t.Context())
		require.NoError(t, err, "Seed rows should be inserted")

		engine, err := internaljs.NewEngine(new(StubLogger), new(StubBus), db, config.SQLite, nil)
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime(js.EnableLibs("sql"))
		require.NoError(t, err, "NewRuntime should succeed")

		count, err := rt.RunString(t.Context(), `sql.queryList('SELECT COUNT(*) AS c FROM t')[0].c`)
		require.NoError(t, err, "A read query should succeed under the default")
		assert.Equal(t, int64(2), count.ToInteger(), "Query should read seeded rows")

		_, err = rt.RunString(t.Context(), `sql.execute('DELETE FROM t')`)
		require.Error(t, err, "exec should be disabled under the default")
		assert.Contains(t, err.Error(), "execute disabled", "Error should carry the capability reason")
	})
}

// TestOverrides tests overriding defaults across both tiers and adding a new
// library.
func TestOverrides(t *testing.T) {
	t.Run("OverrideKeepsAlwaysOnTier", func(t *testing.T) {
		engine := newEngine(t, &MarkerLib{name: "cache", value: "overridden"})

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `typeof cache + ':' + cache`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "string:overridden", result.String(), "Overriding an always-on default should stay always-on")
	})

	t.Run("OverrideKeepsOptInTier", func(t *testing.T) {
		engine := newEngine(t, &MarkerLib{name: "sql", value: "overridden"})

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		before, err := rt.RunString(t.Context(), `typeof sql`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined", before.String(), "Overriding an opt-in default should stay opt-in")

		rt, err = engine.NewRuntime(js.EnableLibs("sql"))
		require.NoError(t, err, "NewRuntime should succeed")

		after, err := rt.RunString(t.Context(), `sql`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "overridden", after.String(), "The opt-in override should replace the default once activated")
	})

	t.Run("NewLibraryJoinsCatalog", func(t *testing.T) {
		engine := newEngine(t, &MarkerLib{name: "custom", value: "hi"})

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		before, err := rt.RunString(t.Context(), `typeof custom`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined", before.String(), "A new application library should be opt-in, not always-on")

		rt, err = engine.NewRuntime(js.EnableLibs("custom"))
		require.NoError(t, err, "NewRuntime should succeed")

		after, err := rt.RunString(t.Context(), `custom`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "hi", after.String(), "A new-named application library should join the catalog")
	})
}

// TestModuleWiring tests that the FX module resolves the engine with all
// dependencies supplied.
func TestModuleWiring(t *testing.T) {
	var engine *js.Engine

	app := fx.New(
		fx.NopLogger,
		internaljs.Module,
		fx.Provide(
			func() logx.Logger { return new(StubLogger) },
			func() event.Bus { return new(StubBus) },
			func() orm.DB { return testx.NewTestDB(t) },
			func() config.DBKind { return config.SQLite },
		),
		fx.Populate(&engine),
	)
	require.NoError(t, app.Err(), "FX graph should resolve the engine")
	require.NotNil(t, engine, "Engine should be populated")

	rt, err := engine.NewRuntime(js.EnableLibs("sql"))
	require.NoError(t, err, "NewRuntime should succeed")

	result, err := rt.RunString(t.Context(), `[typeof crypto.sha256, typeof sql.queryList].join(',')`)
	require.NoError(t, err, "Script should execute successfully")
	assert.Equal(t, "function,function", result.String(), "The wired engine should expose the always-on crypto and the opt-in sql libraries")
}
