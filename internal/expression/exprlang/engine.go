package exprlang

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/coldsmirk/vef-framework-go/expression"
)

// New returns an Engine backed by the pure-Go expr-lang engine
// (github.com/expr-lang/expr). It replaces the former cgo-based Zen backend, so
// the framework builds without CGO.
func New() expression.Engine {
	return new(engine)
}

type engine struct{}

func (*engine) Evaluate(ctx context.Context, source string, env any) (expression.Value, error) {
	if err := ctx.Err(); err != nil {
		return expression.Value{}, err
	}

	prog, err := compile(source, false)
	if err != nil {
		return expression.Value{}, wrapErr(err)
	}

	return run(prog, env)
}

func (*engine) Compile(source string, opts ...expression.CompileOption) (expression.Program, error) {
	var o expression.CompileOptions
	for _, opt := range opts {
		opt(&o)
	}

	// expr-lang has a real compile step, so parse and type errors surface here
	// eagerly rather than being deferred to Run. The Engine contract permits
	// either; eager validation lets a caller that compiles once and runs many
	// times reject malformed source up front.
	prog, err := compile(source, o.Predicate)
	if err != nil {
		return nil, wrapErr(err)
	}

	return &program{source: source, prog: prog}, nil
}

type program struct {
	source string
	prog   *vm.Program
}

func (p *program) Source() string {
	return p.source
}

func (p *program) Run(ctx context.Context, env any) (expression.Value, error) {
	// expr-lang evaluates synchronously and cannot be interrupted mid-run, so
	// honor an already-canceled context instead of starting work.
	if err := ctx.Err(); err != nil {
		return expression.Value{}, err
	}

	return run(p.prog, env)
}

// compile builds an expr-lang program. AllowUndefinedVariables lets a program
// be compiled without the env (provided only at Run); a predicate is compiled
// with AsBool so a statically non-boolean expression is rejected at compile time
// (an expression typed only at runtime, e.g. over an undefined variable, is
// instead caught when its result fails Value.Bool).
func compile(source string, predicate bool) (*vm.Program, error) {
	options := []expr.Option{expr.AllowUndefinedVariables()}
	if predicate {
		options = append(options, expr.AsBool())
	}

	return expr.Compile(source, options...)
}

// run normalizes the environment and evaluates prog against it, joining any
// failure under ErrEvaluationFailed so the stable code drives API mapping while
// the cause stays available for logs.
func run(prog *vm.Program, env any) (expression.Value, error) {
	normalized, err := normalizeEnv(env)
	if err != nil {
		return expression.Value{}, wrapErr(err)
	}

	out, err := expr.Run(prog, normalized)
	if err != nil {
		return expression.Value{}, wrapErr(err)
	}

	return expression.NewValue(out), nil
}

// normalizeEnv reduces the environment to JSON-native values (a map keyed by
// serialized field names, numbers as float64). This gives expressions
// json-tag-based field access regardless of whether the env is a map or a
// struct — matching how derived fields reference siblings by their serialized
// names — and keeps result types consistent with Value.Decode, which is also
// JSON-based. A nil env becomes an empty map so undefined-variable lookups
// resolve to nil rather than panicking.
func normalizeEnv(env any) (any, error) {
	if env == nil {
		return map[string]any{}, nil
	}

	data, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func wrapErr(err error) error {
	return fmt.Errorf("%w: %w", expression.ErrEvaluationFailed, err)
}
