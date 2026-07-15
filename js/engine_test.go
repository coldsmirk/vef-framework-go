package js_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestNewEngine tests engine construction and eager library validation.
func TestNewEngine(t *testing.T) {
	t.Run("DefaultIncludesStdLibs", func(t *testing.T) {
		rt := newStdRuntime(t)

		result, err := rt.RunString(t.Context(), `typeof dayjs`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "function", result.String(), "Standard library dayjs should be installed by default")
	})

	t.Run("WithoutStdLibs", func(t *testing.T) {
		engine, err := js.NewEngine(js.WithoutStdLibs())
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `typeof dayjs`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined", result.String(), "Bare engine should not install standard libraries")
	})

	t.Run("NilLib", func(t *testing.T) {
		_, err := js.NewEngine(js.WithLibs(nil))
		require.ErrorIs(t, err, js.ErrInvalidLib, "A nil lib should fail engine construction")
	})

	t.Run("EmptyLibName", func(t *testing.T) {
		_, err := js.NewEngine(js.WithLibs(&StubLib{name: ""}))
		require.ErrorIs(t, err, js.ErrInvalidLib, "An empty lib name should fail engine construction")
	})

	t.Run("DuplicateLibName", func(t *testing.T) {
		_, err := js.NewEngine(js.WithLibs(&StubLib{name: "dup"}, &StubLib{name: "dup"}))
		require.ErrorIs(t, err, js.ErrDuplicateLib, "Two libs sharing one name should fail engine construction")
	})

	t.Run("ShadowingStdLib", func(t *testing.T) {
		_, err := js.NewEngine(js.WithLibs(&StubLib{name: "dayjs"}))
		require.ErrorIs(t, err, js.ErrDuplicateLib, "A lib shadowing a standard library should fail engine construction")
	})

	t.Run("StdLibNameAllowedWhenBare", func(t *testing.T) {
		_, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(&StubLib{name: "dayjs"}))
		require.NoError(t, err, "A std lib name should be free on a bare engine")
	})
}

// TestWithBaseLibs tests always-on library installation.
func TestWithBaseLibs(t *testing.T) {
	t.Run("InstalledWithoutEnable", func(t *testing.T) {
		engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithBaseLibs(&StubLib{name: "util"}))
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `util`)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "Base libs should install without EnableLibs")
	})

	t.Run("CollidesWithCatalog", func(t *testing.T) {
		_, err := js.NewEngine(
			js.WithoutStdLibs(),
			js.WithBaseLibs(&StubLib{name: "dup"}),
			js.WithLibs(&StubLib{name: "dup"}),
		)
		require.ErrorIs(t, err, js.ErrDuplicateLib, "A base lib and a catalog lib sharing a name should fail construction")
	})

	t.Run("CollidesWithStdLib", func(t *testing.T) {
		_, err := js.NewEngine(js.WithBaseLibs(&StubLib{name: "dayjs"}))
		require.ErrorIs(t, err, js.ErrDuplicateLib, "A base lib shadowing a standard library should fail construction")
	})

	t.Run("RejectsNil", func(t *testing.T) {
		_, err := js.NewEngine(js.WithBaseLibs(nil))
		require.ErrorIs(t, err, js.ErrInvalidLib, "A nil base lib should fail construction")
	})
}

// TestNewRuntime tests catalog activation semantics.
func TestNewRuntime(t *testing.T) {
	t.Run("CatalogLibsAreOptIn", func(t *testing.T) {
		engine, err := js.NewEngine(js.WithLibs(&StubLib{name: "cap"}))
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `typeof cap`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined", result.String(), "Catalog libs must not be installed without EnableLibs")
	})

	t.Run("EnableLibs", func(t *testing.T) {
		engine, err := js.NewEngine(js.WithLibs(&StubLib{name: "cap"}))
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime(js.EnableLibs("cap"))
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `cap`)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "Enabled catalog lib should be installed")
	})

	t.Run("UnknownLibName", func(t *testing.T) {
		engine, err := js.NewEngine()
		require.NoError(t, err, "NewEngine should succeed")

		_, err = engine.NewRuntime(js.EnableLibs("missing"))
		require.ErrorIs(t, err, js.ErrLibNotFound, "Enabling an unregistered lib should fail")
	})

	t.Run("EnableIsIdempotent", func(t *testing.T) {
		var log []string

		engine, err := js.NewEngine(js.WithLibs(&StubLib{name: "cap", log: &log}))
		require.NoError(t, err, "NewEngine should succeed")

		_, err = engine.NewRuntime(js.EnableLibs("cap", "cap"), js.EnableLibs("cap"))
		require.NoError(t, err, "NewRuntime should succeed")
		assert.Equal(t, []string{"cap"}, log, "Enabling a lib repeatedly should install it once")
	})

	t.Run("InstallOrderFollowsArguments", func(t *testing.T) {
		var log []string

		engine, err := js.NewEngine(js.WithLibs(
			&StubLib{name: "first", log: &log},
			&StubLib{name: "second", log: &log},
		))
		require.NoError(t, err, "NewEngine should succeed")

		_, err = engine.NewRuntime(js.EnableLibs("second", "first"))
		require.NoError(t, err, "NewRuntime should succeed")
		assert.Equal(t, []string{"second", "first"}, log, "Install order should follow EnableLibs argument order")
	})

	t.Run("InstallErrorPropagates", func(t *testing.T) {
		installErr := errors.New("boom")

		engine, err := js.NewEngine(js.WithLibs(&StubLib{name: "cap", installErr: installErr}))
		require.NoError(t, err, "NewEngine should succeed")

		_, err = engine.NewRuntime(js.EnableLibs("cap"))
		require.ErrorIs(t, err, installErr, "Install failure should propagate from NewRuntime")
		assert.Contains(t, err.Error(), "cap", "Install failure should name the offending lib")
	})

	t.Run("RuntimesAreIsolated", func(t *testing.T) {
		engine, err := js.NewEngine()
		require.NoError(t, err, "NewEngine should succeed")

		first, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		second, err := engine.NewRuntime()
		require.NoError(t, err, "NewRuntime should succeed")

		_, err = first.RunString(t.Context(), `globalThis.leak = 42`)
		require.NoError(t, err, "Script should execute successfully")

		result, err := second.RunString(t.Context(), `typeof leak`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "undefined", result.String(), "Globals must not leak across runtimes of one engine")
	})
}
