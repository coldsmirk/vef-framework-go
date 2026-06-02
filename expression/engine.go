package expression

import "context"

// Engine evaluates expressions against a runtime environment.
//
// Implementations adapt a concrete expression backend to this contract.
// Callers depend only on this interface, never on the backend, so the
// underlying engine can be replaced without touching consumer code.
//
// Context cancellation is best-effort and backend-dependent: a backend should
// honor an already-canceled context but may be unable to interrupt an
// evaluation that is already in flight.
//
// The expression syntax itself is defined by the active backend; only the Go
// API, evaluation lifecycle, result handling and errors are abstracted here.
type Engine interface {
	// Evaluate compiles and evaluates source against env in a single step.
	// env holds the variable bindings (a map or struct) the expression reads.
	Evaluate(ctx context.Context, source string, env any) (Value, error)
	// Compile prepares source for repeated evaluation, returning a Program.
	// Options tune compilation, e.g. AsPredicate marks the expression as
	// boolean-valued. A backend may validate eagerly or defer parse and
	// evaluation errors to Program.Run, so a nil error from Compile does not
	// guarantee source is well-formed.
	Compile(source string, opts ...CompileOption) (Program, error)
}

// Program is a prepared expression that can be evaluated repeatedly. Depending
// on the backend, parse or validation errors may surface only from Run.
type Program interface {
	// Run evaluates the compiled program against env.
	Run(ctx context.Context, env any) (Value, error)
	// Source returns the original expression text.
	Source() string
}

// EvaluateAs evaluates source with e and decodes the result into T.
func EvaluateAs[T any](ctx context.Context, e Engine, source string, env any) (T, error) {
	value, err := e.Evaluate(ctx, source, env)
	if err != nil {
		var zero T

		return zero, err
	}

	return DecodeValue[T](value)
}

// Match compiles source as a boolean predicate and returns its result.
func Match(ctx context.Context, e Engine, source string, env any) (bool, error) {
	program, err := e.Compile(source, AsPredicate())
	if err != nil {
		return false, err
	}

	value, err := program.Run(ctx, env)
	if err != nil {
		return false, err
	}

	return value.Bool()
}
