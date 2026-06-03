package expression

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
)

// StubEngine is a no-op engine for wiring tests.
type StubEngine struct{}

func (*StubEngine) Evaluate(context.Context, string, any) (expression.Value, error) {
	return expression.Value{}, nil
}

func (*StubEngine) Compile(string, ...expression.CompileOption) (expression.Program, error) {
	return nil, nil
}

func TestEngineResolver(t *testing.T) {
	eng := new(StubEngine)
	resolver := NewEngineResolver(eng)

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, reflect.TypeFor[expression.Engine](), resolver.Type(), "Resolver should handle expression.Engine")
	})

	t.Run("Resolve", func(t *testing.T) {
		value, err := resolver.Resolve(nil)
		require.NoError(t, err, "Resolve should not error")

		got, ok := value.Interface().(*StubEngine)
		require.True(t, ok, "Resolved value should be the engine")
		assert.Same(t, eng, got, "Resolved engine should be the provided instance")
	})
}
