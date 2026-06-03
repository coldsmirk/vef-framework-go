package zen

import (
	"context"
	"fmt"

	zengo "github.com/gorules/zen-go"

	"github.com/coldsmirk/vef-framework-go/expression"
)

// New returns an Engine backed by the gorules Zen expression engine.
func New() expression.Engine {
	return new(engine)
}

type engine struct{}

func (*engine) Evaluate(ctx context.Context, source string, env any) (expression.Value, error) {
	if err := ctx.Err(); err != nil {
		return expression.Value{}, err
	}

	return wrap(zengo.EvaluateExpression[any](source, env))
}

func (*engine) Compile(source string, opts ...expression.CompileOption) (expression.Program, error) {
	var o expression.CompileOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Zen has no separate compile step; the program holds the source and the
	// chosen mode, re-invoking Zen on each Run.
	return &program{source: source, predicate: o.Predicate}, nil
}

type program struct {
	source    string
	predicate bool
}

func (p *program) Source() string {
	return p.source
}

func (p *program) Run(ctx context.Context, env any) (expression.Value, error) {
	// Zen evaluates synchronously via CGO and cannot be interrupted, so honor an
	// already-canceled context instead of starting work.
	if err := ctx.Err(); err != nil {
		return expression.Value{}, err
	}

	if p.predicate {
		return wrap(zengo.EvaluateUnaryExpression(p.source, env))
	}

	return wrap(zengo.EvaluateExpression[any](p.source, env))
}

// wrap adapts a Zen (result, error) pair into the expression contract, joining
// any backend error under ErrEvaluationFailed so the stable code drives API
// mapping while the cause stays available for logs.
func wrap[T any](result T, err error) (expression.Value, error) {
	if err != nil {
		return expression.Value{}, fmt.Errorf("%w: %w", expression.ErrEvaluationFailed, err)
	}

	return expression.NewValue(result), nil
}
