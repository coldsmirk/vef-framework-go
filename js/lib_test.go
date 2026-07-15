package js_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestSourceLib tests source-backed library construction.
func TestSourceLib(t *testing.T) {
	t.Run("InstallsGlobal", func(t *testing.T) {
		lib, err := js.SourceLib("greet", `function greet(name) { return 'hello ' + name; }`)
		require.NoError(t, err, "SourceLib should compile valid source")
		assert.Equal(t, "greet", lib.Name(), "Lib should report its name")

		engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(lib))
		require.NoError(t, err, "NewEngine should succeed")

		rt, err := engine.NewRuntime(js.EnableLibs("greet"))
		require.NoError(t, err, "NewRuntime should succeed")

		result, err := rt.RunString(t.Context(), `greet('js')`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "hello js", result.String(), "Installed source lib should be callable")
	})

	t.Run("CompileErrorSurfacesEagerly", func(t *testing.T) {
		_, err := js.SourceLib("broken", `const x =;`)
		require.Error(t, err, "SourceLib should reject invalid source at construction")
	})
}

// TestProgramLib tests program-backed library construction.
func TestProgramLib(t *testing.T) {
	program := js.MustCompile("answer", `const answer = 42;`, true)
	lib := js.ProgramLib("answer", program)
	assert.Equal(t, "answer", lib.Name(), "Lib should report its name")

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(lib))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs("answer"))
	require.NoError(t, err, "NewRuntime should succeed")

	result, err := rt.RunString(t.Context(), `answer`)
	require.NoError(t, err, "Script should execute successfully")
	assert.Equal(t, int64(42), result.ToInteger(), "Installed program lib should be visible")
}
