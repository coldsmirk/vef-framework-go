package expression_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
)

// StubEngine is a contract-level fake used to exercise the package helpers
// without any backend.
type StubEngine struct {
	value        expression.Value
	err          error
	gotSource    string
	gotEnv       any
	gotPredicate bool
}

func (e *StubEngine) Evaluate(_ context.Context, source string, env any) (expression.Value, error) {
	e.gotSource = source
	e.gotEnv = env

	return e.value, e.err
}

func (e *StubEngine) Compile(source string, opts ...expression.CompileOption) (expression.Program, error) {
	var o expression.CompileOptions
	for _, opt := range opts {
		opt(&o)
	}

	e.gotPredicate = o.Predicate
	e.gotSource = source

	if e.err != nil {
		return nil, e.err
	}

	return &StubProgram{engine: e, source: source}, nil
}

type StubProgram struct {
	engine *StubEngine
	source string
}

func (p *StubProgram) Source() string {
	return p.source
}

func (p *StubProgram) Run(_ context.Context, env any) (expression.Value, error) {
	p.engine.gotEnv = env

	return p.engine.value, p.engine.err
}

func TestEvaluateAs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		eng := &StubEngine{value: expression.NewValue(float64(7))}
		got, err := expression.EvaluateAs[int](context.Background(), eng, "3 + 4", map[string]any{"k": "v"})
		require.NoError(t, err, "EvaluateAs should succeed")
		assert.Equal(t, 7, got, "should decode result to int")
		assert.Equal(t, "3 + 4", eng.gotSource, "source should pass through")
		assert.Equal(t, map[string]any{"k": "v"}, eng.gotEnv, "env should pass through")
	})

	t.Run("Error", func(t *testing.T) {
		sentinel := errors.New("boom")
		eng := &StubEngine{err: sentinel}
		_, err := expression.EvaluateAs[int](context.Background(), eng, "x", nil)
		require.ErrorIs(t, err, sentinel, "engine error should propagate")
	})
}

func TestMatch(t *testing.T) {
	t.Run("Predicate", func(t *testing.T) {
		eng := &StubEngine{value: expression.NewValue(true)}
		ok, err := expression.Match(context.Background(), eng, "$ > 5", map[string]any{"$": 10})
		require.NoError(t, err, "Match should succeed")
		assert.True(t, ok, "Match should return the boolean result")
		assert.True(t, eng.gotPredicate, "Match should compile with AsPredicate")
		assert.Equal(t, map[string]any{"$": 10}, eng.gotEnv, "env should pass through")
	})

	t.Run("Error", func(t *testing.T) {
		sentinel := errors.New("boom")
		eng := &StubEngine{err: sentinel}
		_, err := expression.Match(context.Background(), eng, "x", nil)
		require.ErrorIs(t, err, sentinel, "compile error should propagate")
	})
}
