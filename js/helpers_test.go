package js_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// StubLib is a minimal Lib for engine tests: it records installs and binds a
// marker global named after itself.
type StubLib struct {
	name       string
	installErr error
	log        *[]string
}

func (l *StubLib) Name() string {
	return l.name
}

func (l *StubLib) Install(rt *js.Runtime) error {
	if l.installErr != nil {
		return l.installErr
	}

	if l.log != nil {
		*l.log = append(*l.log, l.name)
	}

	return rt.Set(l.name, true)
}

// newStdRuntime builds a runtime from a default engine (standard libraries
// installed) with the given options.
func newStdRuntime(t *testing.T, opts ...js.RuntimeOption) *js.Runtime {
	t.Helper()

	engine, err := js.NewEngine()
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(opts...)
	require.NoError(t, err, "NewRuntime should succeed")

	return rt
}
